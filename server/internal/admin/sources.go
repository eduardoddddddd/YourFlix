package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/identify"
	"github.com/eduardoddddddd/YourFlix/server/internal/scan"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

func (a *Admin) sourcesList(w http.ResponseWriter, r *http.Request) {
	list, err := a.cfg.DB.Sources()
	if err != nil {
		a.fail(w, err)
		return
	}
	if list == nil {
		list = []store.Source{}
	}
	writeJSON(w, http.StatusOK, list)
}

type sourceIn struct {
	Path     string   `json:"path"`
	Label    *string  `json:"label"`
	Kind     *string  `json:"kind"`
	Excludes []string `json:"excludes"`
	Scan     bool     `json:"scan"`
}

func cleanList(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// overlap: otra fuente que contiene a p o está dentro de ella (no se impide, se avisa:
// los archivos saldrían dos veces como copias).
func overlap(list []store.Source, p string) string {
	in := func(child, parent string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	for _, s := range list {
		if in(p, s.Path) || in(s.Path, p) {
			return s.Label
		}
	}
	return ""
}

func (a *Admin) sourceAdd(w http.ResponseWriter, r *http.Request) {
	var in sourceIn
	if !readJSON(w, r, &in) {
		return
	}
	p := strings.TrimSpace(in.Path)
	if p == "" || !filepath.IsAbs(p) {
		writeError(w, http.StatusBadRequest, "elige una carpeta (ruta absoluta)")
		return
	}
	p = filepath.Clean(p)
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		writeError(w, http.StatusBadRequest, p+" no es una carpeta accesible")
		return
	}
	list, err := a.cfg.DB.Sources()
	if err != nil {
		a.fail(w, err)
		return
	}
	for _, s := range list {
		if filepath.Clean(s.Path) == p {
			writeError(w, http.StatusConflict, "esa carpeta ya es la fuente «"+s.Label+"»")
			return
		}
	}
	warning := ""
	if o := overlap(list, p); o != "" {
		warning = "Se solapa con la fuente «" + o + "»: los vídeos comunes aparecerán como dos copias."
	}
	s, err := a.cfg.DB.AddSource(deref(in.Label), p, deref(in.Kind), cleanList(in.Excludes))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.cfg.Log.Printf("admin: fuente añadida «%s»", s.Label)
	out := map[string]any{"source": s}
	if warning != "" {
		out["warning"] = warning
	}
	if in.Scan {
		if j, err := a.enqueueScan([]string{s.ID}); err == nil {
			out["job"] = j
		}
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *Admin) sourceUpdate(w http.ResponseWriter, r *http.Request) {
	s, err := a.cfg.DB.Source(r.PathValue("id"))
	if err != nil || s.ID != r.PathValue("id") {
		writeError(w, http.StatusNotFound, "fuente desconocida")
		return
	}
	var in sourceIn
	if !readJSON(w, r, &in) {
		return
	}
	label, kind, ex := s.Label, s.Kind, s.Excludes
	if in.Label != nil {
		label = *in.Label
	}
	if in.Kind != nil {
		kind = *in.Kind
	}
	if in.Excludes != nil {
		ex = in.Excludes
	}
	if err := a.cfg.DB.UpdateSource(s.ID, label, kind, ex); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "fuente desconocida")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.cfg.Log.Printf("admin: fuente «%s» actualizada", strings.TrimSpace(label))
	ns, _ := a.cfg.DB.Source(s.ID)
	writeJSON(w, http.StatusOK, ns)
}

func (a *Admin) sourceState(state string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := a.cfg.DB.Source(r.PathValue("id"))
		if err != nil || s.ID != r.PathValue("id") {
			writeError(w, http.StatusNotFound, "fuente desconocida")
			return
		}
		if err := a.cfg.DB.SetSourceState(s.ID, state); err != nil {
			a.fail(w, err)
			return
		}
		a.cfg.Log.Printf("admin: fuente «%s» → %s", s.Label, state)
		ns, _ := a.cfg.DB.Source(s.ID)
		writeJSON(w, http.StatusOK, ns)
	}
}

// sourceRemove saca la fuente del catálogo (sus archivos y su progreso, en la base de
// datos). Nunca toca el disco.
func (a *Admin) sourceRemove(w http.ResponseWriter, r *http.Request) {
	s, err := a.cfg.DB.Source(r.PathValue("id"))
	if err != nil || s.ID != r.PathValue("id") {
		writeError(w, http.StatusNotFound, "fuente desconocida")
		return
	}
	if a.cfg.Lock != nil {
		a.cfg.Lock.Lock() // no quitarla a mitad de un escaneo
		defer a.cfg.Lock.Unlock()
	}
	if err := a.cfg.DB.RemoveSource(s.ID); err != nil {
		a.fail(w, err)
		return
	}
	n, _ := a.cfg.DB.DeleteOrphanTitles()
	a.cfg.Log.Printf("admin: fuente «%s» quitada del catálogo (%d archivos; no se ha borrado nada del disco)", s.Label, s.Files)
	writeJSON(w, http.StatusOK, map[string]any{"removed": s.ID, "files": s.Files, "orphanTitles": n})
}

func (a *Admin) sourceScan(w http.ResponseWriter, r *http.Request) {
	s, err := a.cfg.DB.Source(r.PathValue("id"))
	if err != nil || s.ID != r.PathValue("id") {
		writeError(w, http.StatusNotFound, "fuente desconocida")
		return
	}
	a.respondJob(w)(a.enqueueScan([]string{s.ID}))
}

func (a *Admin) scanAll(w http.ResponseWriter, r *http.Request) {
	a.respondJob(w)(a.enqueueScan(nil))
}

func (a *Admin) respondJob(w http.ResponseWriter) func(*Job, error) {
	return func(j *Job, err error) {
		if err != nil {
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, j)
	}
}

// enqueueScan: escanea las fuentes ids (nil = todas) y, como serve, identifica después
// con TMDB lo nuevo si hay credenciales.
func (a *Admin) enqueueScan(ids []string) (*Job, error) {
	key, title := "scan:all", "Escanear todas las fuentes"
	if ids != nil {
		key = "scan:" + strings.Join(ids, ",")
		title = "Escanear"
		if s, err := a.cfg.DB.Source(ids[0]); err == nil {
			title = "Escanear «" + s.Label + "»"
		}
	}
	return a.jobs.enqueue("scan", key, title, func(ctx context.Context, p *progress) error {
		all, err := a.cfg.DB.Sources()
		if err != nil {
			return err
		}
		var list []store.Source
		for _, s := range all {
			if ids == nil || contains(ids, s.ID) {
				list = append(list, s)
			}
		}
		if len(list) == 0 {
			return errors.New("no hay fuentes que escanear")
		}
		for i := range list {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			a.scanSource(ctx, p, &list[i])
		}
		if a.cfg.TMDB.Enabled() {
			p.Line(false, "Identificando lo nuevo con TMDB…")
			rep, err := identify.Pending(ctx, a.cfg.DB, a.cfg.TMDB, identify.Options{Log: func(f string, v ...any) { p.Line(false, f, v...) }})
			if err != nil {
				p.Summary("TMDB: %s", err)
			} else if rep.Tried > 0 || rep.Episodes > 0 {
				p.Summary("TMDB: %d identificadas · %d fusionadas · %d pendientes · %d errores", rep.Identified, rep.Merged, rep.Pending, len(rep.Errors))
				for i, e := range rep.Errors {
					if i == 3 {
						p.Summary("… y %d errores más (ver el registro)", len(rep.Errors)-3)
						break
					}
					p.Summary("error: %s", e)
				}
			}
		}
		return nil
	})
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (a *Admin) scanSource(ctx context.Context, p *progress, s *store.Source) {
	p.Line(false, "Escaneando «%s»…", s.Label)
	t0 := time.Now()
	rep, err := scan.Source(ctx, a.cfg.DB, s, scan.Options{FFprobe: a.cfg.FFprobe, Log: func(f string, v ...any) {
		// «  ruta → origen» por archivo procesado; los errores de lectura también llegan aquí
		p.Line(strings.Contains(f, "→"), strings.TrimSpace(f), v...)
	}})
	run := store.ScanRun{SourceID: s.ID, Label: s.Label, StartedAt: t0.UTC().Format(time.RFC3339), Seconds: time.Since(t0).Seconds()}
	switch {
	case err != nil:
		run.Error = err.Error()
		p.Summary("«%s»: error: %s", s.Label, err)
	case s.State == "paused":
		p.Summary("«%s»: pausada, no se escanea", s.Label)
	case rep.Offline:
		p.Summary("«%s» no está conectada: %d archivos marcados como ausentes", s.Label, rep.Missing)
	default:
		p.Summary("«%s»: %d vídeos · %d nuevos · %d cambiados · %d movidos · %d ausentes · fichas: %d semilla (bytes), %d semilla/TMDB (título), %d solo nombre · %d errores de ffprobe · %s",
			s.Label, rep.Seen, rep.New, rep.Changed, rep.Moved, rep.Missing, rep.MatchedSeedBytes, rep.MatchedSeedTitle, rep.FromFilename, rep.ProbeErrors, time.Since(t0).Round(time.Millisecond))
	}
	if rep != nil {
		run.Report, _ = json.Marshal(rep)
	}
	if s.State != "paused" {
		if err := a.cfg.DB.AddScanRun(run); err != nil {
			a.cfg.Log.Printf("admin: guardar escaneo: %v", err)
		}
	}
	a.cfg.Log.Printf("admin: escaneo de «%s» terminado en %s", s.Label, time.Since(t0).Round(time.Millisecond))
}
