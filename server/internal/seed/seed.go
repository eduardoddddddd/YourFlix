// Package seed importa la carga inicial desde un clon de cine-t7-catalogo:
// fichas ya verificadas (TMDB), portadas y las copias medidas (bytes + duración)
// con las que se emparejan los archivos del T7 sin consultar a TMDB.
package seed

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eduardoddddddd/YourFlix/server/internal/parse"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

type catalog struct {
	SchemaVersion int    `json:"schemaVersion"`
	Updated       string `json:"updated"`
	Items         []item `json:"items"`
}

type item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Year  string `json:"year"`
	Files []struct {
		Name      string `json:"name"`
		Bytes     int64  `json:"bytes"`
		Season    *int   `json:"season"`
		Episode   *int   `json:"episode"`
		Technical struct {
			Seconds float64 `json:"seconds"`
		} `json:"technical"`
	} `json:"files"`
	Inventory json.RawMessage `json:"inventory"`
	Meta      json.RawMessage `json:"meta"`
}

type meta struct {
	Matched  bool   `json:"matched"`
	TMDB     int64  `json:"tmdb"`
	IMDB     string `json:"imdb"`
	Title    string `json:"title"`
	Original string `json:"original"`
	Year     string `json:"year"`
	Poster   string `json:"poster"`
	Backdrop string `json:"backdrop"`
}

type Report struct {
	Titles  int    `json:"titles"`
	Copies  int    `json:"copies"`
	Images  int    `json:"images"`
	Skipped int    `json:"skippedLocked"`
	Updated string `json:"catalogUpdated"`
}

// Import lee <repo>/data/catalog.json y copia <repo>/web/assets/* a <dataDir>/images/seed.
// Se puede repetir: los IDs son estables («t7-<id>») y no pisa fichas bloqueadas.
func Import(db *store.DB, repo, dataDir string) (*Report, error) {
	raw, err := os.ReadFile(filepath.Join(repo, "data", "catalog.json"))
	if err != nil {
		return nil, fmt.Errorf("no encuentro data/catalog.json en %s: %w", repo, err)
	}
	var c catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("catalog.json: %w", err)
	}
	if c.SchemaVersion != 1 {
		return nil, fmt.Errorf("catalog.json schemaVersion %d no admitido (esperado 1)", c.SchemaVersion)
	}
	imgDir := filepath.Join(dataDir, "images", "seed")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return nil, err
	}
	rep := &Report{Updated: c.Updated}
	for _, it := range c.Items {
		var m meta
		json.Unmarshal(it.Meta, &m)
		t := &store.Title{
			ID: "t7-" + it.ID, Kind: kindOf(it.Type), Title: first(m.Title, it.Title), Original: m.Original, Year: first(m.Year, it.Year),
			TMDB: m.TMDB, IMDB: m.IMDB, Meta: merge(it.Meta, it.Inventory), MatchSource: "seed",
		}
		t.Norm, t.NormOrig = parse.Normalize(t.Title), parse.Normalize(t.Original)
		if !m.Matched {
			t.MatchSource = "filename" // ficha pendiente en la semilla: que TMDB la complete
		}
		for _, img := range []struct {
			src string
			dst *string
		}{{m.Poster, &t.Poster}, {m.Backdrop, &t.Backdrop}} {
			v, copied, err := image(img.src, repo, imgDir)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", it.Title, err)
			}
			*img.dst = v
			if copied {
				rep.Images++
			}
		}
		if existing, err := db.Title(t.ID); err == nil && existing.Locked {
			rep.Skipped++
			continue
		}
		if err := db.UpsertTitle(t, false); err != nil {
			return nil, fmt.Errorf("%s: %w", it.Title, err)
		}
		var copies []store.SeedCopy
		for _, f := range it.Files {
			if f.Bytes > 0 {
				copies = append(copies, store.SeedCopy{TitleID: t.ID, Bytes: f.Bytes, Seconds: f.Technical.Seconds, Season: f.Season, Episode: f.Episode, Label: f.Name})
			}
		}
		if err := db.ReplaceSeedCopies(t.ID, copies); err != nil {
			return nil, err
		}
		rep.Titles++
		rep.Copies += len(copies)
	}
	return rep, nil
}

func kindOf(t string) string {
	if t == "series" {
		return "series"
	}
	return "movie"
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// merge guarda la ficha de la semilla tal cual y añade su inventario (ubicación, notas).
func merge(metaRaw, inv json.RawMessage) json.RawMessage {
	m := map[string]any{}
	json.Unmarshal(metaRaw, &m)
	delete(m, "poster")
	delete(m, "backdrop")
	if len(inv) > 0 {
		var v any
		if json.Unmarshal(inv, &v) == nil {
			m["inventory"] = v
		}
	}
	out, _ := json.Marshal(m)
	return out
}

// image devuelve la referencia que guarda el catálogo: URL https tal cual, o la ruta
// absoluta de la copia local de un «assets/…» del repo.
func image(src, repo, imgDir string) (string, bool, error) {
	switch {
	case src == "":
		return "", false, nil
	case strings.HasPrefix(src, "https://"):
		return src, false, nil
	case strings.HasPrefix(src, "assets/") && !strings.Contains(src, ".."):
		from := filepath.Join(repo, "web", filepath.FromSlash(src))
		to := filepath.Join(imgDir, filepath.Base(src))
		if _, err := os.Stat(to); err == nil {
			return to, false, nil
		}
		in, err := os.Open(from)
		if err != nil {
			return "", false, nil // imagen ausente en el repo: la ficha queda sin ella
		}
		defer in.Close()
		out, err := os.Create(to)
		if err != nil {
			return "", false, err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return "", false, err
		}
		return to, true, out.Close()
	}
	return "", false, nil
}
