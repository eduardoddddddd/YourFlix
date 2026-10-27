package facets

import "strings"

// Nombres de país de TMDB (inglés) → castellano. Los que falten se dejan como vienen.
var countryES = map[string]string{
	"United States of America": "Estados Unidos", "United States": "Estados Unidos", "USA": "Estados Unidos",
	"United Kingdom": "Reino Unido", "France": "Francia", "Spain": "España", "Germany": "Alemania",
	"West Germany": "Alemania Occidental", "East Germany": "Alemania Oriental", "Italy": "Italia",
	"Australia": "Australia", "New Zealand": "Nueva Zelanda", "Canada": "Canadá", "Hong Kong": "Hong Kong",
	"India": "India", "Japan": "Japón", "Ireland": "Irlanda", "Mexico": "México", "Belgium": "Bélgica",
	"Poland": "Polonia", "Czech Republic": "República Checa", "Czechia": "República Checa", "Czechoslovakia": "Checoslovaquia",
	"South Korea": "Corea del Sur", "Korea": "Corea del Sur", "Soviet Union": "Unión Soviética", "Russia": "Rusia",
	"Argentina": "Argentina", "China": "China", "Vietnam": "Vietnam", "Philippines": "Filipinas", "Finland": "Finlandia",
	"Singapore": "Singapur", "Peru": "Perú", "Hungary": "Hungría", "Denmark": "Dinamarca", "Sweden": "Suecia",
	"Norway": "Noruega", "United Arab Emirates": "Emiratos Árabes Unidos", "Malaysia": "Malasia",
	"South Africa": "Sudáfrica", "Taiwan": "Taiwán", "Netherlands": "Países Bajos", "Switzerland": "Suiza",
	"Austria": "Austria", "Portugal": "Portugal", "Brazil": "Brasil", "Chile": "Chile", "Colombia": "Colombia",
	"Cuba": "Cuba", "Greece": "Grecia", "Turkey": "Turquía", "Iran": "Irán", "Israel": "Israel", "Morocco": "Marruecos",
	"Luxembourg": "Luxemburgo", "Iceland": "Islandia", "Romania": "Rumanía", "Thailand": "Tailandia",
}

// CountryES traduce un país; los que ya están en castellano o no se conocen quedan igual.
func CountryES(name string) string {
	name = strings.TrimSpace(name)
	if es, ok := countryES[name]; ok {
		return es
	}
	return name
}

// Countries separa el texto de TMDB («United States of America, United Kingdom») y traduce.
func Countries(s string) []string {
	var out []string
	for _, c := range strings.Split(s, ",") {
		if c = CountryES(c); c != "" {
			out = add(out, c)
		}
	}
	return out
}

// Códigos de idioma de ffprobe (ISO 639-2 B o T, a veces 639-1) → uno solo.
var langAlias = map[string]string{
	"es": "spa", "esp": "spa", "en": "eng", "fr": "fre", "fra": "fre", "de": "ger", "deu": "ger", "it": "ita",
	"ja": "jpn", "pt": "por", "ca": "cat", "eu": "baq", "eus": "baq", "gl": "glg", "ru": "rus", "zh": "chi", "zho": "chi",
	"ko": "kor", "nl": "dut", "nld": "dut", "sv": "swe", "da": "dan", "no": "nor", "fi": "fin", "pl": "pol", "cs": "cze", "ces": "cze",
	"hu": "hun", "el": "gre", "ell": "gre", "tr": "tur", "ar": "ara", "he": "heb", "hi": "hin",
}

var langES = map[string]string{
	"spa": "Castellano", "eng": "Inglés", "fre": "Francés", "ger": "Alemán", "ita": "Italiano", "jpn": "Japonés",
	"por": "Portugués", "cat": "Catalán", "baq": "Euskera", "glg": "Gallego", "rus": "Ruso", "chi": "Chino", "kor": "Coreano",
	"dut": "Neerlandés", "swe": "Sueco", "dan": "Danés", "nor": "Noruego", "fin": "Finés", "pol": "Polaco", "cze": "Checo",
	"hun": "Húngaro", "gre": "Griego", "tur": "Turco", "ara": "Árabe", "heb": "Hebreo", "hin": "Hindi",
}

// Lang normaliza un código de idioma; vacío si no se sabe («und»).
func Lang(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if a, ok := langAlias[c]; ok {
		c = a
	}
	if c == "und" || c == "mis" || c == "zxx" || len(c) < 2 {
		return ""
	}
	return c
}

func LangName(code string) string {
	if n, ok := langES[code]; ok {
		return n
	}
	return strings.ToUpper(code)
}
