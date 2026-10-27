// Package facets es el índice de categorías en memoria: director, reparto, país (en
// castellano), año, década, duración, nota, calidad, idiomas y estado de cada título con
// algún archivo disponible. Sirve las listas filtradas, los recuentos de «Filtrar por» y el
// orden aleatorio estable del kiosco. No crea tablas: se rehace leyendo la base de datos
// cuando cambia su huella (tras escanear, identificar o editar fichas).
package facets

import (
	"encoding/json"
	"hash/fnv"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

// Entry: las categorías de un título.
type Entry struct {
	ID, Kind, Norm string
	Directors      []string
	Cast           []string
	Countries      []string // en castellano
	Genres         []string
	Year           string
	Decade         string // «1980»
	Runtime        int
	RuntimeBand    string // short | mid | long
	Rating         float64
	Quality        []string // 4k | hdr | 1080 | 720 | sd
	Audio          []string // códigos ISO 639-2 (spa, eng…)
	Subs           []string // solo pistas internas de texto
	files          []string
}

// Index: el índice en memoria. Seguro para usar desde varias peticiones.
type Index struct {
	db      *store.DB
	mu      sync.Mutex
	print   string
	entries []*Entry
	byID    map[string]*Entry
}

func New(db *store.DB) *Index { return &Index{db: db} }

// huella barata de lo que alimenta el índice: si cambia, se rehace.
const fingerprintSQL = `SELECT
	(SELECT COUNT(*)||':'||COALESCE(SUM(length(meta)+length(title)+length(year)+length(kind)),0) FROM titles),
	(SELECT COUNT(*)||':'||COALESCE(SUM(missing_since IS NULL),0)||':'||COALESCE(MAX(added_at),'')||':'||COALESCE(SUM(size),0)||':'||
		COALESCE(SUM(length(COALESCE(title_id,''))+length(video)+length(audio)+length(subs)),0) FROM files)`

func (x *Index) snapshot() ([]*Entry, map[string]*Entry, error) {
	var a, b string
	if err := x.db.QueryRow(fingerprintSQL).Scan(&a, &b); err != nil {
		return nil, nil, err
	}
	fp := a + "|" + b
	x.mu.Lock()
	defer x.mu.Unlock()
	if fp == x.print && x.byID != nil {
		return x.entries, x.byID, nil
	}
	entries, err := build(x.db)
	if err != nil {
		return nil, nil, err
	}
	x.print, x.entries, x.byID = fp, entries, map[string]*Entry{}
	for _, e := range entries {
		x.byID[e.ID] = e
	}
	return x.entries, x.byID, nil
}

func build(db *store.DB) ([]*Entry, error) {
	rows, err := db.Query(`SELECT t.id,t.kind,t.norm,t.year,t.meta FROM titles t
		WHERE EXISTS (SELECT 1 FROM files f WHERE f.title_id=t.id AND f.missing_since IS NULL) ORDER BY t.norm, t.id`)
	if err != nil {
		return nil, err
	}
	var out []*Entry
	byID := map[string]*Entry{}
	for rows.Next() {
		var e Entry
		var meta string
		if err := rows.Scan(&e.ID, &e.Kind, &e.Norm, &e.Year, &meta); err != nil {
			rows.Close()
			return nil, err
		}
		var m struct {
			Director []string `json:"director"`
			Cast     []string `json:"cast"`
			Country  string   `json:"country"`
			Genres   []string `json:"genres"`
			Runtime  int      `json:"runtime"`
			Rating   float64  `json:"rating"`
		}
		json.Unmarshal([]byte(meta), &m)
		e.Directors, e.Cast, e.Genres = clean(m.Director), clean(m.Cast), clean(m.Genres)
		e.Countries = Countries(m.Country)
		e.Runtime, e.RuntimeBand, e.Rating = m.Runtime, Band(m.Runtime), m.Rating
		if y, err := strconv.Atoi(e.Year); err == nil && y > 1000 {
			e.Decade = strconv.Itoa(y / 10 * 10)
		}
		out = append(out, &e)
		byID[e.ID] = &e
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	frows, err := db.Query(`SELECT id,title_id,video,audio,subs FROM files WHERE missing_since IS NULL AND title_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer frows.Close()
	for frows.Next() {
		var id, tid, v, a, s string
		if err := frows.Scan(&id, &tid, &v, &a, &s); err != nil {
			return nil, err
		}
		e := byID[tid]
		if e == nil {
			continue
		}
		e.files = append(e.files, id)
		var vid *probe.Video
		var aud []probe.Audio
		var sub []probe.Subtitle
		json.Unmarshal([]byte(v), &vid)
		json.Unmarshal([]byte(a), &aud)
		json.Unmarshal([]byte(s), &sub)
		if vid != nil {
			e.Quality = add(e.Quality, quality(vid))
			if vid.HDR != "" {
				e.Quality = add(e.Quality, "hdr")
			}
		}
		for _, x := range aud {
			if l := Lang(x.Language); l != "" {
				e.Audio = add(e.Audio, l)
			}
		}
		for _, x := range sub {
			if l := Lang(x.Language); l != "" && x.Text {
				e.Subs = add(e.Subs, l)
			}
		}
	}
	return out, frows.Err()
}

func clean(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = add(out, s)
		}
	}
	return out
}

func add(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func quality(v *probe.Video) string {
	switch {
	case v.Width >= 3000 || v.Height >= 2000:
		return "4k"
	case v.Width >= 1800 || v.Height >= 1000:
		return "1080"
	case v.Width >= 1200 || v.Height >= 700:
		return "720"
	case v.Width > 0:
		return "sd"
	}
	return ""
}

// Band: tramo de duración. 0 = desconocida.
func Band(min int) string {
	switch {
	case min <= 0:
		return ""
	case min < 90:
		return "short"
	case min <= 120:
		return "mid"
	}
	return "long"
}

// Filter: condiciones combinables con Y. Las listas exigen todos sus valores.
type Filter struct {
	Kind                                        string
	Genre, Director, Actor, Person, Country     []string
	Year, Decade, Runtime, Quality, Audio, Subs []string
	State                                       string
	Rating                                      float64
}

// ParseFilter lee los filtros de la dirección (mismos nombres en /titles y /facets).
func ParseFilter(q url.Values) Filter {
	f := Filter{Kind: q.Get("kind"), State: q.Get("state"), Genre: vals(q, "genre"), Director: vals(q, "director"), Actor: vals(q, "actor"),
		Person: vals(q, "person"), Country: vals(q, "country"), Year: vals(q, "year"), Decade: vals(q, "decade"),
		Runtime: vals(q, "runtime"), Quality: vals(q, "quality"), Audio: vals(q, "audio"), Subs: vals(q, "subs")}
	for i, c := range f.Country {
		f.Country[i] = CountryES(c) // también vale el nombre en inglés de TMDB
	}
	for _, list := range [][]string{f.Audio, f.Subs} {
		for i, l := range list {
			list[i] = Lang(l)
		}
	}
	for i, d := range f.Decade {
		if y, err := strconv.Atoi(strings.TrimSuffix(d, "s")); err == nil {
			if y < 100 {
				y += 1900 // «80» → 1980
			}
			f.Decade[i] = strconv.Itoa(y / 10 * 10)
		}
	}
	f.Rating, _ = strconv.ParseFloat(q.Get("rating"), 64)
	return f
}

func vals(q url.Values, k string) []string {
	var out []string
	for _, v := range q[k] {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Active: hay algún filtro de los que solo sabe aplicar el índice (no basta el SQL).
func (f Filter) Active() bool {
	return len(f.Director)+len(f.Actor)+len(f.Person)+len(f.Country)+len(f.Year)+len(f.Decade)+len(f.Runtime)+
		len(f.Quality)+len(f.Audio)+len(f.Subs) > 0 || f.State != "" || f.Rating > 0
}

func all(want, got []string) bool {
	for _, w := range want {
		if !has(got, w) {
			return false
		}
	}
	return true
}

func one(want []string, v string) bool {
	for _, w := range want {
		if w != v {
			return false
		}
	}
	return true
}

func (f Filter) match(e *Entry, state string) bool {
	if f.Kind != "" && e.Kind != f.Kind {
		return false
	}
	for _, p := range f.Person {
		if !has(e.Directors, p) && !has(e.Cast, p) {
			return false
		}
	}
	return all(f.Genre, e.Genres) && all(f.Director, e.Directors) && all(f.Actor, e.Cast) && all(f.Country, e.Countries) &&
		one(f.Year, e.Year) && one(f.Decade, e.Decade) && one(f.Runtime, e.RuntimeBand) && all(f.Quality, e.Quality) &&
		all(f.Audio, e.Audio) && all(f.Subs, e.Subs) && (f.State == "" || f.State == state) && (f.Rating <= 0 || e.Rating >= f.Rating)
}

// states: unwatched | partial | watched por título. Una película está vista si alguna copia
// lo está; una serie, si lo están todos sus capítulos. A medias: algún progreso sin terminar.
func (x *Index) states(entries []*Entry) (map[string]string, error) {
	rows, err := x.db.Query(`SELECT p.file_id,p.watched,p.position FROM progress p JOIN files f ON f.id=p.file_id WHERE f.missing_since IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type pr struct {
		watched bool
		pos     float64
	}
	prog := map[string]pr{}
	for rows.Next() {
		var id string
		var w int
		var p float64
		if err := rows.Scan(&id, &w, &p); err != nil {
			return nil, err
		}
		prog[id] = pr{w != 0, p}
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		watched, any := 0, false
		for _, id := range e.files {
			p, ok := prog[id]
			if p.watched {
				watched++
			}
			if ok && (p.watched || p.pos >= 60) {
				any = true
			}
		}
		switch {
		case watched > 0 && (e.Kind != "series" || watched == len(e.files)):
			out[e.ID] = "watched"
		case any:
			out[e.ID] = "partial"
		default:
			out[e.ID] = "unwatched"
		}
	}
	return out, rows.Err()
}

// Match: IDs que cumplen el filtro, en orden alfabético (norm).
func (x *Index) Match(f Filter) ([]string, error) {
	ms, _, err := x.matching(f)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(ms))
	for i, e := range ms {
		ids[i] = e.ID
	}
	return ids, nil
}

func (x *Index) matching(f Filter) ([]*Entry, map[string]string, error) {
	entries, _, err := x.snapshot()
	if err != nil {
		return nil, nil, err
	}
	st, err := x.states(entries)
	if err != nil {
		return nil, nil, err
	}
	out := []*Entry{}
	for _, e := range entries {
		if f.match(e, st[e.ID]) {
			out = append(out, e)
		}
	}
	return out, st, nil
}

// Shuffle ordena los IDs de forma aleatoria pero estable para una semilla: la misma
// semilla da siempre el mismo orden, así se puede paginar y volver atrás sin repetir.
func Shuffle(ids []string, seed int64) []string {
	key := make(map[string]uint64, len(ids))
	s := strconv.FormatInt(seed, 10) + ":"
	for _, id := range ids {
		h := fnv.New64a()
		h.Write([]byte(s + id))
		key[id] = h.Sum64()
	}
	out := append([]string(nil), ids...)
	sort.Slice(out, func(i, j int) bool {
		if key[out[i]] != key[out[j]] {
			return key[out[i]] < key[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// Value: un valor de una categoría y cuántos títulos lo tienen.
type Value struct {
	Value string `json:"value"`
	Name  string `json:"name,omitempty"` // texto para mostrar si no es el propio valor
	Count int    `json:"count"`
}

// Names: las categorías que devuelve /facets, en el orden de «Filtrar por».
var Names = []string{"genre", "director", "actor", "country", "decade", "year", "runtime", "rating", "quality", "audio", "subs", "state"}

// Facets: para cada categoría pedida, sus valores con recuento entre los títulos que cumplen
// el filtro. Los actores con un solo título se omiten salvo allActors.
func (x *Index) Facets(f Filter, only []string, allActors bool) (int, map[string][]Value, error) {
	ms, st, err := x.matching(f)
	if err != nil {
		return 0, nil, err
	}
	want := map[string]bool{}
	for _, n := range only {
		want[n] = true
	}
	out := map[string][]Value{}
	for _, name := range Names {
		if len(only) > 0 && !want[name] {
			continue
		}
		count := map[string]int{}
		for _, e := range ms {
			for _, v := range values(e, name, st[e.ID]) {
				count[v]++
			}
		}
		list := []Value{}
		for v, n := range count {
			if name == "actor" && n < 2 && !allActors {
				continue
			}
			list = append(list, Value{Value: v, Name: Label(name, v), Count: n})
		}
		sortValues(name, list)
		out[name] = list
	}
	return len(ms), out, nil
}

func values(e *Entry, name, state string) []string {
	switch name {
	case "genre":
		return e.Genres
	case "director":
		return e.Directors
	case "actor":
		return e.Cast
	case "country":
		return e.Countries
	case "decade":
		return nonEmpty(e.Decade)
	case "year":
		return nonEmpty(e.Year)
	case "runtime":
		return nonEmpty(e.RuntimeBand)
	case "rating":
		var out []string
		for _, r := range ratingSteps {
			if e.Rating >= float64(r) {
				out = append(out, strconv.Itoa(r))
			}
		}
		return out
	case "quality":
		return e.Quality
	case "audio":
		return e.Audio
	case "subs":
		return e.Subs
	case "state":
		return nonEmpty(state)
	}
	return nil
}

var ratingSteps = []int{6, 7, 8}

func nonEmpty(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}

var fixedOrder = map[string][]string{
	"runtime": {"short", "mid", "long"},
	"rating":  {"6", "7", "8"},
	"quality": {"4k", "hdr", "1080", "720", "sd"},
	"state":   {"unwatched", "partial", "watched"},
}

func sortValues(name string, list []Value) {
	if ord, ok := fixedOrder[name]; ok {
		pos := map[string]int{}
		for i, v := range ord {
			pos[v] = i
		}
		sort.Slice(list, func(i, j int) bool { return pos[list[i].Value] < pos[list[j].Value] })
		return
	}
	if name == "year" || name == "decade" {
		sort.Slice(list, func(i, j int) bool { return list[i].Value > list[j].Value })
		return
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return fold(list[i].Value) < fold(list[j].Value)
	})
}

var folder = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")

func fold(s string) string { return folder.Replace(strings.ToLower(s)) }

// Label: texto para mostrar de un valor que es un código.
func Label(name, v string) string {
	switch name {
	case "decade":
		if len(v) == 4 {
			if v < "2000" {
				return "Años " + v[2:]
			}
			return "Años " + v
		}
	case "runtime":
		return map[string]string{"short": "Menos de 90 min", "mid": "De 90 a 120 min", "long": "Más de 120 min"}[v]
	case "rating":
		return v + " o más"
	case "quality":
		return map[string]string{"4k": "4K", "hdr": "HDR", "1080": "1080p", "720": "720p", "sd": "SD"}[v]
	case "audio", "subs":
		return LangName(v)
	case "state":
		return map[string]string{"unwatched": "Sin ver", "partial": "A medias", "watched": "Vistas"}[v]
	}
	return ""
}

// Counts: recuentos por categoría dentro de un tipo (movie | series), para la ficha.
type Counts struct {
	Director, Actor, Country, Genre, Year, Decade, Runtime map[string]int
}

func (x *Index) Counts(kind string) (*Counts, error) {
	entries, _, err := x.snapshot()
	if err != nil {
		return nil, err
	}
	c := &Counts{map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}}
	for _, e := range entries {
		if kind != "" && e.Kind != kind {
			continue
		}
		for _, v := range e.Directors {
			c.Director[v]++
		}
		for _, v := range e.Cast {
			c.Actor[v]++
		}
		for _, v := range e.Countries {
			c.Country[v]++
		}
		for _, v := range e.Genres {
			c.Genre[v]++
		}
		c.Year[e.Year]++
		c.Decade[e.Decade]++
		c.Runtime[e.RuntimeBand]++
	}
	return c, nil
}

// Entry de un título (nil si no tiene archivos disponibles).
func (x *Index) Entry(id string) (*Entry, error) {
	_, byID, err := x.snapshot()
	if err != nil {
		return nil, err
	}
	return byID[id], nil
}
