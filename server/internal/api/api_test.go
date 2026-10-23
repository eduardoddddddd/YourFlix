package api

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
	"github.com/eduardoddddddd/YourFlix/server/internal/images"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

func setup(t *testing.T) (http.Handler, *store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	lib := filepath.Join(dir, "lib")
	os.MkdirAll(lib, 0o755)
	data := make([]byte, 1<<20)
	rand.Read(data)
	os.WriteFile(filepath.Join(lib, "peli.mkv"), data, 0o644)
	src, _ := db.AddSource("T7", lib, "auto", nil)
	db.UpsertTitle(&store.Title{ID: "t7-1", Kind: "movie", Title: "Platoon", Year: "1986", MatchSource: "seed", Norm: "platoon"}, false)
	db.SaveFile(&store.File{ID: "f1", SourceID: src.ID, RelPath: "peli.mkv", Size: 1 << 20, Fingerprint: "x", Container: "mkv", TitleID: "t7-1",
		Audio: []byte(`[{"codec":"dts","channels":6}]`), Video: []byte(`{"codec":"h264","width":1920,"height":1080}`)})
	prof, _ := compat.Load("")
	s := &Server{DB: db, Images: images.New(filepath.Join(dir, "img")), Profile: prof, Version: "test", Log: log.New(io.Discard, "", 0)}
	return s.Handler(), db, string(data)
}

func do(h http.Handler, method, url, remote string, hdr map[string]string, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, strings.NewReader(body))
	r.RemoteAddr = remote
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const lan = "192.168.1.4:40000"

func TestLANOnly(t *testing.T) {
	h, _, _ := setup(t)
	if w := do(h, "GET", "/api/v1/ping", "203.0.113.9:1234", nil, ""); w.Code != http.StatusForbidden {
		t.Fatalf("IP pública: %d", w.Code)
	}
	if w := do(h, "GET", "/api/v1/ping", lan, nil, ""); w.Code != 200 {
		t.Fatalf("LAN: %d", w.Code)
	}
}

func TestStreamRange(t *testing.T) {
	h, _, data := setup(t)
	w := do(h, "GET", "/api/v1/files/f1/stream", lan, map[string]string{"Range": "bytes=1000-1999"}, "")
	if w.Code != http.StatusPartialContent || w.Body.String() != data[1000:2000] {
		t.Fatalf("rango: %d, %d bytes", w.Code, w.Body.Len())
	}
	if got := w.Header().Get("Content-Range"); got != "bytes 1000-1999/1048576" {
		t.Fatalf("Content-Range: %q", got)
	}
	if w.Header().Get("Content-Type") != "video/x-matroska" {
		t.Fatalf("Content-Type: %q", w.Header().Get("Content-Type"))
	}
	if w = do(h, "GET", "/api/v1/files/f1/stream", lan, map[string]string{"Range": "bytes=-100"}, ""); w.Body.String() != data[len(data)-100:] {
		t.Fatal("rango sufijo (AVPlay lee el final del MKV al abrir)")
	}
	if w = do(h, "GET", "/api/v1/files/../../etc/passwd/stream", lan, nil, ""); w.Code == 200 {
		t.Fatal("no debe aceptar rutas")
	}
	if w = do(h, "GET", "/api/v1/files/nope/stream", lan, nil, ""); w.Code != 404 {
		t.Fatalf("ID desconocido: %d", w.Code)
	}
}

func TestTitleDetailAndProgress(t *testing.T) {
	h, _, _ := setup(t)
	if w := do(h, "PUT", "/api/v1/progress/f1", lan, nil, `{"position":600,"duration":6000}`); w.Code != 204 {
		t.Fatalf("progreso: %d", w.Code)
	}
	w := do(h, "GET", "/api/v1/titles/t7-1", lan, nil, "")
	var d struct {
		Files []struct {
			ID       string
			Compat   compat.Verdict
			Progress *store.Progress
			Stream   string
		}
	}
	json.Unmarshal(w.Body.Bytes(), &d)
	if len(d.Files) != 1 || d.Files[0].Compat.Status != "optimize" || d.Files[0].Progress == nil || d.Files[0].Progress.Position != 600 {
		t.Fatalf("ficha: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "peli.mkv") {
		t.Fatal("la ruta del archivo no debe salir por la API")
	}
	w = do(h, "GET", "/api/v1/titles?q=plat", lan, nil, "")
	if !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatalf("búsqueda: %s", w.Body.String())
	}
}

func TestContinueAndGenres(t *testing.T) {
	h, db, _ := setup(t)
	tt, _ := db.Title("t7-1")
	tt.Meta = []byte(`{"genres":["Bélica","Drama"]}`)
	db.UpsertTitle(tt, false)
	do(h, "PUT", "/api/v1/progress/f1", lan, nil, `{"position":30,"duration":6000}`)
	if w := do(h, "GET", "/api/v1/continue", lan, nil, ""); w.Body.String() != "[]\n" {
		t.Fatalf("menos de 60 s no cuenta: %s", w.Body.String())
	}
	do(h, "PUT", "/api/v1/progress/f1", lan, nil, `{"position":1200,"duration":6000}`)
	if w := do(h, "GET", "/api/v1/continue", lan, nil, ""); !strings.Contains(w.Body.String(), `"fileId":"f1"`) || !strings.Contains(w.Body.String(), `"title":"Platoon"`) {
		t.Fatalf("seguir viendo: %s", w.Body.String())
	}
	do(h, "PUT", "/api/v1/progress/f1", lan, nil, `{"position":5800,"duration":6000}`)
	if w := do(h, "GET", "/api/v1/continue", lan, nil, ""); w.Body.String() != "[]\n" {
		t.Fatalf("vista (>92 %%) sale de seguir viendo: %s", w.Body.String())
	}
	if w := do(h, "GET", "/api/v1/genres", lan, nil, ""); !strings.Contains(w.Body.String(), `{"name":"Bélica","count":1}`) {
		t.Fatalf("géneros: %s", w.Body.String())
	}
	if w := do(h, "GET", "/api/v1/titles?genre=Drama", lan, nil, ""); !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatalf("filtro por género: %s", w.Body.String())
	}
	if w := do(h, "GET", "/api/v1/titles?genre=Western", lan, nil, ""); !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatalf("filtro por género vacío: %s", w.Body.String())
	}
}
