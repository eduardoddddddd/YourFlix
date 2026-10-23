package seed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

const catalogJSON = `{"schemaVersion":1,"updated":"2026-10-12","items":[
 {"id":"61","title":"1917","type":"movies","year":"2019",
  "files":[{"name":"Copia 1","bytes":4559195887,"season":null,"episode":null,"technical":{"seconds":7138.5}}],
  "inventory":{"source":"scan","location":"Samsung T7"},
  "meta":{"matched":true,"tmdb":530915,"title":"1917","original":"1917","year":"2019","synopsis":"…","genres":["Bélica"],
          "poster":"assets/poster-61.jpg","backdrop":"https://image.tmdb.org/t/p/w1280/x.jpg"}},
 {"id":"59","title":"Andor","type":"series","year":"2022",
  "files":[{"name":"Temporada 1, capítulo 1","bytes":2064079584,"season":1,"episode":1,"technical":{"seconds":2471.18}}],
  "meta":{"matched":true,"title":"Andor","original":"Andor"}},
 {"id":"manual-x","title":"Pendiente","type":"movies","year":null,"files":[],"meta":{"matched":false,"title":"Pendiente"}}
]}`

func TestImport(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	os.MkdirAll(filepath.Join(repo, "data"), 0o755)
	os.MkdirAll(filepath.Join(repo, "web", "assets"), 0o755)
	os.WriteFile(filepath.Join(repo, "data", "catalog.json"), []byte(catalogJSON), 0o644)
	os.WriteFile(filepath.Join(repo, "web", "assets", "poster-61.jpg"), []byte("jpg"), 0o644)
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	data := filepath.Join(dir, "data")
	rep, err := Import(db, repo, data)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Titles != 3 || rep.Copies != 2 || rep.Images != 1 {
		t.Fatalf("informe: %+v", rep)
	}
	m, _ := db.Title("t7-61")
	if m.Kind != "movie" || m.TMDB != 530915 || m.Poster != filepath.Join(data, "images", "seed", "poster-61.jpg") || m.Backdrop == "" {
		t.Fatalf("película: %+v", m)
	}
	if s, _ := db.Title("t7-59"); s.Kind != "series" {
		t.Fatalf("serie: %+v", s)
	}
	if p, _ := db.Title("t7-manual-x"); p.MatchSource != "filename" {
		t.Fatalf("una ficha sin verificar en la semilla queda pendiente: %s", p.MatchSource)
	}
	c, _ := db.SeedCopiesByBytes(2064079584)
	if len(c) != 1 || c[0].Season == nil || *c[0].Season != 1 || *c[0].Episode != 1 {
		t.Fatalf("copia de serie: %+v", c)
	}
	// Reimportar no duplica y respeta fichas bloqueadas.
	m.Locked, m.Title = true, "1917 (mi versión)"
	db.UpsertTitle(m, true)
	rep, _ = Import(db, repo, data)
	if rep.Skipped != 1 {
		t.Fatalf("bloqueada: %+v", rep)
	}
	if m, _ = db.Title("t7-61"); m.Title != "1917 (mi versión)" {
		t.Fatal("la reimportación pisó una ficha bloqueada")
	}
}
