package parse

import "testing"

func TestFile(t *testing.T) {
	cases := []struct {
		rel, kind, title, year string
		season, episode        int
	}{
		{"No hay salida (1987) [HD 1080] [ESP-ING + subs] (Geot).mkv", "movie", "No hay salida", "1987", 0, 0},
		{"Platoon.1986.1080p.BluRay.x264.mkv", "movie", "Platoon", "1986", 0, 0},
		{"El profesional (Léon) 1994 BDRip.mkv", "movie", "El profesional (Léon)", "1994", 0, 0},
		{"1917 (2019) WEB-DL 1080p.mkv", "movie", "1917", "2019", 0, 0},
		{"Peliculas/Blade Runner/Blade.Runner.1982.Final.Cut.mkv", "movie", "Blade Runner", "1982", 0, 0},
		{"Andor/Andor.S01E03.1080p.WEB-DL.mkv", "series", "Andor", "", 1, 3},
		{"Westworld 2016/Westworld 2x05 Castellano.mkv", "series", "Westworld", "2016", 2, 5},
		{"Gomorra [Cap.105].mkv", "series", "Gomorra", "", 1, 5},
		{"Retorno a Brideshead/Temporada 1/Capitulo 3.avi", "series", "Retorno a Brideshead", "", 1, 3},
		{"The.Bear.S03E10.HDR.2160p.mkv", "series", "The Bear", "", 3, 10},
		{"Series/The Bear/The.Bear.S01E01.1080p.mkv", "series", "The Bear", "", 1, 1},
		{"Series/La casa de papel/La.casa.de.papel.1x01.mkv", "series", "La casa de papel", "", 1, 1},
		{"Series/Gomorra/Temporada 2/Capitulo 4.mkv", "series", "Gomorra", "", 2, 4},
	}
	for _, c := range cases {
		p := File(c.rel, "auto")
		got := func(n *int) int {
			if n == nil {
				return 0
			}
			return *n
		}
		if p.Kind != c.kind || p.Title != c.title || p.Year != c.year || got(p.Season) != c.season || got(p.Episode) != c.episode {
			t.Errorf("%s:\n  got  %s %q %q S%d E%d\n  want %s %q %q S%d E%d", c.rel, p.Kind, p.Title, p.Year, got(p.Season), got(p.Episode), c.kind, c.title, c.year, c.season, c.episode)
		}
	}
}

func TestSourceKind(t *testing.T) {
	if p := File("Batman 1966/Batman 012.avi", "series"); p.Kind != "series" || p.Title != "Batman" {
		t.Errorf("serie forzada: %+v", p)
	}
	if p := File("Docs/Cosmos S01E01.mkv", "movies"); p.Kind != "movie" {
		t.Errorf("película forzada: %+v", p)
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"No Hay Salida": "no hay salida",
		"El Señor de los Anillos: El retorno del rey": "el senor de los anillos el retorno del rey",
		"Sirāt: Trance en el desierto":                "sirat trance en el desierto",
		"Fast & Furious 5":                            "fast y furious 5",
		"Alien³":                                      "alien 3",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
