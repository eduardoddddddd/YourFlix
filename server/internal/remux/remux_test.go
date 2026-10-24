package remux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseVariant(t *testing.T) {
	for s, want := range map[string]Variant{"a2-copy": {2, ModeCopy}, "a1-h264": {1, ModeH264}, "an-copy": {-1, ModeCopy}} {
		v, ok := ParseVariant(s)
		if !ok || v != want || v.String() != s {
			t.Errorf("%s → %+v %v", s, v, ok)
		}
	}
	for _, s := range []string{"", "a-copy", "a2", "a2-vp9", "b2-copy", "a02-copy", "a-1-copy", "a2-copy/../x", "a99999-copy"} {
		if _, ok := ParseVariant(s); ok {
			t.Errorf("%q no debería valer", s)
		}
	}
}

func TestPlans(t *testing.T) {
	// Claves cada 2 s y una de escena a los 7 s: cortes en el primer clave ≥ 6 s desde el anterior.
	keys := []float64{0, 2, 4, 6, 7, 8, 10, 12, 14, 16, 18, 19.5}
	p := copyPlan(keys, 20)
	want := []float64{0, 6, 12, 18}
	if fmt.Sprint(p.Starts) != fmt.Sprint(want) {
		t.Fatalf("cortes %v, quiero %v", p.Starts, want)
	}
	if p.End(3) != 20 || p.SegmentAt(5.99) != 0 || p.SegmentAt(6) != 1 || p.SegmentAt(5.998+eps) != 1 || p.SegmentAt(100) != 3 || p.SegmentAt(-1) != 0 {
		t.Fatal("SegmentAt")
	}
	g := gridPlan(6863)
	if g.Len() != 1144 || g.Starts[1] != 6 || math.Abs(g.End(g.Len()-1)-g.Starts[g.Len()-1]-5) > 1e-9 {
		t.Fatalf("rejilla: %d segmentos, último %v–%v", g.Len(), g.Starts[g.Len()-1], g.End(g.Len()-1))
	}
	m := p.Media("s=abc")
	for _, s := range []string{"#EXT-X-PLAYLIST-TYPE:VOD", "#EXT-X-TARGETDURATION:6", `#EXT-X-MAP:URI="init.mp4?s=abc"`, "#EXTINF:6.000,\n0.m4s?s=abc", "#EXTINF:2.000,\n3.m4s?s=abc\n#EXT-X-ENDLIST"} {
		if !strings.Contains(m, s) {
			t.Errorf("la lista no tiene %q:\n%s", s, m)
		}
	}
}

func TestCodecStrings(t *testing.T) {
	for _, c := range []struct {
		v    VideoInfo
		want string
	}{
		{VideoInfo{Codec: "h264", Profile: "High", Level: 41}, "avc1.640029"},
		{VideoInfo{Codec: "h264", Profile: "Main", Level: 31}, "avc1.4d401f"},
		{VideoInfo{Codec: "hevc", Profile: "Main 10", Level: 153, BitDepth: 10}, "hvc1.2.4.L153.B0"},
		{VideoInfo{Codec: "hevc", Profile: "Main", Level: 120}, "hvc1.1.6.L120.B0"},
	} {
		if got := videoCodecString(c.v); got != c.want {
			t.Errorf("%+v → %s, quiero %s", c.v, got, c.want)
		}
	}
}

// ---------- Con ffmpeg de verdad ----------

var media string // carpeta con los vídeos de prueba generados en TestMain

func TestMain(m *testing.M) {
	code := func() int {
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			return m.Run()
		}
		dir, err := os.MkdirTemp("", "yourflix-remux-")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(dir)
		if err := makeMedia(dir); err != nil {
			fmt.Fprintln(os.Stderr, "no se pudieron generar los vídeos de prueba:", err)
			return 1
		}
		media = dir
		return m.Run()
	}()
	os.Exit(code)
}

func ff(args ...string) error {
	out, err := exec.Command("ffmpeg", append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

// Los tres casos de la colección real: H.264 + AC3 5.1 (con una segunda pista AAC), HEVC
// Main10 + E-AC3 5.1 y AVI XviD + MP3.
func makeMedia(dir string) error {
	five := "pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0"
	if err := ff("-f", "lavfi", "-i", "testsrc2=size=480x270:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-f", "lavfi", "-i", "sine=frequency=660:sample_rate=44100", "-t", "150",
		"-filter_complex", "[1:a]"+five+"[a]", "-map", "0:v", "-map", "[a]", "-map", "2:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "120", "-keyint_min", "24", "-bf", "2",
		"-c:a:0", "ac3", "-b:a:0", "384k", "-c:a:1", "aac", "-metadata:s:a:0", "language=spa", "-metadata:s:a:1", "language=eng",
		filepath.Join(dir, "h264_ac3.mkv")); err != nil {
		return err
	}
	if err := ff("-f", "lavfi", "-i", "testsrc2=size=320x180:rate=25", "-f", "lavfi", "-i", "sine=frequency=330:sample_rate=48000", "-t", "60",
		"-filter_complex", "[1:a]"+five+"[a]", "-map", "0:v", "-map", "[a]",
		"-c:v", "libx265", "-preset", "ultrafast", "-pix_fmt", "yuv420p10le", "-x265-params", "keyint=50:log-level=error",
		"-c:a", "eac3", filepath.Join(dir, "hevc10_eac3.mkv")); err != nil {
		return err
	}
	return ff("-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25", "-f", "lavfi", "-i", "sine=frequency=550:sample_rate=44100", "-t", "60",
		"-c:v", "mpeg4", "-vtag", "XVID", "-q:v", "5", "-c:a", "libmp3lame", "-b:a", "128k", filepath.Join(dir, "xvid_mp3.avi"))
}

func needFFmpeg(t *testing.T) {
	t.Helper()
	if media == "" {
		t.Skip("sin ffmpeg")
	}
}

func probeJSON(t *testing.T, args ...string) map[string]any {
	t.Helper()
	out, err := exec.Command("ffprobe", append([]string{"-v", "error", "-of", "json"}, args...)...).Output()
	if err != nil {
		t.Fatalf("ffprobe %v: %v", args, err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	return m
}

func ffprobeKeyframes(t *testing.T, path string) []float64 {
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var keys []float64
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ts, fl, _ := strings.Cut(l, ",")
		if strings.HasPrefix(fl, "K") {
			v, _ := strconv.ParseFloat(ts, 64)
			keys = append(keys, v)
		}
	}
	return keys
}

func TestMKVKeyframes(t *testing.T) {
	needFFmpeg(t)
	for _, name := range []string{"h264_ac3.mkv", "hevc10_eac3.mkv"} {
		path := filepath.Join(media, name)
		f, _ := os.Open(path)
		st, _ := f.Stat()
		keys, dur, err := MKVKeyframes(f, st.Size())
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := ffprobeKeyframes(t, path)
		if fmt.Sprint(keys) != fmt.Sprint(want) {
			t.Fatalf("%s: índice %v\nffprobe %v", name, keys, want)
		}
		if dur < 59 {
			t.Fatalf("%s: duración %v", name, dur)
		}
	}
	if _, _, err := MKVKeyframes(bytesReaderAt("no es un mkv"), 12); err == nil {
		t.Fatal("debería fallar con algo que no es MKV")
	}
}

func newManager(t *testing.T, maxJobs int) *Manager {
	t.Helper()
	m := New(Config{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Dir: t.TempDir(), MaxJobs: maxJobs, Encoder: "libx264", Log: log.New(io.Discard, "", 0)})
	t.Cleanup(m.Close)
	return m
}

// segmentFile une init + segmento en un MP4 que ffprobe pueda leer.
func segmentFile(t *testing.T, m *Manager, src Source, v Variant, session string, n int) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ip, err := m.Init(ctx, src, v, session, n)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	sp, err := m.Segment(ctx, src, v, session, n)
	if err != nil {
		t.Fatalf("segmento %d: %v", n, err)
	}
	a, _ := os.ReadFile(ip)
	b, _ := os.ReadFile(sp)
	out := filepath.Join(t.TempDir(), fmt.Sprintf("seg%d.mp4", n))
	os.WriteFile(out, append(a, b...), 0o644)
	return out
}

// starts: inicio de vídeo y de audio del primer fragmento tal como lo calcula un navegador
// (tfdt + desplazamiento de composición), en la línea de tiempo del archivo.
func starts(t *testing.T, file string) (video, audio float64) {
	t.Helper()
	b, _ := os.ReadFile(file)
	var tracks map[uint32]track
	haveV, haveA := false, false
	eachBox(b, func(typ string, p []byte, off int) error {
		raw := b[off-8 : off+len(p)]
		switch typ {
		case "moov":
			tracks, _ = parseMoov(raw)
		case "moof":
			eachBox(p, func(typ string, p []byte, _ int) error {
				if typ != "traf" {
					return nil
				}
				var id uint32
				var base float64
				var cts int64
				eachBox(p, func(typ string, p []byte, _ int) error {
					switch typ {
					case "tfhd":
						id = uint32(p[4])<<24 | uint32(p[5])<<16 | uint32(p[6])<<8 | uint32(p[7])
					case "tfdt":
						if p[0] == 1 {
							base = float64(uint64(p[4])<<56 | uint64(p[5])<<48 | uint64(p[6])<<40 | uint64(p[7])<<32 | uint64(p[8])<<24 | uint64(p[9])<<16 | uint64(p[10])<<8 | uint64(p[11]))
						} else {
							base = float64(uint32(p[4])<<24 | uint32(p[5])<<16 | uint32(p[6])<<8 | uint32(p[7]))
						}
					case "trun":
						cts = firstCTS(p)
					}
					return nil
				})
				tr := tracks[id]
				s := (base+float64(cts))/float64(tr.timescale) - tsOffset
				if tr.video && !haveV {
					video, haveV = s, true
				} else if !tr.video && !haveA {
					audio, haveA = s, true
				}
				return nil
			})
		}
		return nil
	})
	return video, audio
}

// checkSegment: códecs esperados, que decodifica sin errores y que empieza y acaba donde dice el plan.
func checkSegment(t *testing.T, file string, wantVideo string, wantStart, wantEnd float64) {
	t.Helper()
	r := probeJSON(t, "-show_entries", "stream=codec_name,codec_type,channels", file)
	var vc, ac string
	for _, s := range r["streams"].([]any) {
		m := s.(map[string]any)
		if m["codec_type"] == "video" {
			vc = fmt.Sprint(m["codec_name"])
		} else if m["codec_type"] == "audio" {
			ac = fmt.Sprint(m["codec_name"])
			if fmt.Sprint(m["channels"]) != "2" {
				t.Errorf("%s: el audio debería ser estéreo: %v", file, m["channels"])
			}
		}
	}
	if vc != wantVideo || ac != "aac" {
		t.Fatalf("%s: códecs %s + %s", file, vc, ac)
	}
	vs, as := starts(t, file)
	if math.Abs(vs-wantStart) > 0.002 {
		t.Fatalf("%s: el vídeo empieza en %.3f y el plan dice %.3f", file, vs, wantStart)
	}
	// Sin hueco de audio al principio; un solape corto con el segmento anterior es inocuo
	// (MSE y Safari sustituyen lo solapado).
	if as > wantStart+0.05 || as < wantStart-0.3 {
		t.Fatalf("%s: el audio empieza en %.3f y el vídeo en %.3f", file, as, wantStart)
	}
	// Último fotograma: el segmento llega hasta el siguiente corte (ffprobe suma el retardo
	// de los fotogramas B, de ahí el margen).
	pk := probeJSON(t, "-select_streams", "v:0", "-show_entries", "packet=pts_time", file)
	last := 0.0
	for _, p := range pk["packets"].([]any) {
		v, _ := strconv.ParseFloat(fmt.Sprint(p.(map[string]any)["pts_time"]), 64)
		last = math.Max(last, v-tsOffset)
	}
	if last > wantEnd+0.15 || last < wantEnd-1 {
		t.Fatalf("%s: el último fotograma está en %.3f y el segmento acaba en %.3f", file, last, wantEnd)
	}
	if out, err := exec.Command("ffmpeg", "-v", "error", "-i", file, "-f", "null", "-").CombinedOutput(); err != nil || len(bytes.TrimSpace(out)) > 0 {
		t.Fatalf("%s no decodifica limpio: %v %s", file, err, out)
	}
}

func TestRemuxSegments(t *testing.T) {
	needFFmpeg(t)
	cases := []struct {
		file, video, codec string
		audio              int
	}{
		{"h264_ac3.mkv", ModeCopy, "h264", 1},
		{"hevc10_eac3.mkv", ModeCopy, "hevc", 1},
		{"xvid_mp3.avi", ModeH264, "h264", 1},
		{"hevc10_eac3.mkv", ModeH264, "h264", 1}, // HEVC para un navegador sin HEVC (Firefox)
	}
	for _, c := range cases {
		t.Run(c.file+"/"+c.video, func(t *testing.T) {
			m := newManager(t, 2)
			src := Source{ID: "f1", Fingerprint: "abc123", Path: filepath.Join(media, c.file)}
			v := Variant{Audio: c.audio, Video: c.video}
			ctx := context.Background()
			media, err := m.MediaPlaylist(ctx, src, v, "s=a")
			if err != nil {
				t.Fatal(err)
			}
			p, _ := m.prepare(ctx, src, v)
			if n := strings.Count(media, "#EXTINF:"); n != p.plan.Len() || n < 5 {
				t.Fatalf("%d segmentos en la lista, %d en el plan", n, p.plan.Len())
			}
			master, err := m.MasterPlaylist(ctx, src, v, "s=a")
			if err != nil || !strings.Contains(master, "index.m3u8?s=a") {
				t.Fatalf("maestra: %v\n%s", err, master)
			}
			if c.codec == "hevc" && !strings.Contains(master, `CODECS="hvc1.2.4.`) {
				t.Fatalf("maestra HEVC Main10 sin hvc1.2.4: %s", master)
			}
			// Principio, un salto a la mitad (otra sesión: ffmpeg nuevo en ese punto) y el último.
			for _, n := range []int{0, p.plan.Len() / 2, p.plan.Len() - 1} {
				f := segmentFile(t, m, src, v, "s"+strconv.Itoa(n), n)
				start := p.plan.Starts[n]
				if n == 0 && c.video == ModeCopy {
					start = p.info.Keyframes[0] // el primer fotograma puede no estar en 0
				}
				checkSegment(t, f, c.codec, start, p.plan.End(n))
				m.Stop(src, v, "s"+strconv.Itoa(n))
			}
		})
	}
}

// Un salto dentro de la misma sesión relanza ffmpeg en el punto pedido y el siguiente segmento
// sale de esa misma ejecución; el límite de conversiones simultáneas se respeta.
func TestRemuxSeekAndLimit(t *testing.T) {
	needFFmpeg(t)
	m := newManager(t, 1)
	src := Source{ID: "f1", Fingerprint: "abc", Path: filepath.Join(media, "h264_ac3.mkv")}
	v := Variant{Audio: 2, Video: ModeCopy}
	ctx := context.Background()
	p, err := m.prepare(ctx, src, v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Segment(ctx, src, v, "uno", 0); err != nil {
		t.Fatal(err)
	}
	mid := p.plan.Len() - 3
	if _, err := m.Segment(ctx, src, v, "uno", mid); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	j := m.jobs[jobKey(p, "uno")]
	m.mu.Unlock()
	if j == nil || j.start != mid {
		t.Fatalf("el salto debería relanzar ffmpeg en %d: %+v", mid, j)
	}
	if _, err := m.Segment(ctx, src, v, "uno", mid+1); err != nil {
		t.Fatal(err)
	}
	// Otra sesión con el límite en 1 y la primera activa: ocupado.
	if _, err := m.Segment(ctx, src, v, "dos", 3); err != ErrBusy {
		t.Fatalf("con el límite lleno debería dar ErrBusy, da %v", err)
	}
	m.Stop(src, v, "uno")
	if _, err := m.Segment(ctx, src, v, "dos", 3); err != nil {
		t.Fatalf("tras parar la primera: %v", err)
	}
	if _, err := m.Segment(ctx, src, Variant{Audio: 7, Video: ModeCopy}, "dos", 0); err != ErrVariant {
		t.Fatalf("pista de audio inexistente: %v", err)
	}
	if _, err := m.Segment(ctx, Source{ID: "x", Fingerprint: "y", Path: filepath.Join(media, "xvid_mp3.avi")}, Variant{Audio: 1, Video: ModeCopy}, "dos", 0); err != ErrVariant {
		t.Fatalf("XviD no se puede copiar: %v", err)
	}
}

// La lista entera, leída por el demuxer HLS de ffmpeg por HTTP de principio a fin: cada
// segmento sale de la ejecución en marcha o de la caché, y la duración total cuadra.
func TestRemuxWholePlaylistOverHTTP(t *testing.T) {
	needFFmpeg(t)
	m := newManager(t, 2)
	src := Source{ID: "f1", Fingerprint: "abc", Path: filepath.Join(media, "h264_ac3.mkv")}
	v := Variant{Audio: 1, Video: ModeCopy}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		s := r.URL.Query().Get("s")
		var path string
		var err error
		switch {
		case name == "master.m3u8":
			body, e := m.MasterPlaylist(r.Context(), src, v, "s="+s)
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			io.WriteString(w, body)
			return
		case name == "index.m3u8":
			body, e := m.MediaPlaylist(r.Context(), src, v, "s="+s)
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			io.WriteString(w, body)
			return
		case name == "init.mp4":
			path, err = m.Init(r.Context(), src, v, s, 0)
		default:
			n, _ := strconv.Atoi(strings.TrimSuffix(name, ".m4s"))
			path, err = m.Segment(r.Context(), src, v, s, n)
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.ServeFile(w, r, path)
	}))
	defer srv.Close()
	out, err := exec.Command("ffmpeg", "-v", "error", "-i", srv.URL+"/hls/master.m3u8?s=x", "-map", "0:v", "-map", "0:a", "-c", "copy", "-f", "null", "-").CombinedOutput()
	if err != nil || len(bytes.TrimSpace(out)) > 0 {
		t.Fatalf("ffmpeg leyendo la lista: %v\n%s", err, out)
	}
	r := probeJSON(t, "-show_entries", "format=duration:stream=codec_name", srv.URL+"/hls/master.m3u8?s=y")
	d, _ := strconv.ParseFloat(fmt.Sprint(r["format"].(map[string]any)["duration"]), 64)
	if math.Abs(d-150) > 0.5 {
		t.Fatalf("duración por HLS %.2f, el archivo dura 150", d)
	}
}

// hls.js pide a la vez el segmento de inicio y el primero: una sola ejecución de ffmpeg,
// también si el primero es anterior al «from» de la lista.
func TestRemuxParallelStart(t *testing.T) {
	needFFmpeg(t)
	m := newManager(t, 2)
	src := Source{ID: "f1", Fingerprint: "abc", Path: filepath.Join(media, "h264_ac3.mkv")}
	v := Variant{Audio: 1, Video: ModeCopy}
	ctx := context.Background()
	for _, c := range []struct {
		session              string
		from, seg, maxStarts int
	}{{"p1", 5, 5, 1}, {"p2", 6, 5, 2}} {
		before := m.starts
		errs := make(chan error, 2)
		go func() { _, err := m.Init(ctx, src, v, c.session, c.from); errs <- err }()
		go func() { _, err := m.Segment(ctx, src, v, c.session, c.seg); errs <- err }()
		for i := 0; i < 2; i++ {
			if err := <-errs; err != nil {
				t.Fatalf("%s: %v", c.session, err)
			}
		}
		if n := m.starts - before; n > c.maxStarts {
			t.Fatalf("%s: %d ejecuciones de ffmpeg, como mucho %d", c.session, n, c.maxStarts)
		}
		m.Stop(src, v, c.session)
	}
}
