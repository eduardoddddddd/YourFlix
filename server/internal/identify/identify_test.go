package identify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/parse"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
	"github.com/eduardoddddddd/YourFlix/server/internal/tmdb"
)

type res = map[string]any

// fakeTMDB responde como la API v3 con un puñado de títulos.
func fakeTMDB(t *testing.T, calls *int) *httptest.Server {
	search := map[string][]res{
		"movie/blade runner": {{"id": 78, "title": "Blade Runner", "original_title": "Blade Runner", "release_date": "1982-06-25", "vote_count": 14000},
			{"id": 335984, "title": "Blade Runner 2049", "original_title": "Blade Runner 2049", "release_date": "2017-10-04", "vote_count": 13000}},
		"movie/alien": {{"id": 348, "title": "Alien, el octavo pasajero", "original_title": "Alien", "release_date": "1979-05-25", "vote_count": 15000},
			{"id": 999, "title": "Alien", "original_title": "Alien", "release_date": "2023-01-01", "vote_count": 2}},
		"movie/dracula": {{"id": 6114, "title": "Drácula", "original_title": "Dracula", "release_date": "1992-11-13", "vote_count": 5000},
			{"id": 138, "title": "Drácula", "original_title": "Dracula", "release_date": "1931-02-12", "vote_count": 1500}},
		"movie/s1":         {{"id": 5, "title": "S1m0ne", "original_title": "S1m0ne", "release_date": "2002-08-23", "vote_count": 1000}},
		"movie/no way out": {{"id": 10083, "title": "No hay salida", "original_title": "No Way Out", "release_date": "1987-08-14", "vote_count": 859}},
		"tv/the bear":      {{"id": 136315, "name": "The Bear", "original_name": "The Bear", "first_air_date": "2022-06-23", "vote_count": 1993}},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		if r.Header.Get("Authorization") != "Bearer secreto" {
			w.WriteHeader(401)
			return
		}
		p := r.URL.Path
		switch {
		case strings.HasPrefix(p, "/search/"):
			kind := strings.TrimPrefix(p, "/search/")
			json.NewEncoder(w).Encode(res{"results": search[kind+"/"+parse.Normalize(r.URL.Query().Get("query"))]})
		case p == "/movie/78":
			json.NewEncoder(w).Encode(res{"id": 78, "imdb_id": "tt0083658", "title": "Blade Runner", "original_title": "Blade Runner", "release_date": "1982-06-25",
				"overview": "Los Ángeles, 2019.", "runtime": 117, "vote_average": 7.9, "poster_path": "/p.jpg", "backdrop_path": "/b.jpg",
				"genres": []res{{"name": "Ciencia ficción"}}, "credits": res{"crew": []res{{"name": "Ridley Scott", "job": "Director"}}, "cast": []res{{"name": "Harrison Ford"}}}})
		case p == "/movie/348":
			json.NewEncoder(w).Encode(res{"id": 348, "title": "Alien, el octavo pasajero", "original_title": "Alien", "release_date": "1979-05-25", "overview": ""})
		case p == "/tv/136315":
			json.NewEncoder(w).Encode(res{"id": 136315, "name": "The Bear", "original_name": "The Bear", "first_air_date": "2022-06-23", "overview": "Un chef…",
				"created_by": []res{{"name": "Christopher Storer"}}, "external_ids": res{"imdb_id": "tt14452776"}, "number_of_seasons": 3})
		case p == "/tv/136315/season/1":
			json.NewEncoder(w).Encode(res{"episodes": []res{{"season_number": 1, "episode_number": 1, "name": "Sistema", "still_path": "/s.jpg"}}})
		default:
			w.WriteHeader(404)
		}
	}))
}

func setup(t *testing.T) (*store.DB, *tmdb.Client, *int) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	calls := new(int)
	srv := fakeTMDB(t, calls)
	t.Cleanup(srv.Close)
	return db, &tmdb.Client{Token: "secreto", Base: srv.URL, Cache: store.TMDBCache{DB: db}}, calls
}

var src *store.Source

// provisional crea una ficha «filename» con un archivo, como haría el escaneo.
func provisional(t *testing.T, db *store.DB, id, kind, title, year string, season, episode *int) {
	t.Helper()
	if src == nil || src.ID == "" {
		src, _ = db.AddSource("T7", t.TempDir(), "auto", nil)
	}
	db.UpsertTitle(&store.Title{ID: id, Kind: kind, Title: title, Year: year, MatchSource: "filename", Norm: parse.Normalize(title)}, false)
	db.SaveFile(&store.File{ID: "f-" + id, SourceID: src.ID, RelPath: id + ".mkv", Fingerprint: id, TitleID: id, MatchSource: "filename", Season: season, Episode: episode})
}

func TestPending(t *testing.T) {
	src = nil
	db, c, calls := setup(t)
	one := 1
	provisional(t, db, "br", "movie", "Blade Runner", "1982", nil, nil)
	provisional(t, db, "al", "movie", "Alien", "", nil, nil)
	provisional(t, db, "dr", "movie", "Dracula", "", nil, nil)
	provisional(t, db, "s1", "movie", "s1", "", nil, nil)
	provisional(t, db, "bear", "series", "The Bear", "", &one, &one)
	// La semilla ya tiene «No hay salida»: un archivo nuevo de esa película se fusiona.
	db.UpsertTitle(&store.Title{ID: "t7-nhs", Kind: "movie", Title: "No Hay Salida", Year: "1987", TMDB: 10083, MatchSource: "seed", Norm: "no hay salida"}, false)
	provisional(t, db, "nwo", "movie", "No Way Out", "1987", nil, nil)

	rep, err := Pending(context.Background(), db, c, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Identified != 3 || rep.Merged != 1 || rep.Pending != 2 || len(rep.Errors) != 0 {
		t.Fatalf("informe: %+v", rep)
	}
	br, _ := db.Title("br")
	var m map[string]any
	json.Unmarshal(br.Meta, &m)
	if br.MatchSource != "tmdb" || br.TMDB != 78 || br.Poster != "https://image.tmdb.org/t/p/w500/p.jpg" || m["synopsis"] != "Los Ángeles, 2019." || m["director"].([]any)[0] != "Ridley Scott" {
		t.Fatalf("Blade Runner: %+v %s", br, br.Meta)
	}
	if al, _ := db.Title("al"); al.TMDB != 348 || al.Year != "1979" {
		t.Fatalf("Alien sin año debe ser la de 1979: %+v", al)
	}
	if dr, _ := db.Title("dr"); dr.MatchSource != "filename" || !strings.Contains(string(dr.Meta), `"candidates"`) {
		t.Fatalf("Drácula sin año es ambiguo: %+v %s", dr, dr.Meta)
	}
	if s1, _ := db.Title("s1"); s1.MatchSource != "filename" {
		t.Fatalf("«s1» no es «S1m0ne»: %+v", s1)
	}
	if _, err := db.Title("nwo"); err == nil {
		t.Fatal("la ficha provisional fusionada debe desaparecer")
	}
	if files, _ := db.FilesOfTitle("t7-nhs"); len(files) != 1 {
		t.Fatalf("el archivo debe pasar a la ficha de la semilla: %d", len(files))
	}
	bear, _ := db.Title("bear")
	if bear.TMDB != 136315 || !strings.Contains(string(bear.Meta), `"1x1":{`) || !strings.Contains(string(bear.Meta), "Sistema") {
		t.Fatalf("serie con capítulos: %s", bear.Meta)
	}

	// Segunda pasada: no repite las pendientes recientes ni consulta TMDB.
	before := *calls
	rep, _ = Pending(context.Background(), db, c, Options{})
	if rep.Tried != 0 || rep.Skipped != 2 || *calls != before {
		t.Fatalf("segunda pasada: %+v, %d llamadas", rep, *calls-before)
	}
	// Con --retry vuelve a probar, pero desde la caché.
	rep, _ = Pending(context.Background(), db, c, Options{Retry: true})
	if rep.Tried != 2 || *calls != before {
		t.Fatalf("reintento con caché: %+v, %d llamadas", rep, *calls-before)
	}
}

func TestManualMatchLocks(t *testing.T) {
	src = nil
	db, c, _ := setup(t)
	provisional(t, db, "x", "movie", "Una copia mal nombrada", "", nil, nil)
	if _, err := Apply(context.Background(), db, c, mustTitle(t, db, "x"), 78, 1, true); err != nil {
		t.Fatal(err)
	}
	x := mustTitle(t, db, "x")
	if !x.Locked || x.MatchSource != "manual" || x.Title != "Blade Runner" {
		t.Fatalf("manual: %+v", x)
	}
	if got, _ := db.PendingTitles(); len(got) != 0 {
		t.Fatal("una ficha manual no está pendiente")
	}
}

func TestNoCredentialsInCache(t *testing.T) {
	src = nil
	db, c, _ := setup(t)
	c.Search(context.Background(), "movie", "Blade Runner", "1982", "es-ES")
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM tmdb_cache WHERE url LIKE '%secreto%' OR body LIKE '%secreto%'`).Scan(&n)
	if n != 0 {
		t.Fatal("las credenciales no deben llegar a la caché")
	}
}

func mustTitle(t *testing.T, db *store.DB, id string) *store.Title {
	t.Helper()
	x, err := db.Title(id)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestSimilarity(t *testing.T) {
	if similarity("blade runner", "blade runner") != 1 || similarity("s1", "s1m0ne") > 0.6 || similarity("el laberinto del fauno", "el laberinto del fauno") != 1 {
		t.Fatal("similitud")
	}
}
