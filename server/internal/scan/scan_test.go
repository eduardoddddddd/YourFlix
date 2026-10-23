package scan

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

// Sin ffprobe real: los «vídeos» son bytes aleatorios, así que probe falla y se prueba
// la parte que no depende de él (huella, rutas, emparejado por bytes y por nombre).
var opt = Options{FFprobe: "/nonexistent/ffprobe"}

func write(t *testing.T, path string, size int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	b := make([]byte, size)
	rand.Read(b)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (*store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, dir
}

func scanAll(t *testing.T, db *store.DB) []*Report {
	t.Helper()
	srcs, _ := db.Sources()
	var out []*Report
	for i := range srcs {
		r, err := Source(context.Background(), db, &srcs[i], opt)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestSeedBytesMovedMissingOffline(t *testing.T) {
	db, dir := setup(t)
	lib := filepath.Join(dir, "T7")
	write(t, filepath.Join(lib, "pelis", "algo raro.mkv"), 3<<20+17) // nombre inútil: solo los bytes lo identifican
	write(t, filepath.Join(lib, "Platoon (1986).mkv"), 2<<20+5)
	seeded := &store.Title{ID: "t7-1", Kind: "movie", Title: "1917", Year: "2019", MatchSource: "seed", Norm: "1917"}
	if err := db.UpsertTitle(seeded, false); err != nil {
		t.Fatal(err)
	}
	db.ReplaceSeedCopies("t7-1", []store.SeedCopy{{TitleID: "t7-1", Bytes: 3<<20 + 17, Label: "Copia 1"}})
	db.UpsertTitle(&store.Title{ID: "t7-2", Kind: "movie", Title: "Platoon", Year: "1986", MatchSource: "seed", Norm: "platoon"}, false)
	src, _ := db.AddSource("Samsung T7", lib, "auto", nil)

	r := scanAll(t, db)[0]
	if r.New != 2 || r.MatchedSeedBytes != 1 || r.MatchedSeedTitle != 1 || r.FromFilename != 0 {
		t.Fatalf("primer escaneo: %+v", r)
	}
	files, _ := db.FilesOfSource(src.ID)
	ids := map[string]string{}
	for _, f := range files {
		ids[f.TitleID] = f.ID
	}
	if ids["t7-1"] == "" || ids["t7-2"] == "" {
		t.Fatalf("emparejado: %+v", ids)
	}

	// Renombrar y mover: mismo ID (conserva el progreso).
	os.MkdirAll(filepath.Join(lib, "nuevas"), 0o755)
	if err := os.Rename(filepath.Join(lib, "pelis", "algo raro.mkv"), filepath.Join(lib, "nuevas", "1917.mkv")); err != nil {
		t.Fatal(err)
	}
	r = scanAll(t, db)[0]
	if r.Moved != 1 || r.Missing != 0 {
		t.Fatalf("movido: %+v", r)
	}
	f, _ := db.File(ids["t7-1"])
	if f.RelPath != "nuevas/1917.mkv" || f.TitleID != "t7-1" {
		t.Fatalf("tras mover: %+v", f)
	}

	// Borrado: ausente, pero no se pierde la ficha de la semilla.
	os.Remove(filepath.Join(lib, "Platoon (1986).mkv"))
	if r = scanAll(t, db)[0]; r.Missing != 1 {
		t.Fatalf("ausente: %+v", r)
	}
	if _, err := db.Title("t7-2"); err != nil {
		t.Fatal("la ficha de la semilla no debe borrarse")
	}

	// Disco desconectado: fuente offline y todo ausente; al volver, todo reaparece.
	os.Rename(lib, lib+".off")
	if r = scanAll(t, db)[0]; !r.Offline || r.Missing != 1 {
		t.Fatalf("offline: %+v", r)
	}
	if s, _ := db.Source(src.ID); s.State != "offline" {
		t.Fatalf("estado: %s", s.State)
	}
	os.Rename(lib+".off", lib)
	scanAll(t, db)
	if f, _ = db.File(ids["t7-1"]); f.MissingSince != "" {
		t.Fatal("al reconectar debe volver a estar disponible")
	}
	if s, _ := db.Source(src.ID); s.State != "active" {
		t.Fatalf("estado tras reconectar: %s", s.State)
	}
}

func TestSameFileInTwoSources(t *testing.T) {
	db, dir := setup(t)
	a, b := filepath.Join(dir, "T7"), filepath.Join(dir, "Patriot")
	write(t, filepath.Join(a, "Platoon 1986.mkv"), 1<<20+3)
	data, _ := os.ReadFile(filepath.Join(a, "Platoon 1986.mkv"))
	os.MkdirAll(b, 0o755)
	os.WriteFile(filepath.Join(b, "copia de seguridad.mkv"), data, 0o644)
	db.UpsertTitle(&store.Title{ID: "t7-2", Kind: "movie", Title: "Platoon", Year: "1986", MatchSource: "seed", Norm: "platoon"}, false)
	db.AddSource("Samsung T7", a, "auto", nil)
	db.AddSource("USB Patriot", b, "auto", nil)
	scanAll(t, db) // el T7 identifica por nombre; el Patriot, por huella del T7
	files, _ := db.FilesOfTitle("t7-2")
	if len(files) != 2 {
		t.Fatalf("esperaba 2 copias de Platoon, hay %d", len(files))
	}
	items, _, _ := db.ListTitles(store.TitleQuery{})
	if len(items) != 1 || len(items[0].Sources) != 2 {
		t.Fatalf("catálogo: %+v", items)
	}
}

func TestSeriesAndExcludes(t *testing.T) {
	db, dir := setup(t)
	lib := filepath.Join(dir, "series")
	write(t, filepath.Join(lib, "Andor", "Andor.S01E01.mkv"), 1000)
	write(t, filepath.Join(lib, "Andor", "Andor.S01E02.mkv"), 1001)
	write(t, filepath.Join(lib, "Andor", "sample-andor.mkv"), 10)
	write(t, filepath.Join(lib, "Andor", "notas.txt"), 10)
	db.AddSource("Series", lib, "auto", []string{"*sample*"})
	r := scanAll(t, db)[0]
	if r.Seen != 2 {
		t.Fatalf("debería ignorar sample y txt: %+v", r)
	}
	items, _, _ := db.ListTitles(store.TitleQuery{Kind: "series"})
	if len(items) != 1 || items[0].Title != "Andor" || items[0].Files != 2 {
		t.Fatalf("serie: %+v", items)
	}
}

func TestFingerprint(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "v.mkv")
	write(t, p, 3<<20)
	a, _ := Fingerprint(p, 3<<20)
	b, _ := Fingerprint(p, 3<<20)
	if a == "" || a != b {
		t.Fatal("huella inestable")
	}
	small := filepath.Join(dir, "s.mkv")
	write(t, small, 100)
	if c, err := Fingerprint(small, 100); err != nil || c == a {
		t.Fatalf("archivo pequeño: %v %s", err, c)
	}
}
