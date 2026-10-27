package facets

import (
	"net/url"
	"reflect"
	"testing"
)

func TestCountries(t *testing.T) {
	got := Countries("United States of America, United Kingdom,Spain, , Narnia, United States of America")
	want := []string{"Estados Unidos", "Reino Unido", "España", "Narnia"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("países: %q", got)
	}
}

func TestShuffleStable(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	a, b := Shuffle(ids, 8213), Shuffle(ids, 8213)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("misma semilla, distinto orden")
	}
	if reflect.DeepEqual(a, Shuffle(ids, 1)) && reflect.DeepEqual(a, Shuffle(ids, 2)) {
		t.Fatal("la semilla no cambia el orden")
	}
	seen := map[string]bool{}
	for _, x := range a {
		seen[x] = true
	}
	if len(seen) != len(ids) || ids[0] != "a" {
		t.Fatalf("faltan o se repiten: %q (y no debe tocar la lista original: %q)", a, ids)
	}
}

func TestParseFilter(t *testing.T) {
	q, _ := url.ParseQuery("director=Ridley+Scott&decade=80&country=France&audio=es&rating=7&state=unwatched")
	f := ParseFilter(q)
	if f.Decade[0] != "1980" || f.Country[0] != "Francia" || f.Audio[0] != "spa" || f.Rating != 7 || !f.Active() {
		t.Fatalf("%+v", f)
	}
	if (Filter{Kind: "movie", Genre: []string{"Drama"}}).Active() {
		t.Fatal("tipo y género los resuelve el SQL")
	}
	if Band(89) != "short" || Band(90) != "mid" || Band(120) != "mid" || Band(121) != "long" || Band(0) != "" {
		t.Fatal("tramos de duración")
	}
}
