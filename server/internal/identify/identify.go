// Package identify completa con TMDB las fichas provisionales (deducidas del nombre del
// archivo). Asigna solo cuando la coincidencia es clara; si no, deja la ficha pendiente
// con sus candidatos para elegir a mano. Lo que ya identificó la semilla no se toca.
package identify

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/parse"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
	"github.com/eduardoddddddd/YourFlix/server/internal/tmdb"
)

type Candidate struct {
	TMDB  int64   `json:"tmdb"`
	Title string  `json:"title"`
	En    string  `json:"english,omitempty"`
	Orig  string  `json:"original,omitempty"`
	Year  string  `json:"year,omitempty"`
	Score float64 `json:"score"`
	Sim   float64 `json:"-"`
	Votes int     `json:"votes"`
}

type Report struct {
	Tried      int      `json:"tried"`
	Identified int      `json:"identified"`
	Merged     int      `json:"merged"`
	Pending    int      `json:"pending"`
	Skipped    int      `json:"skipped"`
	Episodes   int      `json:"seriesWithNewEpisodes"`
	Errors     []string `json:"errors,omitempty"`
}

type Options struct {
	Retry bool          // reintentar también las que ya quedaron pendientes
	Every time.Duration // reintento automático de pendientes (por defecto 7 días)
	Log   func(format string, args ...any)
}

// Pending recorre las fichas provisionales y las identifica.
func Pending(ctx context.Context, db *store.DB, c *tmdb.Client, opt Options) (*Report, error) {
	if !c.Enabled() {
		return nil, tmdb.ErrNoCredentials
	}
	if opt.Log == nil {
		opt.Log = func(string, ...any) {}
	}
	if opt.Every == 0 {
		opt.Every = 7 * 24 * time.Hour
	}
	rep := &Report{}
	// Series ya identificadas (semilla o TMDB) con capítulos sin nombre: completarlos.
	if series, err := db.SeriesMissingEpisodes(); err == nil {
		for _, t := range series {
			if addEpisodes(ctx, db, c, t) {
				rep.Episodes++
			}
		}
	}
	titles, err := db.PendingTitles()
	if err != nil {
		return nil, err
	}
	for _, t := range titles {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		var m struct {
			IdentifyTried string `json:"identifyTried"`
		}
		json.Unmarshal(t.Meta, &m)
		if tried, err := time.Parse(time.RFC3339, m.IdentifyTried); err == nil && !opt.Retry && time.Since(tried) < opt.Every {
			rep.Skipped++
			continue
		}
		rep.Tried++
		name := strings.TrimSpace(t.Title + " " + t.Year)
		res, err := One(ctx, db, c, t)
		switch {
		case err != nil:
			rep.Errors = append(rep.Errors, name+": "+err.Error())
			opt.Log("  %s: %v", name, err)
		case res.Merged:
			rep.Merged++
			opt.Log("  %s → ya estaba: %s (%s)", name, res.Title, res.Year)
		case res.Identified:
			rep.Identified++
			opt.Log("  %s → %s (%s) · TMDB %d · confianza %.2f", name, res.Title, res.Year, res.TMDB, res.Score)
		default:
			rep.Pending++
			opt.Log("  %s → pendiente (%d candidatos)", name, res.Candidates)
		}
	}
	return rep, nil
}

type Outcome struct {
	Identified, Merged bool
	Title, Year        string
	TMDB               int64
	Score              float64
	Candidates         int
}

// One intenta identificar una ficha provisional.
func One(ctx context.Context, db *store.DB, c *tmdb.Client, t *store.Title) (*Outcome, error) {
	cands, err := candidates(ctx, c, t.Kind, t.Title, t.Year)
	if err != nil {
		return nil, err
	}
	best, ok := decide(cands, parse.Normalize(t.Title), t.Year)
	if !ok {
		meta := map[string]any{}
		json.Unmarshal(t.Meta, &meta)
		top := cands
		if len(top) > 5 {
			top = top[:5]
		}
		meta["candidates"] = top
		meta["identifyTried"] = store.Now()
		t.Meta, _ = json.Marshal(meta)
		return &Outcome{Candidates: len(top)}, db.UpsertTitle(t, false)
	}
	return Apply(ctx, db, c, t, best.TMDB, best.Score, false)
}

// Apply asigna la ficha TMDB id a la ficha provisional t (fusionando si ya existe).
// manual=true bloquea la ficha para que nada la cambie después.
func Apply(ctx context.Context, db *store.DB, c *tmdb.Client, t *store.Title, id int64, score float64, manual bool) (*Outcome, error) {
	source := "tmdb"
	if manual {
		source = "manual"
	}
	if existing, err := db.TitleByTMDB(t.Kind, id); err != nil {
		return nil, err
	} else if existing != nil && existing.ID != t.ID {
		// p. ej. un archivo nuevo de una película que ya estaba en la semilla
		if err := db.MoveFiles(t.ID, existing.ID, source); err != nil {
			return nil, err
		}
		if t.Kind == "series" {
			addEpisodes(ctx, db, c, existing)
		}
		return &Outcome{Merged: true, Title: existing.Title, Year: existing.Year, TMDB: id, Score: score}, nil
	}
	d, err := c.Details(ctx, t.Kind, id)
	if err != nil {
		return nil, err
	}
	fill(t, d, score)
	t.MatchSource, t.Locked = source, manual
	if err := db.UpsertTitle(t, manual); err != nil {
		return nil, err
	}
	if err := db.SetFilesMatchSource(t.ID, source); err != nil {
		return nil, err
	}
	if t.Kind == "series" {
		addEpisodes(ctx, db, c, t)
	}
	return &Outcome{Identified: true, Title: t.Title, Year: t.Year, TMDB: id, Score: score}, nil
}

// candidates busca en español y en inglés (los nombres de archivo vienen en cualquiera de
// los dos: «Parasite» es «Parásitos» en España), con año y, si no sale nada, sin él.
func candidates(ctx context.Context, c *tmdb.Client, kind, title, year string) ([]Candidate, error) {
	byID := map[int64]*Candidate{}
	var order []int64
	for _, lang := range []string{"es-ES", "en-US"} {
		results, err := c.Search(ctx, kind, title, year, lang)
		if err != nil {
			return nil, err
		}
		if len(results) == 0 && year != "" {
			if results, err = c.Search(ctx, kind, title, "", lang); err != nil {
				return nil, err
			}
		}
		for _, r := range results {
			cd := byID[r.ID]
			if cd == nil {
				cd = &Candidate{TMDB: r.ID, Title: r.DisplayTitle(), Orig: r.Original(), Year: r.Year()}
				byID[r.ID] = cd
				order = append(order, r.ID)
			} else if lang == "en-US" {
				cd.En = r.DisplayTitle()
			}
			cd.Votes = max(cd.Votes, r.VoteCount)
		}
	}
	q := parse.Normalize(title)
	out := make([]Candidate, 0, len(order))
	for _, id := range order {
		cd := *byID[id]
		for _, name := range []string{cd.Title, cd.En, cd.Orig} {
			cd.Sim = math.Max(cd.Sim, similarity(q, parse.Normalize(name)))
		}
		cd.Score = cd.Sim + yearScore(year, cd.Year) + math.Min(0.08, math.Log10(float64(cd.Votes)+1)*0.02)
		cd.Score = math.Round(cd.Score*100) / 100
		out = append(out, cd)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

func yearScore(want, got string) float64 {
	if want == "" || got == "" {
		return 0
	}
	a, _ := strconv.Atoi(want)
	b, _ := strconv.Atoi(got)
	switch d := a - b; {
	case d == 0:
		return 0.15
	case d == 1 || d == -1:
		return 0.05 // fechas de estreno distintas según el país
	default:
		return -0.3
	}
}

// decide: título casi idéntico, año compatible y sin un rival igual de bueno.
func decide(c []Candidate, q, year string) (Candidate, bool) {
	if len(c) == 0 {
		return Candidate{}, false
	}
	best := c[0]
	if best.Sim < 0.88 || (year != "" && yearScore(year, best.Year) < 0) {
		return best, false
	}
	// Sin año y con un nombre muy corto («s4», «Up»): solo si es muy conocida.
	if year == "" && len([]rune(q)) < 4 && best.Votes < 1000 {
		return best, false
	}
	if len(c) > 1 {
		second := c[1]
		if year == "" && second.Sim >= best.Sim-0.02 {
			// «Alien» sin año: Alien (1979) y otras con el mismo nombre. Solo si una es
			// muchísimo más conocida (la de 1979 frente a un corto homónimo).
			if best.Votes < 5*second.Votes || best.Votes < 200 {
				return best, false
			}
		}
		if best.Score-second.Score < 0.05 && second.Sim >= 0.88 {
			return best, false
		}
	}
	return best, true
}

// similarity: coeficiente de Dice sobre bigramas de caracteres (1 = idénticos).
func similarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	bg := func(s string) map[string]int {
		r := []rune(s)
		m := map[string]int{}
		for i := 0; i+1 < len(r); i++ {
			m[string(r[i:i+2])]++
		}
		return m
	}
	x, y := bg(a), bg(b)
	inter, total := 0, 0
	for k, n := range x {
		total += n
		if m := y[k]; m > 0 {
			inter += min(n, m)
		}
	}
	for _, n := range y {
		total += n
	}
	if total == 0 {
		return 0
	}
	return 2 * float64(inter) / float64(total)
}

func img(size, path string) string {
	if path == "" {
		return ""
	}
	return tmdb.ImageBase + size + path
}

// fill vuelca la ficha de TMDB en el formato de la semilla (cine-t7-catalogo).
func fill(t *store.Title, d *tmdb.Details, score float64) {
	title, orig, date := d.Title, d.OriginalTitle, d.ReleaseDate
	if t.Kind == "series" {
		title, orig, date = d.Name, d.OriginalName, d.FirstAirDate
	}
	year := ""
	if len(date) >= 4 {
		year = date[:4]
	}
	var genres, countries, director, cast []string
	for _, g := range d.Genres {
		genres = append(genres, g.Name)
	}
	for _, c := range d.Countries {
		countries = append(countries, c.Name)
	}
	if t.Kind == "series" {
		for _, p := range d.CreatedBy {
			director = append(director, p.Name)
		}
	} else {
		for _, p := range d.Credits.Crew {
			if p.Job == "Director" {
				director = append(director, p.Name)
			}
		}
	}
	for i, p := range d.Credits.Cast {
		if i == 10 {
			break
		}
		cast = append(cast, p.Name)
	}
	runtime := d.Runtime
	if runtime == 0 && len(d.EpisodeRun) > 0 {
		runtime = d.EpisodeRun[0]
	}
	imdb := d.IMDB
	if imdb == "" {
		imdb = d.ExternalIDs.IMDB
	}
	meta := map[string]any{
		"matched": true, "tmdb": d.ID, "imdb": imdb, "title": title, "original": orig, "year": year,
		"synopsis": d.Overview, "genres": nz(genres), "runtime": runtime, "rating": d.VoteAverage, "votes": d.VoteCount,
		"tagline": d.Tagline, "country": join(countries), "director": nz(director), "cast": nz(cast),
		"source": "tmdb", "confidence": score, "checkedAt": time.Now().Format("2006-01-02"),
	}
	if d.NumberOfSeasons > 0 {
		meta["seasons"] = d.NumberOfSeasons
	}
	t.Title, t.Original, t.Year, t.TMDB, t.IMDB = title, orig, year, d.ID, imdb
	t.Norm, t.NormOrig = parse.Normalize(title), parse.Normalize(orig)
	t.Poster, t.Backdrop = img("w500", d.PosterPath), img("w1280", d.BackdropPath)
	t.Meta, _ = json.Marshal(meta)
}

// addEpisodes añade a la ficha de una serie el nombre, la sinopsis y la foto de cada
// capítulo que hay en disco, en meta.episodes["T×E"].
func addEpisodes(ctx context.Context, db *store.DB, c *tmdb.Client, t *store.Title) bool {
	if t.TMDB == 0 {
		return false
	}
	files, err := db.FilesOfTitle(t.ID)
	if err != nil {
		return false
	}
	seasons := map[int]bool{}
	for _, f := range files {
		if f.Season != nil {
			seasons[*f.Season] = true
		}
	}
	meta := map[string]any{}
	json.Unmarshal(t.Meta, &meta)
	eps, _ := meta["episodes"].(map[string]any)
	if eps == nil {
		eps = map[string]any{}
	}
	added := false
	for s := range seasons {
		list, err := c.Season(ctx, t.TMDB, s)
		if err != nil {
			continue
		}
		added = added || len(list) > 0
		for _, e := range list {
			eps[fmt.Sprintf("%dx%d", e.Season, e.Episode)] = map[string]any{"name": e.Name, "overview": e.Overview, "still": img("w300", e.Still), "airDate": e.AirDate, "runtime": e.Runtime}
		}
	}
	if !added {
		return false
	}
	meta["episodes"] = eps
	t.Meta, _ = json.Marshal(meta)
	db.UpsertTitle(t, t.Locked)
	return true
}

func nz(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}
