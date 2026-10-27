package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
	"github.com/eduardoddddddd/YourFlix/server/internal/images"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

type fixture struct {
	id, kind, title, year, meta, video, audio, subs string
}

// catálogo pequeño con directores, actores y países repetidos, para las categorías.
var catalog = []fixture{
	{"m1", "movie", "Alien", "1979", `{"director":["Ridley Scott"],"cast":["Sigourney Weaver","Tom Skerritt"],"country":"United States of America, United Kingdom","genres":["Terror","Ciencia ficción"],"runtime":117,"rating":8.1}`,
		`{"codec":"hevc","width":3840,"height":1600,"hdr":"HDR10"}`, `[{"codec":"ac3","channels":6,"language":"spa"},{"codec":"dts","channels":6,"language":"eng"}]`, `[{"codec":"subrip","language":"spa","text":true}]`},
	{"m2", "movie", "Blade Runner", "1982", `{"director":["Ridley Scott"],"cast":["Harrison Ford","Rutger Hauer"],"country":"United States of America","genres":["Ciencia ficción"],"runtime":117,"rating":7.9}`,
		`{"codec":"h264","width":1920,"height":800}`, `[{"codec":"ac3","channels":6,"language":"eng"}]`, `[{"codec":"hdmv_pgs_subtitle","language":"spa","text":false}]`},
	{"m3", "movie", "Gladiator", "2000", `{"director":["Ridley Scott"],"cast":["Russell Crowe","Joaquin Phoenix"],"country":"United Kingdom, United States of America","genres":["Acción","Drama"],"runtime":155,"rating":8.2}`,
		`{"codec":"h264","width":1920,"height":1040}`, `[{"codec":"aac","channels":2,"language":"es"}]`, `[]`},
	{"m4", "movie", "Indiana Jones en busca del arca perdida", "1981", `{"director":["Steven Spielberg"],"cast":["Harrison Ford","Karen Allen"],"country":"United States of America","genres":["Aventura","Acción"],"runtime":115,"rating":7.9}`,
		`{"codec":"h264","width":1280,"height":720}`, `[{"codec":"ac3","channels":2,"language":"spa"}]`, `[]`},
	{"m5", "movie", "Amanece que no es poco", "1989", `{"director":["José Luis Cuerda"],"cast":["Antonio Resines"],"country":"Spain","genres":["Comedia"],"runtime":89,"rating":7.3}`,
		`{"codec":"mpeg4","width":640,"height":352}`, `[{"codec":"mp3","channels":2,"language":"spa"}]`, `[]`},
	{"m6", "movie", "La haine", "1995", `{"director":["Mathieu Kassovitz"],"cast":["Vincent Cassel"],"country":"France","genres":["Drama"],"rating":7.8}`,
		`{"codec":"mpeg2video","width":720,"height":576}`, `[{"codec":"mp2","channels":2,"language":"fre"}]`, `[{"codec":"subrip","language":"spa","text":true}]`},
	{"s1", "series", "The Bear", "2022", `{"director":["Christopher Storer"],"cast":["Jeremy Allen White"],"country":"United States of America","genres":["Drama","Comedia"],"runtime":30,"rating":8.2}`,
		`{"codec":"h264","width":1920,"height":1080}`, `[{"codec":"eac3","channels":6,"language":"eng"}]`, `[]`},
}

func setupCatalog(t *testing.T) (http.Handler, *store.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	src, _ := db.AddSource("Ally", dir, "auto", nil)
	for i, c := range catalog {
		if err := db.UpsertTitle(&store.Title{ID: c.id, Kind: c.kind, Title: c.title, Year: c.year, Meta: json.RawMessage(c.meta), MatchSource: "seed", Norm: c.title}, false); err != nil {
			t.Fatal(err)
		}
		f := &store.File{ID: "f" + c.id, SourceID: src.ID, RelPath: c.id + ".mkv", Size: int64(i + 1), Fingerprint: c.id, Container: "mkv", TitleID: c.id,
			Video: []byte(c.video), Audio: []byte(c.audio), Subs: []byte(c.subs)}
		if err := db.SaveFile(f); err != nil {
			t.Fatal(err)
		}
	}
	// Segundo capítulo de la serie, sin ver; y un archivo ausente que no debe contar.
	db.SaveFile(&store.File{ID: "fs1b", SourceID: src.ID, RelPath: "s1b.mkv", Size: 99, Fingerprint: "s1b", TitleID: "s1", Video: []byte(`{"width":1920,"height":1080}`)})
	db.UpsertTitle(&store.Title{ID: "gone", Kind: "movie", Title: "Prometheus", Year: "2012", Meta: json.RawMessage(`{"director":["Ridley Scott"]}`), MatchSource: "seed", Norm: "prometheus"}, false)
	db.SaveFile(&store.File{ID: "fgone", SourceID: src.ID, RelPath: "gone.mkv", Size: 7, Fingerprint: "gone", TitleID: "gone", MissingSince: store.Now()})
	db.SaveProgress(store.Progress{FileID: "fm1", Position: 7000, Duration: 7020, Watched: true}) // Alien vista
	db.SaveProgress(store.Progress{FileID: "fm2", Position: 600, Duration: 7020})                // Blade Runner a medias
	db.SaveProgress(store.Progress{FileID: "fs1", Position: 1700, Duration: 1750, Watched: true}) // The Bear: 1 de 2
	prof, _ := compat.Load("")
	s := &Server{DB: db, Images: images.New(filepath.Join(dir, "img")), Profile: prof, Version: "test", Log: log.New(io.Discard, "", 0)}
	return s.Handler(), db
}

type listOut struct {
	Total int `json:"total"`
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
}

func list(t *testing.T, h http.Handler, query string) (int, []string) {
	t.Helper()
	w := do(h, "GET", "/api/v1/titles?"+query, lan, nil, "")
	if w.Code != 200 {
		t.Fatalf("%s: %d %s", query, w.Code, w.Body.String())
	}
	var out listOut
	json.Unmarshal(w.Body.Bytes(), &out)
	ids := []string{}
	for _, it := range out.Items {
		ids = append(ids, it.ID)
	}
	return out.Total, ids
}

func TestTitleFilters(t *testing.T) {
	h, _ := setupCatalog(t)
	cases := []struct {
		q    string
		want string
	}{
		{"kind=movie&director=Ridley+Scott", "[m1 m2 m3]"}, // Prometheus está ausente
		{"kind=movie&director=Ridley+Scott&decade=1980", "[m2]"},
		{"kind=movie&director=Ridley+Scott&decade=80", "[m2]"},
		{"actor=Harrison+Ford", "[m2 m4]"},
		{"person=Harrison+Ford&person=Ridley+Scott", "[m2]"},
		{"country=Espa%C3%B1a", "[m5]"},
		{"country=United+Kingdom", "[m1 m3]"}, // también el nombre de TMDB
		{"year=1981", "[m4]"},
		{"kind=movie&runtime=short", "[m5]"},
		{"kind=movie&runtime=long", "[m3]"},
		{"kind=movie&rating=8", "[m1 m3]"},
		{"quality=4k", "[m1]"},
		{"quality=hdr", "[m1]"},
		{"kind=movie&quality=sd", "[m5 m6]"},
		{"audio=spa", "[m1 m5 m3 m4]"},
		{"subs=spa", "[m1 m6]"}, // la PGS de Blade Runner no cuenta
		{"kind=movie&state=watched", "[m1]"},
		{"state=partial", "[m2 s1]"}, // a The Bear le falta un capítulo
		{"kind=movie&state=unwatched", "[m5 m3 m4 m6]"},
		{"kind=movie&genre=Drama&country=Francia", "[m6]"},
		{"director=Nadie", "[]"},
		{"kind=movie&director=Ridley+Scott&sort=runtime", "[m1 m2 m3]"},
	}
	for _, c := range cases {
		total, ids := list(t, h, c.q)
		if got := "[" + join(ids) + "]"; got != c.want || total != len(ids) {
			t.Errorf("%s: %s (total %d), quería %s", c.q, got, total, c.want)
		}
	}
	// Sin filtros nuevos la lista sigue como antes (la app de la TV instalada).
	if total, _ := list(t, h, "kind=movie"); total != 6 {
		t.Fatalf("películas: %d", total)
	}
	// Orden por duración: sin duración (La haine) al final.
	if _, ids := list(t, h, "kind=movie&sort=runtime"); ids[0] != "m5" || ids[len(ids)-1] != "m6" {
		t.Fatalf("duración: %v", ids)
	}
}

func join(ids []string) string {
	s := ""
	for i, x := range ids {
		if i > 0 {
			s += " "
		}
		s += x
	}
	return s
}

func TestRandomStable(t *testing.T) {
	h, _ := setupCatalog(t)
	_, all := list(t, h, "sort=random&seed=8213&limit=50")
	_, again := list(t, h, "sort=random&seed=8213&limit=50")
	if join(all) != join(again) || len(all) != 7 {
		t.Fatalf("misma semilla: %v / %v", all, again)
	}
	var paged []string
	for off := 0; off < 7; off += 3 {
		total, ids := list(t, h, "sort=random&seed=8213&limit=3&offset="+strconv.Itoa(off))
		if total != 7 {
			t.Fatalf("total %d", total)
		}
		paged = append(paged, ids...)
	}
	if join(paged) != join(all) {
		t.Fatalf("páginas %v ≠ entera %v", paged, all)
	}
	other := false
	for seed := 1; seed < 6 && !other; seed++ {
		_, ids := list(t, h, "sort=random&limit=50&seed="+strconv.Itoa(seed))
		other = join(ids) != join(all)
	}
	if !other {
		t.Fatal("la semilla no cambia el orden")
	}
	if _, ids := list(t, h, "kind=movie&director=Ridley+Scott&sort=random&seed=3"); len(ids) != 3 {
		t.Fatalf("aleatorio con filtro: %v", ids)
	}
}

type facetsOut struct {
	Total  int `json:"total"`
	Facets map[string][]struct {
		Value string `json:"value"`
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"facets"`
}

func getFacets(t *testing.T, h http.Handler, query string) facetsOut {
	t.Helper()
	w := do(h, "GET", "/api/v1/facets?"+query, lan, nil, "")
	if w.Code != 200 {
		t.Fatalf("%s: %d", query, w.Code)
	}
	var out facetsOut
	json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

func TestFacets(t *testing.T) {
	h, _ := setupCatalog(t)
	f := getFacets(t, h, "kind=movie")
	if f.Total != 6 {
		t.Fatalf("total %d", f.Total)
	}
	if d := f.Facets["director"][0]; d.Value != "Ridley Scott" || d.Count != 3 {
		t.Fatalf("director: %+v", f.Facets["director"])
	}
	if a := f.Facets["actor"]; len(a) != 1 || a[0].Value != "Harrison Ford" || a[0].Count != 2 {
		t.Fatalf("actores con 2 o más: %+v", a)
	}
	if c := f.Facets["country"][0]; c.Value != "Estados Unidos" || c.Count != 4 {
		t.Fatalf("país: %+v", f.Facets["country"])
	}
	if s := f.Facets["state"]; len(s) != 3 || s[0].Value != "unwatched" || s[0].Name != "Sin ver" || s[0].Count != 4 {
		t.Fatalf("estado: %+v", s)
	}
	if a := f.Facets["audio"][0]; a.Value != "spa" || a.Name != "Castellano" || a.Count != 4 {
		t.Fatalf("audio: %+v", f.Facets["audio"])
	}
	// Con los filtros puestos: las décadas de Ridley Scott.
	f = getFacets(t, h, "kind=movie&director=Ridley+Scott&only=decade,actor&all=1")
	if f.Total != 3 || len(f.Facets) != 2 || len(f.Facets["actor"]) != 6 {
		t.Fatalf("filtrado: %+v", f)
	}
	if dec := f.Facets["decade"]; len(dec) != 3 || dec[0].Value != "2000" || dec[0].Name != "Años 2000" || dec[2].Name != "Años 70" || dec[2].Count != 1 {
		t.Fatalf("décadas: %+v", dec)
	}
	if q := getFacets(t, h, "only=quality").Facets["quality"]; q[0].Value != "4k" || q[1].Value != "hdr" {
		t.Fatalf("calidad: %+v", q)
	}
}

func TestTitleLinks(t *testing.T) {
	h, _ := setupCatalog(t)
	w := do(h, "GET", "/api/v1/titles/m2", lan, nil, "")
	var out struct {
		Title struct {
			Meta struct {
				Director []string `json:"director"`
				Country  string   `json:"country"`
			} `json:"meta"`
		} `json:"title"`
		Links struct {
			Director []link `json:"director"`
			Cast     []link `json:"cast"`
			Country  []link `json:"country"`
			Decade   link   `json:"decade"`
			Runtime  link   `json:"runtime"`
			Year     link   `json:"year"`
		} `json:"links"`
		Files []json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	l := out.Links
	if len(l.Director) != 1 || l.Director[0] != (link{Name: "Ridley Scott", Count: 3}) {
		t.Fatalf("director: %+v", l.Director)
	}
	if l.Cast[0] != (link{Name: "Harrison Ford", Count: 2}) || l.Cast[1].Count != 1 {
		t.Fatalf("reparto: %+v", l.Cast)
	}
	if l.Country[0] != (link{Name: "Estados Unidos", Count: 4}) {
		t.Fatalf("país: %+v", l.Country)
	}
	if l.Decade != (link{Name: "Años 80", Value: "1980", Count: 3}) || l.Runtime.Value != "mid" || l.Year.Name != "1982" {
		t.Fatalf("década/duración/año: %+v %+v %+v", l.Decade, l.Runtime, l.Year)
	}
	// Lo que ya usaba la TV no cambia.
	if out.Title.Meta.Director[0] != "Ridley Scott" || out.Title.Meta.Country != "United States of America" || len(out.Files) != 1 {
		t.Fatalf("ficha: %+v", out.Title)
	}
	// Recuentos dentro del tipo: The Bear (serie) no suma a las películas de EE. UU.
	w = do(h, "GET", "/api/v1/titles/s1", lan, nil, "")
	var s struct {
		Links struct {
			Country []link `json:"country"`
		} `json:"links"`
	}
	json.Unmarshal(w.Body.Bytes(), &s)
	if s.Links.Country[0].Count != 1 {
		t.Fatalf("serie: %+v", s.Links)
	}
}

func TestFacetsFollowDB(t *testing.T) {
	h, db := setupCatalog(t)
	if n, _ := list(t, h, "director=Ridley+Scott"); n != 3 {
		t.Fatalf("antes: %d", n)
	}
	// Una ficha editada (o recién identificada) se ve sin reiniciar.
	db.UpsertTitle(&store.Title{ID: "m4", Kind: "movie", Title: "Indiana Jones", Year: "1981", Meta: json.RawMessage(`{"director":["Ridley Scott"]}`), MatchSource: "manual", Norm: "indiana"}, true)
	if n, _ := list(t, h, "director=Ridley+Scott"); n != 4 {
		t.Fatalf("después de editar: %d", n)
	}
	// El progreso nuevo cambia el estado al momento.
	db.SaveProgress(store.Progress{FileID: "fm6", Position: 6000, Duration: 6000, Watched: true})
	if _, ids := list(t, h, "kind=movie&state=watched"); join(ids) != "m1 m6" {
		t.Fatalf("vistas: %v", ids)
	}
}
