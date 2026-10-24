// Package remux convierte en vivo para los navegadores (ADR-006): HLS con segmentos fMP4,
// vídeo copiado (H.264/HEVC) o recodificado a H.264, y audio a AAC estéreo. La TV no lo usa:
// sigue con el archivo tal cual (ADR-001).
//
// La lista es VOD con toda la duración. Cada segmento se genera bajo demanda: una ejecución de
// ffmpeg escribe un MP4 fragmentado por su salida estándar desde el punto pedido y el servidor
// lo corta en segmentos según el plan. Si el reproductor salta lejos, se relanza ffmpeg en ese
// segmento. ffmpeg se frena solo cuando va muy por delante (el servidor deja de leer la tubería)
// y se mata cuando el cliente deja de pedir. Los segmentos quedan en una caché temporal.
package remux

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrBusy      = errors.New("hay demasiadas conversiones en marcha")
	ErrVariant   = errors.New("variante no válida para este archivo")
	ErrNoEncoder = errors.New("el servidor no tiene codificador de vídeo H.264")
	ErrRange     = errors.New("segmento fuera de rango")
	ErrTimeout   = errors.New("la conversión no llegó a tiempo")
)

const (
	lookahead = 4  // un segmento hasta cur+lookahead lo dará la ejecución actual sin relanzarla
	ahead     = 8  // ffmpeg se para cuando va este número de segmentos por delante de lo pedido
	prefetch  = 3  // al servir de la caché, se prepara ffmpeg si falta alguno de los siguientes
	behind    = 30 // con la caché llena se borran segmentos de este número hacia atrás
)

type Config struct {
	FFmpeg, FFprobe string
	Dir             string // caché temporal (dentro del directorio de datos)
	MaxJobs         int    // conversiones simultáneas (por defecto 2)
	Encoder         string // auto | h264_amf | libx264 | off
	CacheBytes      int64  // tope de la caché (por defecto 20 GB)
	Idle            time.Duration
	Log             *log.Logger
}

// Source: el archivo de la biblioteca. La ruta solo la conoce el servidor.
type Source struct {
	ID, Fingerprint, Path string
}

type Manager struct {
	cfg     Config
	ready   chan struct{}
	encoder string
	tonemap bool

	mu      sync.Mutex
	jobs    map[string]*job
	infos   map[string]*Info
	locks   map[string]*sync.Mutex
	lastUse map[string]time.Time
	stop    chan struct{}
	closed  bool
	starts  int // ejecuciones de ffmpeg lanzadas (para las pruebas)
}

func New(cfg Config) *Manager {
	if cfg.MaxJobs <= 0 {
		cfg.MaxJobs = 2
	}
	if cfg.CacheBytes <= 0 {
		cfg.CacheBytes = 20 << 30
	}
	if cfg.Idle <= 0 {
		cfg.Idle = 60 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = log.New(io.Discard, "", 0)
	}
	m := &Manager{cfg: cfg, ready: make(chan struct{}), jobs: map[string]*job{}, infos: map[string]*Info{},
		locks: map[string]*sync.Mutex{}, lastUse: map[string]time.Time{}, stop: make(chan struct{})}
	m.wipe()
	go func() {
		m.encoder = DetectEncoder(context.Background(), cfg.FFmpeg, cfg.Encoder)
		m.tonemap = m.encoder != "" && hasTonemap(context.Background(), cfg.FFmpeg)
		close(m.ready)
		if m.encoder == "" {
			cfg.Log.Printf("web: sin codificador H.264 (--video-encoder %s): XviD y HEVC sin soporte en el navegador no se podrán ver", cfg.Encoder)
		} else {
			cfg.Log.Printf("web: codificador de vídeo %s (mapeo de tonos HDR: %v)", m.encoder, m.tonemap)
		}
	}()
	go m.janitor()
	return m
}

type Status struct {
	Available    bool   `json:"available"`
	VideoEncoder string `json:"videoEncoder"`
	MaxJobs      int    `json:"maxJobs"`
	Jobs         int    `json:"jobs"`
}

func (m *Manager) Status(ctx context.Context) Status {
	select {
	case <-m.ready:
	case <-ctx.Done():
	case <-time.After(25 * time.Second):
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, j := range m.jobs {
		if !j.isDone() {
			n++
		}
	}
	return Status{Available: true, VideoEncoder: m.Encoder(), MaxJobs: m.cfg.MaxJobs, Jobs: n}
}

// Encoder: codificador elegido ("" si no hay o aún se está probando).
func (m *Manager) Encoder() string {
	select {
	case <-m.ready:
		return m.encoder
	default:
		return ""
	}
}

// Close mata todas las conversiones (al parar el servidor).
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	close(m.stop)
	for k, j := range m.jobs {
		j.cancel()
		delete(m.jobs, k)
	}
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func fileKey(src Source) string {
	fp := unsafeChars.ReplaceAllString(src.Fingerprint, "")
	if len(fp) > 12 {
		fp = fp[:12]
	}
	return unsafeChars.ReplaceAllString(src.ID, "_") + "-" + fp
}

var sessionRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func cleanSession(s string) string {
	if sessionRe.MatchString(s) {
		return s
	}
	return "default"
}

type prepared struct {
	src  Source
	key  string // carpeta del archivo
	dir  string // carpeta de la variante
	info *Info
	v    Variant
	plan *Plan
}

func (m *Manager) lockFor(key string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.locks[key]
	if l == nil {
		l = &sync.Mutex{}
		m.locks[key] = l
	}
	return l
}

// prepare: ffprobe (una vez por archivo, se guarda en info.json), fotogramas clave si hace falta y plan.
func (m *Manager) prepare(ctx context.Context, src Source, v Variant) (*prepared, error) {
	key := fileKey(src)
	l := m.lockFor(key)
	l.Lock()
	defer l.Unlock()
	m.mu.Lock()
	in := m.infos[key]
	m.mu.Unlock()
	infoPath := filepath.Join(m.cfg.Dir, key, "info.json")
	if in == nil {
		if b, err := os.ReadFile(infoPath); err == nil {
			var x Info
			if json.Unmarshal(b, &x) == nil && x.Duration > 0 {
				in = &x
			}
		}
	}
	dirty := false
	if in == nil {
		var err error
		if in, err = probeInfo(ctx, m.cfg.FFprobe, src.Path); err != nil {
			return nil, err
		}
		dirty = true
	}
	if v.Video == ModeCopy && len(in.Keyframes) == 0 {
		if in.Video.Codec != "h264" && in.Video.Codec != "hevc" {
			return nil, ErrVariant
		}
		t0 := time.Now()
		keys, err := keyframes(ctx, m.cfg.FFprobe, src.Path, in)
		if err != nil {
			return nil, err
		}
		in.Keyframes = keys
		dirty = true
		if d := time.Since(t0); d > time.Second {
			m.cfg.Log.Printf("web: %d fotogramas clave de %s en %s", len(keys), src.ID, d.Round(time.Millisecond))
		}
	}
	if dirty {
		os.MkdirAll(filepath.Dir(infoPath), 0o755)
		if b, err := json.Marshal(in); err == nil {
			writeAtomic(infoPath, b)
		}
	}
	m.mu.Lock()
	m.infos[key] = in
	m.mu.Unlock()

	if v.Audio >= 0 {
		ok := false
		for _, a := range in.Audio {
			ok = ok || a.Index == v.Audio
		}
		if !ok {
			return nil, ErrVariant
		}
	}
	p := &prepared{src: src, key: key, dir: filepath.Join(m.cfg.Dir, key, v.String()), info: in, v: v}
	switch v.Video {
	case ModeCopy:
		p.plan = copyPlan(in.Keyframes, in.Duration)
	case ModeH264:
		select {
		case <-m.ready:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if m.encoder == "" {
			return nil, ErrNoEncoder
		}
		p.plan = gridPlan(in.Duration)
	default:
		return nil, ErrVariant
	}
	return p, nil
}

// MasterPlaylist: lista maestra (una variante, con CODECS). query: «s=…&from=…».
func (m *Manager) MasterPlaylist(ctx context.Context, src Source, v Variant, query string) (string, error) {
	p, err := m.prepare(ctx, src, v)
	if err != nil {
		return "", err
	}
	in := p.info
	codecs, bw := transcodeCodecs, int64(12_000_000)
	w, h := in.Video.Width, in.Video.Height
	vr := ""
	if v.Video == ModeCopy {
		codecs = videoCodecString(in.Video)
		if v.Audio >= 0 {
			codecs += "," + aacCodec
		}
		bw = in.Bitrate
		if bw <= 0 {
			bw = 10_000_000
		}
		vr = videoRange(in.Video.Transfer)
	} else {
		if v.Audio < 0 {
			codecs = strings.Split(transcodeCodecs, ",")[0]
		}
		if h > 1080 || w > 1920 {
			if w*1080 > h*1920 {
				h, w = h*1920/w, 1920
			} else {
				w, h = w*1080/h, 1080
			}
		}
	}
	uri := "index.m3u8"
	if query != "" {
		uri += "?" + query
	}
	return Master(codecs, bw, w, h, vr, uri), nil
}

func (m *Manager) MediaPlaylist(ctx context.Context, src Source, v Variant, query string) (string, error) {
	p, err := m.prepare(ctx, src, v)
	if err != nil {
		return "", err
	}
	return p.plan.Media(query), nil
}

// SegmentFor: índice del segmento que contiene t (para el «from» del segmento de inicio).
func (m *Manager) SegmentFor(ctx context.Context, src Source, v Variant, t float64) (int, error) {
	p, err := m.prepare(ctx, src, v)
	if err != nil {
		return 0, err
	}
	return p.plan.SegmentAt(t), nil
}

func segName(n int) string { return strconv.Itoa(n) + ".m4s" }

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Init devuelve el segmento de inicio (ftyp+moov). from: segmento por el que empezará el
// reproductor, para arrancar ffmpeg ahí y no al principio.
func (m *Manager) Init(ctx context.Context, src Source, v Variant, session string, from int) (string, error) {
	p, err := m.prepare(ctx, src, v)
	if err != nil {
		return "", err
	}
	if from < 0 || from >= p.plan.Len() {
		from = 0
	}
	final := filepath.Join(p.dir, "init.mp4")
	m.touch(p.dir)
	if exists(final) {
		return final, nil
	}
	return m.wait(ctx, p, cleanSession(session), from, final)
}

// Segment devuelve la ruta del segmento n, generándolo si hace falta.
func (m *Manager) Segment(ctx context.Context, src Source, v Variant, session string, n int) (string, error) {
	p, err := m.prepare(ctx, src, v)
	if err != nil {
		return "", err
	}
	if n < 0 || n >= p.plan.Len() {
		return "", ErrRange
	}
	session = cleanSession(session)
	final := filepath.Join(p.dir, segName(n))
	m.touch(p.dir)
	if exists(final) {
		// De la caché; se prepara ffmpeg si falta alguno de los siguientes.
		for i := n + 1; i <= n+prefetch && i < p.plan.Len(); i++ {
			if !exists(filepath.Join(p.dir, segName(i))) {
				m.ensureJob(p, session, n, i)
				break
			}
		}
		m.want(p, session, n)
		return final, nil
	}
	return m.wait(ctx, p, session, n, final)
}

// wait asegura una ejecución que produzca el segmento n (o el de inicio) y espera al archivo.
func (m *Manager) wait(ctx context.Context, p *prepared, session string, n int, final string) (string, error) {
	j, err := m.ensureJob(p, session, n, n)
	if err != nil {
		return "", err
	}
	timeout := time.NewTimer(90 * time.Second)
	defer timeout.Stop()
	restarts := 0
	isInit := strings.HasSuffix(final, "init.mp4")
	for {
		if exists(final) {
			return final, nil
		}
		j.mu.Lock()
		done, jerr, cur, ch := j.done, j.err, j.cur, j.changed
		j.mu.Unlock()
		// La ejecución terminó o pasó de largo sin dar el segmento (p. ej. un corte sin
		// fotograma clave al recodificar): una nueva empieza justo en él.
		if (done || (!isInit && cur > n)) && !exists(final) {
			// Otra petición de la misma sesión ya relanzó ffmpeg: si esa ejecución lo dará, se espera a ella.
			if nj := m.current(p, session); nj != nil && nj != j && nj.covers(n, isInit) {
				j = nj
				continue
			}
			if restarts >= 2 {
				if jerr != nil {
					return "", jerr
				}
				return "", fmt.Errorf("ffmpeg no produjo el segmento %d", n)
			}
			restarts++
			if j, err = m.restart(p, session, n); err != nil {
				return "", err
			}
			continue
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout.C:
			return "", ErrTimeout
		}
	}
}

func (m *Manager) current(p *prepared, session string) *job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[jobKey(p, session)]
}

// covers: la ejecución sigue viva y dará pronto el segmento n (cualquiera da el de inicio).
func (j *job) covers(n int, isInit bool) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return !j.done && (isInit || (n >= j.start && n <= j.cur+lookahead))
}

func (m *Manager) touch(dir string) {
	m.mu.Lock()
	m.lastUse[dir] = time.Now()
	m.mu.Unlock()
}

func jobKey(p *prepared, session string) string { return p.key + "/" + p.v.String() + "/" + session }

// want: el reproductor ha pedido el segmento n (mueve el freno de la ejecución en marcha).
func (m *Manager) want(p *prepared, session string, n int) {
	m.mu.Lock()
	j := m.jobs[jobKey(p, session)]
	m.mu.Unlock()
	if j != nil {
		j.request(n)
	}
}

// ensureJob: si la ejecución de esta sesión dará pronto el segmento need, se reutiliza; si no,
// se relanza ffmpeg en need. requested es lo que el reproductor ha pedido de verdad.
// Todo bajo m.mu: hls.js pide a la vez el segmento de inicio y el primero, y las dos
// peticiones no deben lanzar dos ffmpeg.
func (m *Manager) ensureJob(p *prepared, session string, requested, need int) (*job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[jobKey(p, session)]; j != nil {
		if j.covers(need, false) {
			j.request(requested)
			return j, nil
		}
	}
	return m.restartLocked(p, session, need)
}

// restart lanza ffmpeg desde el segmento start (matando la ejecución anterior de la sesión).
func (m *Manager) restart(p *prepared, session string, start int) (*job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restartLocked(p, session, start)
}

func (m *Manager) restartLocked(p *prepared, session string, start int) (*job, error) {
	if m.closed {
		return nil, errors.New("servidor parándose")
	}
	key := jobKey(p, session)
	// Una sesión (un reproductor) solo convierte una cosa a la vez: al saltar, cambiar de
	// audio o de capítulo, la ejecución anterior sobra.
	for k, j := range m.jobs {
		if k == key || strings.HasSuffix(k, "/"+session) || j.isDone() {
			j.cancel()
			delete(m.jobs, k)
		}
	}
	if len(m.jobs) >= m.cfg.MaxJobs {
		// Se libera la más antigua si su reproductor lleva un rato sin pedir nada.
		var oldK string
		var old *job
		for k, j := range m.jobs {
			if old == nil || j.last().Before(old.last()) {
				oldK, old = k, j
			}
		}
		if old == nil || time.Since(old.last()) < 15*time.Second {
			return nil, ErrBusy
		}
		old.cancel()
		delete(m.jobs, oldK)
	}
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{m: m, p: p, start: start, cur: start, wanted: start, lastReq: time.Now(), cancel: cancel, changed: make(chan struct{})}
	m.jobs[key] = j
	m.starts++
	m.cfg.Log.Printf("web: ffmpeg %s %s desde el segmento %d (%s)", p.src.ID, p.v, start, clock(p.plan.Starts[start]))
	go j.run(ctx)
	return j, nil
}

// Stop: el reproductor se ha cerrado.
func (m *Manager) Stop(src Source, v Variant, session string) {
	key := fileKey(src) + "/" + v.String() + "/" + cleanSession(session)
	m.mu.Lock()
	defer m.mu.Unlock()
	if j := m.jobs[key]; j != nil {
		j.cancel()
		delete(m.jobs, key)
	}
}

func clock(t float64) string {
	s := int(t)
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

type job struct {
	m      *Manager
	p      *prepared
	start  int
	cancel context.CancelFunc

	mu      sync.Mutex
	cur     int // segmento que se está escribiendo
	wanted  int // el más alto que ha pedido el reproductor
	lastReq time.Time
	done    bool
	err     error
	changed chan struct{} // se cierra (y se renueva) con cada novedad
}

func (j *job) broadcast() {
	close(j.changed)
	j.changed = make(chan struct{})
}

func (j *job) request(n int) {
	j.mu.Lock()
	if n > j.wanted {
		j.wanted = n
	}
	j.lastReq = time.Now()
	j.broadcast()
	j.mu.Unlock()
}

func (j *job) last() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.lastReq
}

func (j *job) isDone() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.done
}

func (j *job) setCur(n int) {
	j.mu.Lock()
	j.cur = n
	j.broadcast()
	j.mu.Unlock()
}

// room: frena la lectura (y con ella a ffmpeg, que se bloquea al escribir) si va muy por delante.
func (j *job) room(ctx context.Context) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	for j.cur > j.wanted+ahead {
		ch := j.changed
		j.mu.Unlock()
		select {
		case <-ctx.Done():
			j.mu.Lock()
			return false
		case <-ch:
		case <-time.After(5 * time.Second):
		}
		j.mu.Lock()
	}
	return ctx.Err() == nil
}

// tail guarda las últimas líneas de error de ffmpeg.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	t.buf = append(t.buf, b...)
	if len(t.buf) > 2048 {
		t.buf = t.buf[len(t.buf)-2048:]
	}
	t.mu.Unlock()
	return len(b), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}

func (j *job) run(ctx context.Context) {
	p := j.p
	m := j.m
	var stderr tail
	cmd := exec.CommandContext(ctx, m.cfg.FFmpeg, m.ffmpegArgs(p.src.Path, p.info, p.v, p.plan, j.start)...)
	cmd.Stderr = &stderr
	cmd.WaitDelay = 5 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		j.finish(fmt.Errorf("ffmpeg: %w", err))
		return
	}
	readErr := j.segmentStream(ctx, bufio.NewReaderSize(stdout, 1<<20))
	if readErr != nil {
		cmd.Process.Kill()
	}
	io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	switch {
	case ctx.Err() != nil:
		j.finish(nil) // la hemos parado nosotros
	case readErr != nil:
		m.cfg.Log.Printf("web: %s %s: %v", p.src.ID, p.v, readErr)
		j.finish(readErr)
	case waitErr != nil:
		m.cfg.Log.Printf("web: ffmpeg %s %s: %v: %s", p.src.ID, p.v, waitErr, stderr.String())
		j.finish(fmt.Errorf("ffmpeg falló: %v", waitErr))
	default:
		j.finish(nil)
	}
}

func (j *job) finish(err error) {
	j.mu.Lock()
	j.done, j.err = true, err
	j.broadcast()
	j.mu.Unlock()
}

// segmentStream lee el MP4 fragmentado y escribe init.mp4 y los segmentos del plan.
func (j *job) segmentStream(ctx context.Context, br *bufio.Reader) error {
	p := j.p
	var tracks map[uint32]track
	var initBuf []byte
	seg, frag := -1, 0
	var w *segWriter
	drop := false
	defer func() {
		if w != nil {
			w.abort()
		}
	}()
	for {
		if !j.room(ctx) {
			return nil
		}
		typ, raw, err := readBox(br)
		if err == io.EOF {
			if w != nil && ctx.Err() == nil {
				if err := w.commit(); err != nil {
					return err
				}
				w = nil
				j.setCur(seg + 1)
			}
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		switch typ {
		case "ftyp":
			initBuf = append(initBuf, raw...)
		case "moov":
			initBuf = append(initBuf, raw...)
			if tracks, err = parseMoov(raw); err != nil {
				return err
			}
			if path := filepath.Join(p.dir, "init.mp4"); !exists(path) {
				if err := writeAtomic(path, initBuf); err != nil {
					return err
				}
			}
			j.setCur(j.start) // despierta a quien espera el segmento de inicio
		case "moof":
			if tracks == nil {
				return errors.New("MP4: moof antes de moov")
			}
			fi, err := parseMoof(raw, tracks)
			if err != nil {
				return err
			}
			s := seg
			if fi.hasVideo {
				s = p.plan.SegmentAt(fi.videoStart - tsOffset)
			}
			drop = s < j.start
			if drop {
				continue
			}
			if s != seg {
				if w != nil {
					if err := w.commit(); err != nil {
						return err
					}
				}
				seg, frag = s, 0
				w = newSegWriter(filepath.Join(p.dir, segName(s)))
				j.setCur(s)
			}
			frag++
			if fi.seqOff >= 0 && fi.seqOff+4 <= len(raw) {
				// Números de secuencia crecientes entre ejecuciones distintas de ffmpeg.
				n := uint32(s)*10000 + uint32(frag)
				raw[fi.seqOff], raw[fi.seqOff+1], raw[fi.seqOff+2], raw[fi.seqOff+3] = byte(n>>24), byte(n>>16), byte(n>>8), byte(n)
			}
			if err := w.write(raw); err != nil {
				return err
			}
		case "mdat":
			if drop || w == nil {
				continue
			}
			if err := w.write(raw); err != nil {
				return err
			}
		}
	}
}

// segWriter escribe en un temporal y lo renombra al terminar. Si el segmento ya está en la
// caché (otra ejecución lo hizo) no escribe nada: en Windows no se puede reemplazar un
// archivo que se está sirviendo.
type segWriter struct {
	final string
	f     *os.File
}

func newSegWriter(final string) *segWriter {
	w := &segWriter{final: final}
	if !exists(final) {
		if f, err := os.CreateTemp(filepath.Dir(final), filepath.Base(final)+".tmp-*"); err == nil {
			w.f = f
		}
	}
	return w
}

func (w *segWriter) write(b []byte) error {
	if w.f == nil {
		return nil
	}
	_, err := w.f.Write(b)
	return err
}

func (w *segWriter) commit() error {
	if w.f == nil {
		return nil
	}
	name := w.f.Name()
	err := w.f.Close()
	w.f = nil
	if err == nil && !exists(w.final) {
		err = os.Rename(name, w.final)
	}
	if err != nil || exists(name) {
		os.Remove(name)
	}
	return err
}

func (w *segWriter) abort() {
	if w.f != nil {
		name := w.f.Name()
		w.f.Close()
		os.Remove(name)
		w.f = nil
	}
}

func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}

// wipe: al arrancar, la caché de segmentos empieza vacía (se conservan los info.json).
func (m *Manager) wipe() {
	os.MkdirAll(m.cfg.Dir, 0o755)
	files, _ := filepath.Glob(filepath.Join(m.cfg.Dir, "*", "*"))
	for _, f := range files {
		if st, err := os.Stat(f); err == nil && st.IsDir() {
			os.RemoveAll(f)
		}
	}
}

// janitor: mata conversiones abandonadas y mantiene la caché por debajo del tope.
func (m *Manager) janitor() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	n := 0
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
		}
		m.reap()
		if n++; n%4 == 0 {
			m.cleanCache()
		}
	}
}

func (m *Manager) reap() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, j := range m.jobs {
		if j.isDone() {
			delete(m.jobs, k)
		} else if time.Since(j.last()) > m.cfg.Idle {
			m.cfg.Log.Printf("web: ffmpeg %s %s parado: el reproductor ya no pide", j.p.src.ID, j.p.v)
			j.cancel()
			delete(m.jobs, k)
		}
	}
}

type cacheDir struct {
	path string
	size int64
	used time.Time
}

func (m *Manager) cleanCache() {
	paths, _ := filepath.Glob(filepath.Join(m.cfg.Dir, "*", "*"))
	m.mu.Lock()
	active := map[string]*job{}
	for _, j := range m.jobs {
		active[j.p.dir] = j
	}
	var dirs []cacheDir
	var total int64
	for _, d := range paths {
		st, err := os.Stat(d)
		if err != nil || !st.IsDir() {
			continue
		}
		cd := cacheDir{path: d, used: m.lastUse[d]}
		if cd.used.IsZero() {
			cd.used = st.ModTime()
		}
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if i, err := e.Info(); err == nil {
				cd.size += i.Size()
			}
		}
		total += cd.size
		dirs = append(dirs, cd)
	}
	m.mu.Unlock()
	sort.Slice(dirs, func(a, b int) bool { return dirs[a].used.Before(dirs[b].used) })
	for _, d := range dirs {
		if active[d.path] != nil {
			continue
		}
		if time.Since(d.used) > 6*time.Hour || total > m.cfg.CacheBytes {
			if os.RemoveAll(d.path) == nil {
				total -= d.size
				m.mu.Lock()
				delete(m.lastUse, d.path)
				m.mu.Unlock()
			}
		}
	}
	if total <= m.cfg.CacheBytes {
		return
	}
	// Aún por encima: segmentos muy por detrás de lo que se está viendo.
	for dir, j := range active {
		j.mu.Lock()
		limit := j.wanted - behind
		j.mu.Unlock()
		for i := 0; i < limit; i++ {
			os.Remove(filepath.Join(dir, segName(i)))
		}
	}
}
