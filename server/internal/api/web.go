package api

// Cliente web (ADR-006): la página estática de web/ en «/» y lo que necesitan los navegadores
// además de /api/v1: HLS con remux en vivo y subtítulos en WebVTT. Todo se añade sin tocar las
// rutas que usa la TV; la TV sigue con /stream (ADR-001).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/remux"
	"github.com/eduardoddddddd/YourFlix/server/internal/subs"
)

type Web struct {
	API   *Server
	Dir   string         // carpeta web/ ("" = sin página web)
	Remux *remux.Manager // nil = sin conversión en vivo (solo reproducción directa)
}

// Handler: la API de la TV intacta, más las rutas de la web.
func (wb *Web) Handler() http.Handler {
	s := wb.API
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/remux", wb.status)
	mux.HandleFunc("GET /api/v1/files/{id}/vtt/{sid}", wb.vtt)
	mux.HandleFunc("GET /api/v1/files/{id}/hls/{variant}/{name}", wb.hls)
	mux.HandleFunc("POST /api/v1/files/{id}/hls/{variant}/stop", wb.stop)
	mux.Handle("/api/", s.Handler())
	mux.Handle("/", wb.static())
	return s.lanOnly(cors(mux))
}

func (wb *Web) status(w http.ResponseWriter, r *http.Request) {
	if wb.Remux == nil {
		writeJSON(w, remux.Status{})
		return
	}
	writeJSON(w, wb.Remux.Status(r.Context()))
}

func (wb *Web) source(w http.ResponseWriter, id string) (remux.Source, bool) {
	s := wb.API
	f, err := s.DB.File(id)
	if err != nil {
		s.fail(w, err)
		return remux.Source{}, false
	}
	if f.MissingSince != "" {
		http.Error(w, "el disco de este archivo no está conectado", http.StatusGone)
		return remux.Source{}, false
	}
	src, err := s.DB.Source(f.SourceID)
	if err != nil {
		s.fail(w, err)
		return remux.Source{}, false
	}
	return remux.Source{ID: f.ID, Fingerprint: f.Fingerprint, Path: filepath.Join(src.Path, filepath.FromSlash(f.RelPath))}, true
}

var segRe = regexp.MustCompile(`^(0|[1-9][0-9]{0,6})\.m4s$`)

// hls: /files/{id}/hls/{variant}/master.m3u8?s=SESIÓN&t=SEGUNDOS · index.m3u8 · init.mp4 · N.m4s
// variant: «a<índice de la pista de audio>-copy|h264» o «an-…» sin audio.
func (wb *Web) hls(w http.ResponseWriter, r *http.Request) {
	if wb.Remux == nil {
		http.Error(w, "conversión en vivo desactivada en el servidor", http.StatusNotImplemented)
		return
	}
	v, ok := remux.ParseVariant(r.PathValue("variant"))
	if !ok {
		http.Error(w, "variante no válida", http.StatusBadRequest)
		return
	}
	src, ok := wb.source(w, r.PathValue("id"))
	if !ok {
		return
	}
	q := r.URL.Query()
	session := q.Get("s")
	from, _ := strconv.Atoi(q.Get("from"))
	ctx := r.Context()
	name := r.PathValue("name")
	var err error
	switch {
	case name == "master.m3u8" || name == "index.m3u8":
		if t, perr := strconv.ParseFloat(q.Get("t"), 64); perr == nil && t > 0 && name == "master.m3u8" {
			if from, err = wb.Remux.SegmentFor(ctx, src, v, t); err != nil {
				break
			}
		}
		query := "s=" + urlSafe(session) + "&from=" + strconv.Itoa(from)
		var body string
		if name == "master.m3u8" {
			body, err = wb.Remux.MasterPlaylist(ctx, src, v, query)
		} else {
			body, err = wb.Remux.MediaPlaylist(ctx, src, v, query)
		}
		if err == nil {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, body)
			return
		}
	case name == "init.mp4" || segRe.MatchString(name):
		var path string
		t0 := time.Now()
		if name == "init.mp4" {
			path, err = wb.Remux.Init(ctx, src, v, session, from)
		} else {
			n, _ := strconv.Atoi(strings.TrimSuffix(name, ".m4s"))
			path, err = wb.Remux.Segment(ctx, src, v, session, n)
		}
		if err == nil {
			if d := time.Since(t0); d > 3*time.Second {
				wb.API.Log.Printf("web: %s %s %s tardó %s", src.ID, v, name, d.Round(time.Millisecond))
			}
			serveCached(w, r, path)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	wb.remuxError(w, r, src.ID, err)
}

var sessionRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func urlSafe(s string) string {
	if sessionRe.MatchString(s) {
		return s
	}
	return "default"
}

func serveCached(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "segmento no disponible", http.StatusServiceUnavailable)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

func (wb *Web) remuxError(w http.ResponseWriter, r *http.Request, id string, err error) {
	switch {
	case errors.Is(err, remux.ErrBusy):
		w.Header().Set("Retry-After", "10")
		http.Error(w, "El servidor ya está convirtiendo otros vídeos; inténtalo en un momento.", http.StatusServiceUnavailable)
	case errors.Is(err, remux.ErrVariant):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, remux.ErrNoEncoder):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, remux.ErrRange):
		http.NotFound(w, r)
	case errors.Is(err, remux.ErrTimeout):
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
	case errors.Is(err, context.Canceled) || r.Context().Err() != nil:
		// el reproductor ya no lo quiere
	default:
		wb.API.Log.Printf("web: %s: %v", id, err)
		http.Error(w, "no se pudo convertir el vídeo", http.StatusInternalServerError)
	}
}

func (wb *Web) stop(w http.ResponseWriter, r *http.Request) {
	if v, ok := remux.ParseVariant(r.PathValue("variant")); ok && wb.Remux != nil {
		if f, err := wb.API.DB.File(r.PathValue("id")); err == nil {
			wb.Remux.Stop(remux.Source{ID: f.ID, Fingerprint: f.Fingerprint}, v, r.URL.Query().Get("s"))
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// vtt: la misma pista que /subtitles/{sid}, en WebVTT para <track> (se ve también en la
// pantalla completa nativa del iPhone, donde no se puede dibujar HTML encima del vídeo).
func (wb *Web) vtt(w http.ResponseWriter, r *http.Request) {
	s := wb.API
	f, path, tracks, _, err := s.fileTracks(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	cues, err := s.Subs.Cues(r.Context(), f.ID, f.Fingerprint, path, tracks, r.PathValue("sid"))
	if err != nil {
		s.Log.Printf("subtítulos %s/%s: %v", f.ID, r.PathValue("sid"), err)
		http.Error(w, "no se pudieron preparar los subtítulos", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	fmt.Fprint(w, WebVTT(cues))
}

// WebVTT convierte las líneas [inicio, fin, texto] de subs.
func WebVTT(cues []subs.Cue) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	for _, c := range cues {
		start, ok1 := num(c[0])
		end, ok2 := num(c[1])
		text, _ := c[2].(string)
		if !ok1 || !ok2 || end <= start {
			continue
		}
		var lines []string
		for _, l := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, esc.Replace(l))
			}
		}
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", vttTime(start), vttTime(end), strings.Join(lines, "\n"))
	}
	return b.String()
}

func num(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	}
	return 0, false
}

func vttTime(t float64) string {
	ms := int64(t*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// static sirve web/: solo archivos (sin listados de carpetas ni las pruebas).
func (wb *Web) static() http.Handler {
	var files http.Handler
	if wb.Dir != "" {
		files = http.FileServer(http.Dir(wb.Dir))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		if files == nil || strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/tests/") || strings.Contains(p, "/.") || (p != "/" && strings.HasSuffix(p, "/")) {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; worker-src 'self' blob:; style-src 'self' 'unsafe-inline'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		files.ServeHTTP(w, r)
	})
}
