// Package parse deduce título, año, temporada y capítulo del nombre de un vídeo.
// Las reglas de limpieza vienen de CineT7/work/build_catalog.py (clean_title).
package parse

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type Parsed struct {
	Kind    string // movie | series
	Title   string // película o serie
	Year    string
	Season  *int
	Episode *int
}

var (
	reSep       = regexp.MustCompile(`[._]+`)
	reBrackets  = regexp.MustCompile(`\[[^\]]*\]|\{[^}]*\}`)
	reParens    = regexp.MustCompile(`\(([^)]*)\)`)
	reYear      = regexp.MustCompile(`(?:^|[^\d])((?:19|20)\d{2})(?:[^\d]|$)`)
	reTech      = regexp.MustCompile(`(?i)\b(?:WEB[ -]?DL|WEB[ -]?Rip|BDRip|BRRip|BluRay|Blu-Ray|HDRip|HDTS|HDTV|DVDRip|DVD5|DVD9|REMUX|Micro(?:HD)?|x26[45]|h\.?26[45]|HEVC|AVC|AAC|AC3|EAC3|DDP?5\.?1|DTS|1080p|2160p|720p|480p|4K|UHD|HDR10?|10bit|Dual|Castellano|Spanish|ESP|VOSE|Subs?|Extended|Directors?\.? ?Cut|Remastered|Proper|Repack)\b.*`)
	reGroupTail = regexp.MustCompile(`(?i)\s*-\s*(?:xusman|theboss|kowalski|scorpion|canibales|WEBDL|Geot)\b.*`)
	reSxE       = regexp.MustCompile(`(?i)\bS(\d{1,2})[ ._-]?E(\d{1,3})\b`)
	reNxN       = regexp.MustCompile(`(?i)\b(\d{1,2})x(\d{1,3})\b`)
	reCap       = regexp.MustCompile(`(?i)\bCap(?:itulo|ítulo)?[ ._-]*(\d)(\d{2})\b`)
	reSeason    = regexp.MustCompile(`(?i)\b(?:Temporada|Season|T)[ ._-]*(\d{1,2})\b`)
	reEpisode   = regexp.MustCompile(`(?i)\b(?:Cap(?:itulo|ítulo)?|Episodio|Episode|Ep)[ ._-]*(\d{1,3})\b`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// File analiza una ruta relativa a su fuente. sourceKind: auto | movies | series.
func File(rel, sourceKind string) Parsed {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	stem := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(rel))
	top := ""
	if len(parts) > 1 {
		top = parts[0]
	}

	season, episode, cut := episodeOf(stem)
	if season == nil && top != "" {
		// «Serie/Temporada 2/Capítulo 3.mkv»
		for _, dir := range parts[1 : len(parts)-1] {
			if m := reSeason.FindStringSubmatch(dir); m != nil {
				season = atoi(m[1])
			}
		}
		if m := reEpisode.FindStringSubmatch(stem); m != nil && season != nil {
			episode = atoi(m[1])
		}
	}

	isSeries := sourceKind == "series" || (sourceKind != "movies" && episode != nil)
	p := Parsed{Kind: "movie", Season: season, Episode: episode}
	if isSeries {
		p.Kind = "series"
		name := top
		if name == "" {
			name = stem[:cut]
		}
		p.Title, p.Year = Title(name)
		if p.Title == "" {
			p.Title, p.Year = Title(stem)
		}
		return p
	}
	p.Season, p.Episode = nil, nil
	p.Title, p.Year = Title(stem)
	if len([]rune(p.Title)) < 2 && top != "" {
		p.Title, p.Year = Title(top)
	}
	return p
}

// episodeOf busca SxxEyy, NxNN o Cap.105 y devuelve dónde empieza el patrón.
func episodeOf(s string) (season, episode *int, cut int) {
	for _, re := range []*regexp.Regexp{reSxE, reNxN, reCap} {
		if m := re.FindStringSubmatchIndex(s); m != nil {
			return atoi(s[m[2]:m[3]]), atoi(s[m[4]:m[5]]), m[0]
		}
	}
	return nil, nil, len(s)
}

// Title limpia un nombre de archivo o carpeta y extrae el año.
func Title(raw string) (title, year string) {
	s := reSep.ReplaceAllString(raw, " ")
	year, at := findYear(s)
	if year != "" {
		// El año marca el final del título: «Platoon 1986 1080p», «1917 (2019) WEB-DL».
		s = s[:at]
	}
	s = reBrackets.ReplaceAllString(s, " ")
	// Paréntesis: fuera los técnicos y el año; se conserva texto como «(Léon)».
	s = reParens.ReplaceAllStringFunc(s, func(g string) string {
		in := strings.TrimSpace(g[1 : len(g)-1])
		if reYear.MatchString(" "+in+" ") || reTech.MatchString(in) || len(in) <= 4 {
			return " "
		}
		return g
	})
	s = reGroupTail.ReplaceAllString(s, "")
	s = reTech.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.Trim(s, " -.,·[({"), year
}

// findYear elige el año del título: primero uno entre paréntesis o corchetes; si no,
// el último año suelto que no esté al principio (en «1917» o «2001 Una odisea…» es título).
func findYear(s string) (year string, at int) {
	ms := reYear.FindAllStringSubmatchIndex(s, -1)
	for _, m := range ms {
		if m[2] > 0 && strings.ContainsAny(s[m[2]-1:m[2]], "([") {
			return s[m[2]:m[3]], m[2] - 1
		}
	}
	for i := len(ms) - 1; i >= 0; i-- {
		if m := ms[i]; m[2] > 0 {
			return s[m[2]:m[3]], m[2]
		}
	}
	return "", 0
}

var fold = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "å", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o", "ø", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c", "ß", "ss", "æ", "ae", "œ", "oe", "ā", "a",
	"&", " y ",
)

// Normalize: minúsculas, sin tildes ni signos, espacios simples. Sirve para emparejar.
func Normalize(s string) string {
	s = fold.Replace(strings.ToLower(s))
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space && b.Len() > 0 {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func atoi(s string) *int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}
