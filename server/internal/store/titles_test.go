package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// «Recién añadidas» (sort=added) ordena por la llegada del archivo disponible más reciente,
// no por la creación de la ficha: una ficha de la semilla creada días antes sin archivo
// sube arriba cuando por fin llega su archivo (caso real: «No hay salida», 27-10-2026).
func TestListTitlesAddedUsesLatestFile(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	src, err := db.AddSource("Pruebas", dir, "auto", nil)
	if err != nil {
		t.Fatal(err)
	}
	title := func(id, kind, name, at string) {
		t.Helper()
		if err := db.UpsertTitle(&Title{ID: id, Kind: kind, Title: name, MatchSource: "seed", Norm: strings.ToLower(name), AddedAt: at}, false); err != nil {
			t.Fatal(err)
		}
	}
	file := func(id, titleID, at, missing string) {
		t.Helper()
		f := &File{ID: id, SourceID: src.ID, RelPath: id + ".mkv", Size: 1, Fingerprint: id, TitleID: titleID, MatchSource: "seed", AddedAt: at, MissingSince: missing}
		if err := db.SaveFile(f); err != nil {
			t.Fatal(err)
		}
	}
	title("vieja", "movie", "Vieja", "2026-10-20T10:00:00Z")
	file("f-vieja", "vieja", "2026-10-20T10:00:00Z", "")
	file("f-vieja-ausente", "vieja", "2026-10-30T10:00:00Z", "2026-10-30T11:00:00Z") // ausente: no cuenta
	title("reciente", "movie", "Reciente", "2026-10-23T10:00:00Z")
	file("f-reciente", "reciente", "2026-10-23T10:00:00Z", "")
	title("serie", "series", "Serie", "2026-10-10T10:00:00Z")
	file("f-s1e1", "serie", "2026-10-10T10:00:00Z", "")
	file("f-s1e2", "serie", "2026-10-25T10:00:00Z", "") // capítulo nuevo: la serie sube
	title("semilla", "movie", "Semilla", "2026-10-01T10:00:00Z")

	order := func() ([]string, []TitleSummary) {
		t.Helper()
		items, _, err := db.ListTitles(TitleQuery{Sort: "added"})
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		return ids, items
	}
	if ids, _ := order(); strings.Join(ids, ",") != "serie,reciente,vieja" {
		t.Fatalf("sin archivo, la ficha de la semilla no sale y el resto va por su archivo más reciente: %v", ids)
	}

	file("f-semilla", "semilla", "2026-10-27T10:00:00Z", "") // llega su archivo
	ids, items := order()
	if strings.Join(ids, ",") != "semilla,serie,reciente,vieja" {
		t.Fatalf("el archivo recién llegado debe ir primero: %v", ids)
	}
	if items[0].AddedAt != "2026-10-27T10:00:00Z" || items[1].AddedAt != "2026-10-25T10:00:00Z" || items[3].AddedAt != "2026-10-20T10:00:00Z" {
		t.Fatalf("addedAt debe ser la llegada del archivo disponible más reciente: %+v", items)
	}
}
