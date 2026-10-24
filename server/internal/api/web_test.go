package api

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
	"github.com/eduardoddddddd/YourFlix/server/internal/images"
	"github.com/eduardoddddddd/YourFlix/server/internal/remux"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
	"github.com/eduardoddddddd/YourFlix/server/internal/subs"
)

// setupWeb: biblioteca con un MKV de verdad (si hay ffmpeg), un SRT al lado y la carpeta web.
func setupWeb(t *testing.T, withRemux bool) (http.Handler, *store.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	lib := filepath.Join(dir, "lib")
	os.MkdirAll(lib, 0o755)
	video := filepath.Join(lib, "peli.mkv")
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=25",
			"-f", "lavfi", "-i", "sine=sample_rate=48000", "-t", "20", "-c:v", "libx264", "-preset", "ultrafast", "-g", "50", "-c:a", "ac3", video).CombinedOutput()
		if err != nil {
			t.Fatalf("ffmpeg: %v %s", err, out)
		}
	} else {
		os.WriteFile(video, []byte("no es vídeo"), 0o644)
	}
	os.WriteFile(filepath.Join(lib, "peli.es.srt"), []byte("1\r\n00:00:01,000 --> 00:00:02,500\r\nHola <i>y</i> adiós & más\r\n\r\n2\r\n00:01:02,000 --> 00:01:04,000\r\nDos\r\nlíneas\r\n"), 0o644)
	src, _ := db.AddSource("Ally", lib, "auto", nil)
	db.UpsertTitle(&store.Title{ID: "t1", Kind: "movie", Title: "Platoon", Year: "1986", MatchSource: "seed", Norm: "platoon"}, false)
	db.SaveFile(&store.File{ID: "f1", SourceID: src.ID, RelPath: "peli.mkv", Size: 1, Fingerprint: "fp1", Container: "mkv", TitleID: "t1",
		Audio: []byte(`[{"index":1,"codec":"ac3","channels":2}]`), Video: []byte(`{"codec":"h264","width":320,"height":180}`)})
	web := filepath.Join(dir, "web")
	os.MkdirAll(filepath.Join(web, "js"), 0o755)
	os.MkdirAll(filepath.Join(web, "tests"), 0o755)
	os.WriteFile(filepath.Join(web, "index.html"), []byte("<!doctype html><title>YourFlix</title>"), 0o644)
	os.WriteFile(filepath.Join(web, "js", "app.js"), []byte("'use strict';"), 0o644)
	os.WriteFile(filepath.Join(web, "tests", "x.test.js"), []byte("//"), 0o644)
	prof, _ := compat.Load("")
	s := &Server{DB: db, Images: images.New(filepath.Join(dir, "img")), Subs: &subs.Extractor{FFmpeg: "ffmpeg", Dir: filepath.Join(dir, "subs")},
		Profile: prof, Version: "test", Log: log.New(io.Discard, "", 0)}
	wb := &Web{API: s, Dir: web}
	if withRemux {
		wb.Remux = remux.New(remux.Config{FFmpeg: "ffmpeg", FFprobe: "ffprobe", Dir: filepath.Join(dir, "remux"), Encoder: "off", Log: log.New(io.Discard, "", 0)})
		t.Cleanup(wb.Remux.Close)
	}
	return wb.Handler(), db
}

func TestWebStatic(t *testing.T) {
	h, _ := setupWeb(t, false)
	w := do(h, "GET", "/", lan, nil, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<title>YourFlix") || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("/: %d %q", w.Code, w.Body.String())
	}
	if w := do(h, "GET", "/js/app.js", lan, nil, ""); w.Code != 200 {
		t.Fatalf("js: %d", w.Code)
	}
	for _, p := range []string{"/js/", "/tests/x.test.js", "/t.db", "/%2e%2e/t.db", "/nada.html", "/api/v1/nada"} {
		if w := do(h, "GET", p, lan, nil, ""); w.Code != 404 {
			t.Errorf("%s: %d, quiero 404", p, w.Code)
		}
	}
	if w := do(h, "GET", "/", "203.0.113.9:1234", nil, ""); w.Code != http.StatusForbidden {
		t.Fatalf("la web también es solo LAN: %d", w.Code)
	}
	// La API de la TV sigue igual detrás del mismo manejador.
	if w := do(h, "GET", "/api/v1/ping", lan, nil, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"YourFlix"`) {
		t.Fatalf("ping: %d %s", w.Code, w.Body.String())
	}
	if w := do(h, "PUT", "/api/v1/progress/f1", lan, nil, `{"position":600,"duration":1000}`); w.Code != 204 {
		t.Fatalf("progreso: %d", w.Code)
	}
	// Sin remux: estado «no disponible» y las rutas HLS lo dicen.
	if w := do(h, "GET", "/api/v1/remux", lan, nil, ""); !strings.Contains(w.Body.String(), `"available":false`) {
		t.Fatalf("estado: %s", w.Body.String())
	}
	if w := do(h, "GET", "/api/v1/files/f1/hls/a1-copy/master.m3u8", lan, nil, ""); w.Code != http.StatusNotImplemented {
		t.Fatalf("hls sin remux: %d", w.Code)
	}
}

func TestWebVTT(t *testing.T) {
	h, _ := setupWeb(t, false)
	w := do(h, "GET", "/api/v1/files/f1/subtitles", lan, nil, "")
	if !strings.Contains(w.Body.String(), `"id":"x0"`) {
		t.Fatalf("pistas: %s", w.Body.String())
	}
	w = do(h, "GET", "/api/v1/files/f1/vtt/x0", lan, nil, "")
	want := "WEBVTT\n\n00:00:01.000 --> 00:00:02.500\nHola y adiós &amp; más\n\n00:01:02.000 --> 00:01:04.000\nDos\nlíneas\n\n"
	if w.Code != 200 || w.Body.String() != want || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/vtt") {
		t.Fatalf("vtt %d %q", w.Code, w.Body.String())
	}
	if got := WebVTT([]subs.Cue{{3725.5, 3726.25, "a <b> c\n\n-->"}, {5.0, 4.0, "al revés"}}); got != "WEBVTT\n\n01:02:05.500 --> 01:02:06.250\na &lt;b&gt; c\n--&gt;\n\n" {
		t.Fatalf("WebVTT: %q", got)
	}
	if w := do(h, "GET", "/api/v1/files/nada/vtt/x0", lan, nil, ""); w.Code != 404 {
		t.Fatalf("archivo desconocido: %d", w.Code)
	}
}

func TestWebHLS(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("sin ffmpeg")
	}
	h, db := setupWeb(t, true)
	if w := do(h, "GET", "/api/v1/remux", lan, nil, ""); !strings.Contains(w.Body.String(), `"available":true`) || !strings.Contains(w.Body.String(), `"videoEncoder":""`) {
		t.Fatalf("estado: %s", w.Body.String())
	}
	for path, code := range map[string]int{
		"/api/v1/files/f1/hls/zz/master.m3u8":       400, // variante mal formada
		"/api/v1/files/f1/hls/a5-copy/master.m3u8":  400, // pista de audio que no existe
		"/api/v1/files/nada/hls/a1-copy/index.m3u8": 404,
		"/api/v1/files/f1/hls/a1-h264/master.m3u8":  409, // sin codificador (--video-encoder off)
		"/api/v1/files/f1/hls/a1-copy/99.m4s":       404,
		"/api/v1/files/f1/hls/a1-copy/x.m4s":        404,
	} {
		if w := do(h, "GET", path, lan, nil, ""); w.Code != code {
			t.Errorf("%s: %d, quiero %d (%s)", path, w.Code, code, strings.TrimSpace(w.Body.String()))
		}
	}
	w := do(h, "GET", "/api/v1/files/f1/hls/a1-copy/master.m3u8?s=web1&t=12", lan, nil, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "index.m3u8?s=web1&from=") || w.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
		t.Fatalf("maestra %d: %s", w.Code, w.Body.String())
	}
	w = do(h, "GET", "/api/v1/files/f1/hls/a1-copy/index.m3u8?s=web1&from=1", lan, nil, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `#EXT-X-MAP:URI="init.mp4?s=web1&from=1"`) || !strings.Contains(w.Body.String(), "#EXT-X-ENDLIST") {
		t.Fatalf("lista %d: %s", w.Code, w.Body.String())
	}
	for _, p := range []string{"init.mp4?s=web1&from=1", "1.m4s?s=web1", "0.m4s?s=web1"} {
		w := do(h, "GET", "/api/v1/files/f1/hls/a1-copy/"+p, lan, nil, "")
		if w.Code != 200 || w.Body.Len() < 100 || w.Header().Get("Content-Type") != "video/mp4" {
			t.Fatalf("%s: %d, %d bytes", p, w.Code, w.Body.Len())
		}
	}
	if w := do(h, "GET", "/api/v1/files/f1/hls/a1-copy/0.m4s?s=web1", lan, map[string]string{"Range": "bytes=0-99"}, ""); w.Code != 206 || w.Body.Len() != 100 {
		t.Fatalf("Range en un segmento: %d %d", w.Code, w.Body.Len())
	}
	if w := do(h, "POST", "/api/v1/files/f1/hls/a1-copy/stop?s=web1", lan, nil, ""); w.Code != 204 {
		t.Fatalf("stop: %d", w.Code)
	}
	// Disco desconectado: 410 como /stream.
	f, _ := db.File("f1")
	f.MissingSince = store.Now()
	db.SaveFile(f)
	if w := do(h, "GET", "/api/v1/files/f1/hls/a1-copy/index.m3u8", lan, nil, ""); w.Code != http.StatusGone {
		t.Fatalf("desconectado: %d", w.Code)
	}
}
