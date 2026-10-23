package scan

import (
	"math"

	"github.com/eduardoddddddd/YourFlix/server/internal/parse"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

// matcher asigna a cada archivo su ficha, en cascada:
//  1. semilla por bytes exactos (+ duración si hay varias candidatas);
//  2. ficha existente (semilla, TMDB o manual) por título normalizado + año;
//  3. ficha provisional «filename» con lo deducido del nombre (TMDB la completará).
type matcher struct {
	db   *store.DB
	kind string
}

func (m *matcher) match(f *store.File, p parse.Parsed) string {
	// 0. Mismo contenido ya identificado en otra fuente (p. ej. Platoon en T7 y en Patriot):
	//    un título, dos copias.
	if twins, _ := m.db.FilesByFingerprint(f.Fingerprint); len(twins) > 0 {
		for _, t := range twins {
			if t.ID != f.ID && t.TitleID != "" && t.MatchSource != "filename" {
				f.TitleID, f.Season, f.Episode, f.Label, f.MatchSource = t.TitleID, t.Season, t.Episode, t.Label, t.MatchSource
				return "seed-title"
			}
		}
	}
	if c := m.bySeedBytes(f); c != nil {
		f.TitleID, f.Season, f.Episode, f.Label, f.MatchSource = c.TitleID, c.Season, c.Episode, c.Label, "seed-bytes"
		return f.MatchSource
	}
	f.Season, f.Episode, f.Label = p.Season, p.Episode, ""
	norm := parse.Normalize(p.Title)
	if norm == "" {
		norm = "sin titulo"
	}
	year := p.Year
	if p.Kind == "series" {
		year = "" // el año de una carpeta de serie no siempre es el de estreno
	}
	cands, _ := m.db.TitlesByNorm(p.Kind, norm, year)
	if year == "" && p.Kind == "movie" {
		// Sin año, «Drácula» puede ser la de 1931, 1992 o 2025: solo vale si hay una única ficha.
		n := 0
		for _, t := range cands {
			if t.MatchSource != "filename" {
				n++
			}
		}
		if n > 1 {
			cands = filterFilename(cands)
		}
	}
	var best *store.Title
	for _, t := range cands {
		if best == nil || (t.MatchSource != "filename" && best.MatchSource == "filename") || (year != "" && t.Year == year && best.Year != year) {
			best = t
		}
	}
	if best != nil && best.MatchSource != "filename" {
		f.TitleID, f.MatchSource = best.ID, "seed-title"
		if best.MatchSource == "tmdb" {
			f.MatchSource = "tmdb"
		}
		return "seed-title"
	}
	if best == nil {
		best = &store.Title{ID: store.NewID(), Kind: p.Kind, Title: p.Title, Year: p.Year, MatchSource: "filename", Norm: norm}
		if best.Title == "" {
			best.Title = "Sin título"
		}
		if err := m.db.UpsertTitle(best, false); err != nil {
			return ""
		}
	}
	f.TitleID, f.MatchSource = best.ID, "filename"
	return "filename"
}

func filterFilename(ts []*store.Title) []*store.Title {
	var out []*store.Title
	for _, t := range ts {
		if t.MatchSource == "filename" {
			out = append(out, t)
		}
	}
	return out
}

func (m *matcher) bySeedBytes(f *store.File) *store.SeedCopy {
	copies, err := m.db.SeedCopiesByBytes(f.Size)
	if err != nil || len(copies) == 0 {
		return nil
	}
	best := &copies[0]
	if len(copies) > 1 && f.Duration > 0 {
		for i := range copies {
			if math.Abs(copies[i].Seconds-f.Duration) < math.Abs(best.Seconds-f.Duration) {
				best = &copies[i]
			}
		}
	}
	// Mismos bytes pero duración distinta: no es la misma copia.
	if f.Duration > 0 && best.Seconds > 0 && math.Abs(best.Seconds-f.Duration) > 2 {
		return nil
	}
	return best
}
