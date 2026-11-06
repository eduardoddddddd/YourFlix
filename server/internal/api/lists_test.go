package api

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

type listsResp map[string]struct {
	Titles []struct {
		ID    string
		Lists []string
	}
	Episodes []struct {
		TitleID, FileID string
		Season, Episode int
		Position        float64
	}
}


func TestLists(t *testing.T) {
	h, db, _ := setup(t)
	src, _ := db.Sources()
	db.UpsertTitle(&store.Title{ID: "bear", Kind: "series", Title: "The Bear", MatchSource: "seed", Norm: "the bear"}, false)
	one, two := 1, 2
	for i, ep := range []*int{&one, &two} {
		db.SaveFile(&store.File{ID: "e" + strconv.Itoa(i+1), SourceID: src[0].ID, RelPath: "bear" + strconv.Itoa(i) + ".mkv", Fingerprint: "b" + strconv.Itoa(i), Container: "mkv", TitleID: "bear", Season: &one, Episode: ep, Duration: 1800})
	}
	read := func() listsResp {
		t.Helper()
		w := do(h, "GET", "/api/v1/lists", lan, nil, "")
		var r listsResp
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil || w.Code != 200 {
			t.Fatalf("GET /lists: %d %s", w.Code, w.Body.String())
		}
		return r
	}
	if r := read(); len(r["favorites"].Titles) != 0 || len(r["watchlist"].Episodes) != 0 {
		t.Fatalf("vacías al principio: %+v", r)
	}
	for _, c := range []struct {
		method, url string
		code        int
	}{
		{"PUT", "/api/v1/lists/favorites/t7-1", 204},
		{"PUT", "/api/v1/lists/favorites/t7-1", 204}, // idempotente
		{"PUT", "/api/v1/lists/watchlist/t7-1", 204},
		{"PUT", "/api/v1/lists/watchlist/bear?season=1&episode=2", 204},
		{"PUT", "/api/v1/lists/favorites/bear?season=1&episode=1", 204},
		{"PUT", "/api/v1/lists/otra/t7-1", 404},
		{"PUT", "/api/v1/lists/favorites/nada", 404},
		{"PUT", "/api/v1/lists/favorites/bear?season=1", 400},
		{"PUT", "/api/v1/lists/favorites/t7-1?season=1&episode=1", 400}, // una película no tiene capítulos
	} {
		if w := do(h, c.method, c.url, lan, nil, ""); w.Code != c.code {
			t.Fatalf("%s %s: %d (esperado %d) %s", c.method, c.url, w.Code, c.code, w.Body.String())
		}
	}
	r := read()
	if f := r["favorites"]; len(f.Titles) != 1 || f.Titles[0].ID != "t7-1" || len(f.Episodes) != 1 || f.Episodes[0].FileID != "e1" {
		t.Fatalf("favoritos: %+v", f)
	}
	if wl := r["watchlist"]; len(wl.Titles) != 1 || len(wl.Episodes) != 1 || wl.Episodes[0].Episode != 2 {
		t.Fatalf("pendientes: %+v", wl)
	}
	// La tarjeta del catálogo y la ficha saben en qué listas está cada cosa.
	var page struct{ Items []struct{ ID string; Lists []string } }
	json.Unmarshal(do(h, "GET", "/api/v1/titles?kind=movie", lan, nil, "").Body.Bytes(), &page)
	if len(page.Items) != 1 || len(page.Items[0].Lists) != 2 {
		t.Fatalf("lists en /titles: %+v", page)
	}
	var det struct {
		Lists []string
		Files []struct{ ID string; Lists []string }
	}
	json.Unmarshal(do(h, "GET", "/api/v1/titles/bear", lan, nil, "").Body.Bytes(), &det)
	if len(det.Lists) != 0 || len(det.Files) != 2 || len(det.Files[0].Lists) != 1 || det.Files[0].Lists[0] != "favorites" || det.Files[1].Lists[0] != "watchlist" {
		t.Fatalf("ficha de la serie: %+v", det)
	}
	// Terminar de ver: sale de pendientes (película y capítulo), no de favoritos.
	do(h, "PUT", "/api/v1/progress/f1", lan, nil, `{"position":5900,"duration":6000}`)
	do(h, "PUT", "/api/v1/progress/e2", lan, nil, `{"position":1790,"duration":1800}`)
	r = read()
	if wl := r["watchlist"]; len(wl.Titles) != 0 || len(wl.Episodes) != 0 {
		t.Fatalf("lo visto debe salir de pendientes: %+v", wl)
	}
	if len(r["favorites"].Titles) != 1 {
		t.Fatalf("favoritos no cambia al ver: %+v", r["favorites"])
	}
	// Quitar.
	if w := do(h, "DELETE", "/api/v1/lists/favorites/bear?season=1&episode=1", lan, nil, ""); w.Code != 204 {
		t.Fatalf("DELETE: %d", w.Code)
	}
	if len(read()["favorites"].Episodes) != 0 {
		t.Fatal("el capítulo debe salir de favoritos")
	}
	// Un disco desconectado oculta el título, pero la marca se conserva.
	f, _ := db.File("f1")
	f.MissingSince = store.Now()
	db.SaveFile(f)
	if len(read()["favorites"].Titles) != 0 {
		t.Fatal("sin archivo disponible no se lista")
	}
	f.MissingSince = ""
	db.SaveFile(f)
	if len(read()["favorites"].Titles) != 1 {
		t.Fatal("al volver el disco, vuelve a la lista")
	}
}

func TestListsSurviveMerge(t *testing.T) {
	_, db, _ := setup(t)
	db.UpsertTitle(&store.Title{ID: "prov", Kind: "movie", Title: "platoon", MatchSource: "filename", Norm: "platoon"}, false)
	db.AddToList(store.ListFavorites, "prov", nil, nil)
	if err := db.MoveFiles("prov", "t7-1", "tmdb"); err != nil {
		t.Fatal(err)
	}
	whole, _, _ := db.TitleLists("t7-1")
	if len(whole) != 1 || whole[0] != store.ListFavorites {
		t.Fatalf("la marca pasa a la ficha de destino: %v", whole)
	}
}
