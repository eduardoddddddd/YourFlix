package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/identify"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
	"github.com/eduardoddddddd/YourFlix/server/internal/tmdb"
)

// Catálogo: fichas pendientes con sus candidatos de TMDB, emparejado manual (como
// «yourflix match»: la ficha queda bloqueada), búsqueda en TMDB e identificación.

type fileRef struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Path    string `json:"path"` // relativa a la fuente (solo admin)
	Size    int64  `json:"size"`
	Season  *int   `json:"season,omitempty"`
	Episode *int   `json:"episode,omitempty"`
}

type pendingOut struct {
	ID            string               `json:"id"`
	Kind          string               `json:"kind"`
	Title         string               `json:"title"`
	Year          string               `json:"year,omitempty"`
	Files         []fileRef            `json:"files"`
	Candidates    []identify.Candidate `json:"candidates"`
	IdentifyTried string               `json:"identifyTried,omitempty"`
}

func refs(files []*store.File) []fileRef {
	out := []fileRef{}
	for _, f := range files {
		out = append(out, fileRef{ID: f.ID, Source: f.SourceLabel, Path: f.RelPath, Size: f.Size, Season: f.Season, Episode: f.Episode})
	}
	return out
}

func (a *Admin) pending(w http.ResponseWriter, r *http.Request) {
	ts, err := a.cfg.DB.PendingTitles()
	if err != nil {
		a.fail(w, err)
		return
	}
	out := []pendingOut{}
	for _, t := range ts {
		files, err := a.cfg.DB.FilesOfTitle(t.ID)
		if err != nil {
			a.fail(w, err)
			return
		}
		var m struct {
			Candidates    []identify.Candidate `json:"candidates"`
			IdentifyTried string               `json:"identifyTried"`
		}
		json.Unmarshal(t.Meta, &m)
		if m.Candidates == nil {
			m.Candidates = []identify.Candidate{}
		}
		out = append(out, pendingOut{ID: t.ID, Kind: t.Kind, Title: t.Title, Year: t.Year, Files: refs(files), Candidates: m.Candidates, IdentifyTried: m.IdentifyTried})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "tmdb": a.cfg.TMDB.Enabled()})
}

// titles: buscar cualquier ficha del catálogo (para corregir un emparejado equivocado).
func (a *Admin) titles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, total, err := a.cfg.DB.ListTitles(store.TitleQuery{Q: q.Get("q"), Kind: q.Get("kind"), Limit: 50})
	if err != nil {
		a.fail(w, err)
		return
	}
	type out struct {
		ID          string    `json:"id"`
		Kind        string    `json:"kind"`
		Title       string    `json:"title"`
		Year        string    `json:"year,omitempty"`
		TMDB        int64     `json:"tmdb,omitempty"`
		MatchSource string    `json:"matchSource"`
		Locked      bool      `json:"locked"`
		Poster      string    `json:"poster,omitempty"`
		Files       []fileRef `json:"files"`
	}
	res := []out{}
	for _, it := range items {
		t, err := a.cfg.DB.Title(it.ID)
		if err != nil {
			continue
		}
		files, _ := a.cfg.DB.FilesOfTitle(t.ID)
		o := out{ID: t.ID, Kind: t.Kind, Title: t.Title, Year: t.Year, TMDB: t.TMDB, MatchSource: t.MatchSource, Locked: t.Locked, Files: refs(files)}
		if t.Poster != "" {
			o.Poster = "/api/v1/titles/" + url.PathEscape(t.ID) + "/poster?w=185"
		}
		res = append(res, o)
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": res})
}

func (a *Admin) tmdbReady(w http.ResponseWriter) bool {
	if !a.cfg.TMDB.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "TMDB no está configurado en el servidor (--tmdb-token-file o TMDB_ACCESS_TOKEN)")
		return false
	}
	return true
}

// tmdbErr: los errores del cliente de TMDB ya no llevan credenciales; aun así se
// devuelven sin la URL y se registran sin el token.
func (a *Admin) tmdbErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	if c := a.cfg.TMDB; c != nil {
		for _, s := range []string{c.Token, c.APIKey} {
			if len(s) >= 6 {
				msg = strings.ReplaceAll(msg, s, "[oculto]")
			}
		}
	}
	a.cfg.Log.Printf("admin: TMDB: %s", msg)
	writeError(w, http.StatusBadGateway, msg)
}

// tmdbSearch: GET /admin/api/tmdb/search?q=&year=&kind=movie|series
func (a *Admin) tmdbSearch(w http.ResponseWriter, r *http.Request) {
	if !a.tmdbReady(w) {
		return
	}
	q := r.URL.Query()
	query, year, kind := strings.TrimSpace(q.Get("q")), strings.TrimSpace(q.Get("year")), q.Get("kind")
	if query == "" {
		writeError(w, http.StatusBadRequest, "escribe un título")
		return
	}
	if kind != "series" {
		kind = "movie"
	}
	if year != "" {
		if n, err := strconv.Atoi(year); err != nil || n < 1870 || n > 2200 {
			writeError(w, http.StatusBadRequest, "año no válido")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := a.cfg.TMDB.Search(ctx, kind, query, year, "es-ES")
	if err == nil && len(res) == 0 && year != "" {
		res, err = a.cfg.TMDB.Search(ctx, kind, query, "", "es-ES")
	}
	if err != nil {
		a.tmdbErr(w, err)
		return
	}
	type out struct {
		TMDB       int64   `json:"tmdb"`
		Title      string  `json:"title"`
		Original   string  `json:"original,omitempty"`
		Year       string  `json:"year,omitempty"`
		Votes      int     `json:"votes"`
		Popularity float64 `json:"popularity"`
		Poster     string  `json:"poster,omitempty"`
	}
	items := []out{}
	for i, x := range res {
		if i == 20 {
			break
		}
		o := out{TMDB: x.ID, Title: x.DisplayTitle(), Original: x.Original(), Year: x.Year(), Votes: x.VoteCount, Popularity: x.Popularity}
		if x.PosterPath != "" {
			o.Poster = tmdb.ImageBase + "w92" + x.PosterPath
		}
		items = append(items, o)
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "items": items})
}

// match: POST /admin/api/titles/{id}/match {"tmdb": 78} — lo mismo que
// «yourflix match ID --tmdb 78»: la ficha queda bloqueada (manual).
func (a *Admin) match(w http.ResponseWriter, r *http.Request) {
	if !a.tmdbReady(w) {
		return
	}
	var in struct {
		TMDB int64 `json:"tmdb"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.TMDB <= 0 {
		writeError(w, http.StatusBadRequest, "falta el ID de TMDB")
		return
	}
	t, err := a.cfg.DB.Title(r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	before := t.Title
	o, err := identify.Apply(ctx, a.cfg.DB, a.cfg.TMDB, t, in.TMDB, 1, true)
	if err != nil {
		a.tmdbErr(w, err)
		return
	}
	a.cfg.Log.Printf("admin: «%s» → %s (%s), TMDB %d (manual, bloqueada)", before, o.Title, o.Year, in.TMDB)
	writeJSON(w, http.StatusOK, map[string]any{"title": o.Title, "year": o.Year, "tmdb": o.TMDB, "merged": o.Merged, "locked": !o.Merged})
}

// identify: POST /admin/api/identify {"retry": false} — «yourflix identify [--retry]».
func (a *Admin) identify(w http.ResponseWriter, r *http.Request) {
	if !a.tmdbReady(w) {
		return
	}
	var in struct {
		Retry bool `json:"retry"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	title := "Identificar con TMDB"
	if in.Retry {
		title += " (reintentar pendientes)"
	}
	a.respondJob(w)(a.jobs.enqueue("identify", "identify:"+strconv.FormatBool(in.Retry), title, func(ctx context.Context, p *progress) error {
		rep, err := identify.Pending(ctx, a.cfg.DB, a.cfg.TMDB, identify.Options{Retry: in.Retry, Log: func(f string, v ...any) { p.Line(true, strings.TrimSpace(f), v...) }})
		if err != nil {
			return err
		}
		p.Summary("TMDB: %d probadas · %d identificadas · %d fusionadas con fichas existentes · %d pendientes · %d sin reintentar · %d series con capítulos nuevos · %d errores",
			rep.Tried, rep.Identified, rep.Merged, rep.Pending, rep.Skipped, rep.Episodes, len(rep.Errors))
		for _, e := range rep.Errors {
			p.Summary("error: %s", e)
		}
		return nil
	}))
}
