// Package api expone /api/v1 para la TV (contrato en docs/API.md).
// Solo LAN: rechaza IP que no sean privadas o de bucle local.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
	"github.com/eduardoddddddd/YourFlix/server/internal/images"
	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

type Server struct {
	DB       *store.DB
	Images   *images.Cache
	Profile  *compat.Profile
	Version  string
	ServerID string
	Log      *log.Logger
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ping", s.ping)
	mux.HandleFunc("GET /api/v1/titles", s.titles)
	mux.HandleFunc("GET /api/v1/titles/{id}", s.title)
	mux.HandleFunc("GET /api/v1/titles/{id}/{img}", s.titleImage)
	mux.HandleFunc("GET /api/v1/files/{id}/stream", s.stream)
	mux.HandleFunc("HEAD /api/v1/files/{id}/stream", s.stream)
	mux.HandleFunc("PUT /api/v1/progress/{id}", s.putProgress)
	mux.HandleFunc("GET /api/v1/sources", s.sources)
	return s.lanOnly(cors(mux))
}

func (s *Server) lanOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if ip == nil || !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
			http.Error(w, "solo red local", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Range")
		h.Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, OPTIONS")
		h.Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "no encontrado", http.StatusNotFound)
		return
	}
	s.Log.Printf("error: %v", err)
	http.Error(w, "error interno", http.StatusInternalServerError)
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"name": "YourFlix", "version": s.Version, "serverId": s.ServerID, "api": 1})
}

func imgURL(titleID, kind, ref string, width int) string {
	if ref == "" {
		return ""
	}
	return "/api/v1/titles/" + url.PathEscape(titleID) + "/" + kind + "?w=" + strconv.Itoa(width)
}

func (s *Server) titles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	off, _ := strconv.Atoi(q.Get("offset"))
	lim, _ := strconv.Atoi(q.Get("limit"))
	items, total, err := s.DB.ListTitles(store.TitleQuery{Kind: q.Get("kind"), Q: q.Get("q"), Sort: q.Get("sort"), Offset: off, Limit: lim})
	if err != nil {
		s.fail(w, err)
		return
	}
	for i := range items {
		items[i].Poster = imgURL(items[i].ID, "poster", items[i].Poster, 300)
		items[i].Backdrop = imgURL(items[i].ID, "backdrop", items[i].Backdrop, 1280)
	}
	writeJSON(w, map[string]any{"total": total, "offset": off, "items": items})
}

type fileOut struct {
	*store.File
	Compat   compat.Verdict  `json:"compat"`
	Progress *store.Progress `json:"progress,omitempty"`
	Stream   string          `json:"stream"`
}

func (s *Server) title(w http.ResponseWriter, r *http.Request) {
	t, err := s.DB.Title(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	files, err := s.DB.FilesOfTitle(t.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]fileOut, 0, len(files))
	for _, f := range files {
		var v *probe.Video
		var a []probe.Audio
		var sb []probe.Subtitle
		json.Unmarshal(f.Video, &v)
		json.Unmarshal(f.Audio, &a)
		json.Unmarshal(f.Subs, &sb)
		p, _ := s.DB.ProgressOf(f.ID)
		out = append(out, fileOut{File: f, Compat: s.Profile.Check(f.Container, f.Bitrate, v, a, sb), Progress: p, Stream: "/api/v1/files/" + f.ID + "/stream"})
	}
	writeJSON(w, map[string]any{
		"title":    t,
		"poster":   imgURL(t.ID, "poster", t.Poster, 600),
		"backdrop": imgURL(t.ID, "backdrop", t.Backdrop, 1920),
		"files":    out,
	})
}

func (s *Server) titleImage(w http.ResponseWriter, r *http.Request) {
	t, err := s.DB.Title(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	ref := map[string]string{"poster": t.Poster, "backdrop": t.Backdrop}[r.PathValue("img")]
	if ref == "" {
		http.NotFound(w, r)
		return
	}
	width, _ := strconv.Atoi(r.URL.Query().Get("w"))
	if !images.Widths[width] {
		width = 300
	}
	path, err := s.Images.Get(r.Context(), ref, width)
	if err != nil {
		s.Log.Printf("imagen %s/%s: %v", t.ID, r.PathValue("img"), err)
		http.Error(w, "imagen no disponible", http.StatusBadGateway)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, path)
}

var mime = map[string]string{"mkv": "video/x-matroska", "mp4": "video/mp4", "avi": "video/x-msvideo", "ts": "video/mp2t"}

// stream sirve el archivo con Range (206) gracias a http.ServeContent.
// La ruta real nunca sale de la base de datos: el cliente solo conoce el ID.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	f, err := s.DB.File(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if f.MissingSince != "" {
		http.Error(w, "el disco de este archivo no está conectado", http.StatusGone)
		return
	}
	src, err := s.DB.Source(f.SourceID)
	if err != nil {
		s.fail(w, err)
		return
	}
	fh, err := openTimeout(filepath.Join(src.Path, filepath.FromSlash(f.RelPath)), 15*time.Second)
	if err != nil {
		s.Log.Printf("stream %s: %v", f.ID, err)
		http.Error(w, "no se puede leer el archivo", http.StatusServiceUnavailable)
		return
	}
	defer fh.Close()
	if ct := mime[f.Container]; ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	start := time.Now()
	http.ServeContent(w, r, "", time.Unix(f.Mtime, 0), fh)
	if rg := r.Header.Get("Range"); rg != "" && !strings.HasPrefix(rg, "bytes=0-") {
		s.Log.Printf("stream %s %s (%d ms)", f.ID, rg, time.Since(start).Milliseconds())
	}
}

// openTimeout: un disco que no despierta o un permiso de macOS pendiente bloquean os.Open;
// mejor un 503 que AVPlay esperando sin fin.
func openTimeout(path string, d time.Duration) (*os.File, error) {
	type res struct {
		f   *os.File
		err error
	}
	ch := make(chan res, 1)
	go func() { f, err := os.Open(path); ch <- res{f, err} }()
	select {
	case r := <-ch:
		return r.f, r.err
	case <-time.After(d):
		go func() {
			if r := <-ch; r.f != nil {
				r.f.Close() // llegó tarde: no dejar el descriptor abierto
			}
		}()
		return nil, errors.New("el disco no responde")
	}
}

func (s *Server) putProgress(w http.ResponseWriter, r *http.Request) {
	var p store.Progress
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&p); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	if _, err := s.DB.File(r.PathValue("id")); err != nil {
		s.fail(w, err)
		return
	}
	p.FileID = r.PathValue("id")
	if p.Duration > 0 && p.Position/p.Duration > 0.92 {
		p.Watched = true
	}
	if err := s.DB.SaveProgress(p); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sources(w http.ResponseWriter, r *http.Request) {
	list, err := s.DB.Sources()
	if err != nil {
		s.fail(w, err)
		return
	}
	// La TV solo necesita etiqueta y estado; las rutas no salen del servidor.
	type pub struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Kind  string `json:"kind"`
		State string `json:"state"`
		Files int    `json:"files"`
	}
	out := []pub{}
	for _, x := range list {
		out = append(out, pub{x.ID, x.Label, x.Kind, x.State, x.Files})
	}
	writeJSON(w, out)
}
