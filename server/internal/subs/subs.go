// Package subs prepara los subtítulos de texto para que la TV los dibuje (AVPlay no permite
// elegir pista de texto sin colgarse: docs/DECISIONES.md, ADR-001). Fuentes: pistas internas
// de texto (SRT, ASS, mov_text, WebVTT) y archivos junto al vídeo (.srt, .ass, .ssa, .vtt).
// Las pistas en imagen (PGS, DVD) no se pueden convertir aquí.
package subs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
)

// Cue: [inicio, fin, texto] en segundos; compacto para la TV.
type Cue [3]any

type Track struct {
	ID       string `json:"id"` // e<índice de pista> (interna) · x<n> (archivo externo)
	Language string `json:"language,omitempty"`
	Label    string `json:"label"`
	Codec    string `json:"codec"`
	Forced   bool   `json:"forced,omitempty"`
	Default  bool   `json:"default,omitempty"`
	Source   string `json:"source"` // embedded | external
	path     string
	index    int
}

var sidecarExt = map[string]bool{".srt": true, ".ass": true, ".ssa": true, ".vtt": true}

var langWords = map[string]string{
	"es": "spa", "spa": "spa", "esp": "spa", "spanish": "spa", "espanol": "spa", "español": "spa", "castellano": "spa", "cas": "spa",
	"en": "eng", "eng": "eng", "english": "eng", "ingles": "eng", "inglés": "eng",
	"fr": "fre", "fre": "fre", "fra": "fre", "french": "fre", "it": "ita", "ita": "ita", "de": "ger", "ger": "ger", "deu": "ger",
	"pt": "por", "por": "por", "ca": "cat", "cat": "cat", "latino": "spa",
}

var langNames = map[string]string{"spa": "Español", "eng": "Inglés", "fre": "Francés", "ita": "Italiano", "ger": "Alemán", "por": "Portugués", "cat": "Catalán", "jpn": "Japonés", "kor": "Coreano"}

func label(lang, title string, forced bool) string {
	l := langNames[lang]
	if l == "" {
		l = strings.ToUpper(lang)
	}
	if l == "" {
		l = "Desconocido"
	}
	if forced {
		l += " (forzados)"
	} else if t := strings.TrimSpace(title); t != "" && !strings.EqualFold(t, l) {
		l += " · " + t
	}
	return l
}

// Tracks lista las pistas de texto de un vídeo: internas (de ffprobe) y externas.
func Tracks(videoPath string, embedded []probe.Subtitle) []Track {
	var out []Track
	for _, s := range embedded {
		if !s.Text {
			continue
		}
		forced := s.Forced || strings.Contains(strings.ToLower(s.Title), "forzad") || strings.Contains(strings.ToLower(s.Title), "forced")
		out = append(out, Track{ID: "e" + strconv.Itoa(s.Index), Language: s.Language, Label: label(s.Language, s.Title, forced), Codec: s.Codec,
			Forced: forced, Default: s.Default, Source: "embedded", index: s.Index})
	}
	dir, base := filepath.Dir(videoPath), strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && sidecarExt[strings.ToLower(filepath.Ext(n))] && strings.HasPrefix(strings.ToLower(n), strings.ToLower(base)+".") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for i, n := range names {
		// «Película.es.forced.srt» → idioma es, forzados
		mid := strings.ToLower(strings.TrimSuffix(n[len(base):], filepath.Ext(n)))
		lang, forced := "", false
		for _, w := range strings.FieldsFunc(mid, func(r rune) bool { return r == '.' || r == '_' || r == '-' || r == ' ' || r == '[' || r == ']' }) {
			if l, ok := langWords[w]; ok {
				lang = l
			}
			if w == "forced" || w == "forzados" || w == "forzado" {
				forced = true
			}
		}
		out = append(out, Track{ID: "x" + strconv.Itoa(i), Language: lang, Label: label(lang, "", forced) + " · archivo", Codec: strings.TrimPrefix(strings.ToLower(filepath.Ext(n)), "."),
			Forced: forced, Source: "external", path: filepath.Join(dir, n)})
	}
	return out
}

// Extractor convierte pistas a cues y las guarda en caché (data/subs).
type Extractor struct {
	FFmpeg string
	Dir    string
	mu     sync.Mutex
	locks  map[string]*sync.Mutex
}

func (x *Extractor) lock(key string) func() {
	x.mu.Lock()
	if x.locks == nil {
		x.locks = map[string]*sync.Mutex{}
	}
	l := x.locks[key]
	if l == nil {
		l = &sync.Mutex{}
		x.locks[key] = l
	}
	x.mu.Unlock()
	l.Lock()
	return l.Unlock
}

func (x *Extractor) cachePath(fileID, trackID string) string {
	return filepath.Join(x.Dir, fileID+"-"+trackID+".json")
}

// Cues devuelve las líneas de una pista. fingerprint invalida la caché si el archivo cambia.
// Las internas se extraen todas en una sola lectura del vídeo (puede tardar en discos lentos).
func (x *Extractor) Cues(ctx context.Context, fileID, fingerprint, videoPath string, tracks []Track, trackID string) ([]Cue, error) {
	var tr *Track
	for i := range tracks {
		if tracks[i].ID == trackID {
			tr = &tracks[i]
		}
	}
	if tr == nil {
		return nil, fmt.Errorf("pista %s no existe", trackID)
	}
	key := fileID + "-" + fingerprint[:min(8, len(fingerprint))]
	if c, ok := x.load(key, trackID); ok {
		return c, nil
	}
	defer x.lock(fileID)()
	if c, ok := x.load(key, trackID); ok {
		return c, nil
	}
	if tr.Source == "external" {
		data, err := os.ReadFile(tr.path)
		if err != nil {
			return nil, err
		}
		if tr.Codec != "srt" {
			if data, err = x.ffmpegToSRT(ctx, tr.path, -1); err != nil {
				return nil, err
			}
		}
		cues := ParseSRT(data)
		return cues, x.save(key, trackID, cues)
	}
	// Una pasada para todas las internas de texto.
	var emb []Track
	for _, t := range tracks {
		if t.Source == "embedded" {
			emb = append(emb, t)
		}
	}
	all, err := x.extractAll(ctx, videoPath, emb)
	if err != nil {
		return nil, err
	}
	for id, cues := range all {
		x.save(key, id, cues)
	}
	return all[trackID], nil
}

func (x *Extractor) load(key, trackID string) ([]Cue, bool) {
	data, err := os.ReadFile(x.cachePath(key, trackID))
	if err != nil {
		return nil, false
	}
	var c []Cue
	return c, json.Unmarshal(data, &c) == nil
}

func (x *Extractor) save(key, trackID string, cues []Cue) error {
	if err := os.MkdirAll(x.Dir, 0o755); err != nil {
		return err
	}
	if cues == nil {
		cues = []Cue{}
	}
	data, _ := json.Marshal(cues)
	tmp := x.cachePath(key, trackID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, x.cachePath(key, trackID))
}

func (x *Extractor) extractAll(ctx context.Context, video string, tracks []Track) (map[string][]Cue, error) {
	tmp, err := os.MkdirTemp("", "yourflix-subs-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	args := []string{"-v", "error", "-nostdin", "-i", video}
	for _, t := range tracks {
		args = append(args, "-map", "0:"+strconv.Itoa(t.index), "-c:s", "srt", "-f", "srt", filepath.Join(tmp, t.ID+".srt"))
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(ctx, x.FFmpeg, args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %v %s", err, bytes.TrimSpace(out))
	}
	res := map[string][]Cue{}
	for _, t := range tracks {
		data, _ := os.ReadFile(filepath.Join(tmp, t.ID+".srt"))
		res[t.ID] = ParseSRT(data)
	}
	return res, nil
}

func (x *Extractor) ffmpegToSRT(ctx context.Context, in string, index int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	args := []string{"-v", "error", "-nostdin", "-i", in}
	if index >= 0 {
		args = append(args, "-map", "0:"+strconv.Itoa(index))
	}
	args = append(args, "-c:s", "srt", "-f", "srt", "-")
	out, err := exec.CommandContext(ctx, x.FFmpeg, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	return out, nil
}

var (
	reTime = regexp.MustCompile(`(\d{1,2}):(\d{2}):(\d{2})[,.](\d{1,3})\s*-->\s*(\d{1,2}):(\d{2}):(\d{2})[,.](\d{1,3})`)
	reTags = regexp.MustCompile(`</?[a-zA-Z][^>]*>|\{\\[^}]*\}`)
)

// ParseSRT lee SRT en UTF-8 (con o sin BOM) o en Windows-1252/Latin-1 (habitual en
// subtítulos españoles antiguos), con saltos de línea CRLF o LF.
func ParseSRT(data []byte) []Cue {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		data = cp1252(data)
	}
	var out []Cue
	sc := bufio.NewScanner(bytes.NewReader(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	var start, end float64
	var text []string
	inCue := false
	flush := func() {
		if inCue {
			t := strings.TrimSpace(reTags.ReplaceAllString(strings.Join(text, "\n"), ""))
			if t != "" && end > start {
				out = append(out, Cue{round(start), round(end), t})
			}
		}
		inCue, text = false, nil
	}
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t\r")
		if m := reTime.FindStringSubmatch(line); m != nil {
			flush()
			start, end, inCue = ts(m[1:5]), ts(m[5:9]), true
			continue
		}
		if line == "" {
			flush()
			continue
		}
		if inCue {
			text = append(text, line)
		}
	}
	flush()
	sort.SliceStable(out, func(i, j int) bool { return out[i][0].(float64) < out[j][0].(float64) })
	return out
}

func ts(p []string) float64 {
	h, _ := strconv.Atoi(p[0])
	m, _ := strconv.Atoi(p[1])
	s, _ := strconv.Atoi(p[2])
	ms := p[3]
	for len(ms) < 3 {
		ms += "0"
	}
	f, _ := strconv.Atoi(ms)
	return float64(h*3600+m*60+s) + float64(f)/1000
}

func round(f float64) float64 { return float64(int64(f*1000+0.5)) / 1000 }

var cp1252High = [32]rune{'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
	0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ'}

func cp1252(b []byte) []byte {
	var buf strings.Builder
	for _, c := range b {
		switch {
		case c < 0x80:
			buf.WriteByte(c)
		case c < 0xA0:
			buf.WriteRune(cp1252High[c-0x80])
		default:
			buf.WriteRune(rune(c))
		}
	}
	return []byte(buf.String())
}
