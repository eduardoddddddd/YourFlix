package subs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
)

func TestParseSRT(t *testing.T) {
	src := "\xEF\xBB\xBF1\r\n00:00:05,500 --> 00:00:07,000\r\n<i>¿Qué</i> tal?\r\n{\\an8}Segunda línea\r\n\r\n" +
		"3\n00:01:02.25 --> 00:01:04,000\nPosterior\n\n2\n00:00:10,000 --> 00:00:12,000\nAnterior\n\n" +
		"4\n00:00:20,000 --> 00:00:20,000\nduración cero\n"
	c := ParseSRT([]byte(src))
	if len(c) != 3 {
		t.Fatalf("esperaba 3 líneas, %d: %v", len(c), c)
	}
	if c[0][0] != 5.5 || c[0][1] != 7.0 || c[0][2] != "¿Qué tal?\nSegunda línea" {
		t.Fatalf("primera: %v", c[0])
	}
	if c[1][2] != "Anterior" || c[2][0] != 62.25 {
		t.Fatalf("orden por tiempo y milisegundos de 2 cifras: %v", c)
	}
}

func TestParseSRTWindows1252(t *testing.T) {
	// «Señor, ¿adónde vas?» en Windows-1252, como muchos SRT españoles antiguos.
	raw := []byte("1\n00:00:01,000 --> 00:00:02,000\nSe\xf1or, \xbfad\xf3nde vas? \x93ya\x94\n")
	c := ParseSRT(raw)
	if len(c) != 1 || c[0][2] != "Señor, ¿adónde vas? “ya”" {
		t.Fatalf("cp1252: %v", c)
	}
}

func TestTracks(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Platoon (1986).mkv")
	for _, n := range []string{"Platoon (1986).mkv", "Platoon (1986).es.srt", "Platoon (1986).en.forced.srt", "Platoon (1986) 2.srt", "Otra.srt", "Platoon (1986).spa.ass"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	emb := []probe.Subtitle{
		{Index: 3, Codec: "subrip", Language: "spa", Title: "Forzados", Text: true},
		{Index: 4, Codec: "subrip", Language: "spa", Default: true, Text: true},
		{Index: 5, Codec: "hdmv_pgs_subtitle", Language: "eng"},
	}
	tr := Tracks(video, emb)
	var got []string
	for _, x := range tr {
		got = append(got, x.ID+" "+x.Label)
	}
	want := []string{"e3 Español (forzados)", "e4 Español", "x0 Inglés (forzados) · archivo", "x1 Español · archivo", "x2 Español · archivo"}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Fatalf("pistas:\n got %v\nwant %v", got, want)
	}
}

// Extracción real: MKV con dos pistas SRT generado con ffmpeg (se salta si no hay ffmpeg).
func TestExtractEmbedded(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("sin ffmpeg")
	}
	dir := t.TempDir()
	es := filepath.Join(dir, "es.srt")
	en := filepath.Join(dir, "en.srt")
	os.WriteFile(es, []byte("1\n00:00:01,000 --> 00:00:02,500\nHola\n\n2\n00:00:03,000 --> 00:00:04,000\nAdiós\n"), 0o644)
	os.WriteFile(en, []byte("1\n00:00:01,000 --> 00:00:02,500\nHello\n"), 0o644)
	video := filepath.Join(dir, "v.mkv")
	out, err := exec.Command(ff, "-v", "error", "-f", "lavfi", "-i", "color=black:s=64x64:d=5", "-i", es, "-i", en,
		"-map", "0", "-map", "1", "-map", "2", "-c:v", "libx264", "-c:s", "srt", video).CombinedOutput()
	if err != nil {
		t.Skipf("no se pudo generar el MKV: %v %s", err, out)
	}
	tracks := Tracks(video, []probe.Subtitle{{Index: 1, Codec: "subrip", Language: "spa", Text: true}, {Index: 2, Codec: "subrip", Language: "eng", Text: true}})
	x := &Extractor{FFmpeg: ff, Dir: filepath.Join(dir, "cache")}
	c, err := x.Cues(context.Background(), "f1", "abcdef123456", video, tracks, "e1")
	if err != nil || len(c) != 2 || c[1][2] != "Adiós" {
		t.Fatalf("español: %v %v", c, err)
	}
	// La otra pista ya quedó en caché en la misma pasada: no vuelve a leer el vídeo.
	os.Remove(video)
	c, err = x.Cues(context.Background(), "f1", "abcdef123456", video, tracks, "e2")
	if err != nil || len(c) != 1 || c[0][2] != "Hello" {
		t.Fatalf("inglés desde caché: %v %v", c, err)
	}
}
