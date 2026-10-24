package admin

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
	"github.com/eduardoddddddd/YourFlix/server/internal/parse"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
	"github.com/eduardoddddddd/YourFlix/server/internal/tmdb"
)

const (
	tv     = "192.168.1.4:40000" // la TV (o cualquier equipo de la LAN sin sesión)
	mac    = "192.168.1.3:50000"
	local  = "127.0.0.1:50000"
	token  = "token-secreto-de-tmdb-123"
	public = "203.0.113.9:1234"
)

func init() { Iterations = 1000 } // los tests no necesitan 600 000 vueltas

type env struct {
	t    *testing.T
	a    *Admin
	db   *store.DB
	dir  string
	ring *Ring
	logs *bytes.Buffer
}

// fakeTMDB: lo justo para buscar y emparejar «Blade Runner».
func fakeTMDB(t *testing.T) *httptest.Server {
	type res = map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/search/movie":
			var out []res
			if parse.Normalize(r.URL.Query().Get("query")) == "blade runner" {
				out = []res{{"id": 78, "title": "Blade Runner", "original_title": "Blade Runner", "release_date": "1982-06-25", "vote_count": 14000, "poster_path": "/p.jpg"}}
			}
			json.NewEncoder(w).Encode(res{"results": out})
		case "/search/tv":
			var out []res
			if parse.Normalize(r.URL.Query().Get("query")) == "andor" {
				out = []res{{"id": 83867, "name": "Andor", "original_name": "Andor", "first_air_date": "2022-09-21", "vote_count": 1500}}
			}
			json.NewEncoder(w).Encode(res{"results": out})
		case "/tv/83867":
			json.NewEncoder(w).Encode(res{"id": 83867, "name": "Andor", "original_name": "Andor", "first_air_date": "2022-09-21", "overview": "Cassian Andor."})
		case "/movie/78":
			json.NewEncoder(w).Encode(res{"id": 78, "title": "Blade Runner", "original_title": "Blade Runner", "release_date": "1982-06-25", "overview": "Los Ángeles, 2019.", "poster_path": "/p.jpg"})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setup(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	prof, _ := compat.Load("")
	logs := &bytes.Buffer{}
	ring := NewRing(100, token)
	lg := log.New(io.MultiWriter(logs, ring), "", 0)
	c := &tmdb.Client{Token: token, Base: fakeTMDB(t).URL, Cache: store.TMDBCache{DB: db}}
	a, err := New(Config{DB: db, TMDB: c, Profile: prof, Version: "test", DataDir: dir, FFprobe: "/nonexistent/ffprobe", FFmpeg: "/nonexistent/ffmpeg",
		WebDir: webDir(t), Log: lg, Ring: ring,
		Roots: func() []Place { return []Place{{Name: "pruebas", Path: dir}} }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return &env{t: t, a: a, db: db, dir: dir, ring: ring, logs: logs}
}

// webDir: la carpeta admin/ real del repositorio.
func webDir(t *testing.T) string {
	d, _ := filepath.Abs("../../../admin")
	if FindWebDir(d) == "" {
		t.Fatalf("no encuentro %s/index.html", d)
	}
	return d
}

type client struct {
	e      *env
	remote string
	host   string
	cookie *http.Cookie
	csrf   string
}

func (e *env) client(remote string) *client {
	host := "192.168.1.13:8787"
	if remote == local {
		host = "localhost:8787"
	}
	return &client{e: e, remote: remote, host: host}
}

func (c *client) do(method, url string, body any) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, url, rd)
	r.RemoteAddr, r.Host = c.remote, c.host
	if body != nil || method != "GET" {
		r.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != nil {
		r.AddCookie(c.cookie)
	}
	if c.csrf != "" {
		r.Header.Set("X-CSRF-Token", c.csrf)
	}
	w := httptest.NewRecorder()
	c.e.a.ServeHTTP(w, r)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == cookieName {
			if ck.MaxAge < 0 {
				c.cookie = nil
			} else {
				c.cookie = ck
			}
		}
	}
	return w
}

// session pide /session y guarda el token CSRF.
func (c *client) session() map[string]any {
	w := c.do("GET", "/admin/api/session", nil)
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	if s, ok := m["csrf"].(string); ok {
		c.csrf = s
	}
	return m
}

func (c *client) login(pw string) int {
	w := c.do("POST", "/admin/api/login", map[string]string{"password": pw})
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	if s, ok := m["csrf"].(string); ok {
		c.csrf = s
	}
	return w.Code
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("JSON: %v: %s", err, w.Body.String())
	}
	return v
}

func writeRandom(t *testing.T, path string, size int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	b := make([]byte, size)
	rand.Read(b)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLANOnlyAndAuth(t *testing.T) {
	e := setup(t)
	if w := e.client(public).do("GET", "/admin/", nil); w.Code != http.StatusForbidden {
		t.Fatalf("IP pública: %d", w.Code)
	}
	tvc := e.client(tv)
	for _, u := range []string{"/admin/api/sources", "/admin/api/browse?path=" + e.dir, "/admin/api/status", "/admin/api/pending", "/admin/api/jobs/events"} {
		if w := tvc.do("GET", u, nil); w.Code != http.StatusUnauthorized {
			t.Fatalf("la TV sin sesión no entra en %s: %d", u, w.Code)
		}
	}
	if m := tvc.session(); m["authenticated"] != false || m["csrf"] != nil {
		t.Fatalf("sesión desde la LAN sin contraseña: %v", m)
	}
	// Desde el propio equipo se entra sin contraseña…
	lc := e.client(local)
	if w := lc.do("GET", "/admin/api/sources", nil); w.Code != 200 {
		t.Fatalf("localhost: %d", w.Code)
	}
	// …pero lo que cambia algo exige el token CSRF de la sesión.
	if w := lc.do("POST", "/admin/api/scan", map[string]any{}); w.Code != http.StatusForbidden {
		t.Fatalf("sin CSRF: %d", w.Code)
	}
	if m := lc.session(); m["authenticated"] != true || lc.csrf == "" || lc.cookie == nil {
		t.Fatalf("sesión local: %v", m)
	}
	if lc.cookie.Path != "/admin/" || !lc.cookie.HttpOnly || lc.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie: %+v", lc.cookie)
	}
	bad := *lc
	bad.csrf = "otro"
	if w := bad.do("POST", "/admin/api/identify", map[string]any{}); w.Code != http.StatusForbidden {
		t.Fatalf("CSRF falso: %d", w.Code)
	}
	// Un formulario de otra web (sin JSON) o con otro Origin no pasa.
	r := httptest.NewRequest("POST", "/admin/api/identify", strings.NewReader("retry=true"))
	r.RemoteAddr, r.Host = local, "localhost:8787"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(lc.cookie)
	r.Header.Set("X-CSRF-Token", lc.csrf)
	w := httptest.NewRecorder()
	e.a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("formulario: %d", w.Code)
	}
	r = httptest.NewRequest("POST", "/admin/api/identify", strings.NewReader("{}"))
	r.RemoteAddr, r.Host = local, "localhost:8787"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://evil.example")
	r.AddCookie(lc.cookie)
	r.Header.Set("X-CSRF-Token", lc.csrf)
	w = httptest.NewRecorder()
	e.a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("otro origen: %d", w.Code)
	}
	// DNS rebinding: desde 127.0.0.1 pero con otro Host no es «local».
	rb := e.client(local)
	rb.host = "evil.example:8787"
	if w := rb.do("GET", "/admin/api/sources", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("Host ajeno desde localhost: %d", w.Code)
	}
	// La cookie local robada no sirve desde otro equipo.
	stolen := e.client(mac)
	stolen.cookie, stolen.csrf = lc.cookie, lc.csrf
	if w := stolen.do("GET", "/admin/api/sources", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("cookie local desde la LAN: %d", w.Code)
	}
}

func TestPasswordLoginLogout(t *testing.T) {
	e := setup(t)
	mc := e.client(mac)
	mc.session()
	if code := mc.login("loquesea1"); code != http.StatusConflict {
		t.Fatalf("sin contraseña definida: %d", code)
	}
	// Nadie de la LAN puede poner la primera contraseña.
	if w := mc.do("POST", "/admin/api/password", map[string]string{"password": "mia-mia-mia"}); w.Code != http.StatusForbidden {
		t.Fatalf("primera contraseña desde la LAN: %d", w.Code)
	}
	lc := e.client(local)
	lc.session()
	if w := lc.do("POST", "/admin/api/password", map[string]string{"password": "corta"}); w.Code != http.StatusBadRequest {
		t.Fatalf("contraseña corta: %d", w.Code)
	}
	if w := lc.do("POST", "/admin/api/password", map[string]string{"password": "cine en casa 1"}); w.Code != http.StatusNoContent {
		t.Fatalf("definir desde localhost: %d %s", w.Code, w.Body.String())
	}
	stored, _, _ := e.db.Setting(passwordKey)
	if strings.Contains(stored, "cine en casa") || !strings.HasPrefix(stored, "pbkdf2-sha256$") {
		t.Fatalf("debe guardarse con hash: %q", stored)
	}
	if code := mc.login("otra cosa"); code != http.StatusUnauthorized {
		t.Fatalf("contraseña mala: %d", code)
	}
	if code := mc.login("cine en casa 1"); code != 200 || mc.cookie == nil || mc.csrf == "" {
		t.Fatalf("entrar: %d", code)
	}
	if w := mc.do("GET", "/admin/api/browse?path="+e.dir, nil); w.Code != 200 {
		t.Fatalf("explorador con sesión: %d", w.Code)
	}
	if w := mc.do("POST", "/admin/api/scan", map[string]any{}); w.Code != http.StatusAccepted && w.Code != http.StatusTooManyRequests {
		t.Fatalf("escaneo con sesión: %d %s", w.Code, w.Body.String())
	}
	// Cambiar la contraseña desde la LAN exige la actual; las demás sesiones se cierran.
	other := e.client("192.168.1.20:1")
	other.login("cine en casa 1")
	if w := mc.do("POST", "/admin/api/password", map[string]string{"current": "mal", "password": "nueva clave 2"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("cambio sin la actual: %d", w.Code)
	}
	if w := mc.do("POST", "/admin/api/password", map[string]string{"current": "cine en casa 1", "password": "nueva clave 2"}); w.Code != http.StatusNoContent {
		t.Fatalf("cambio: %d %s", w.Code, w.Body.String())
	}
	if w := mc.do("GET", "/admin/api/sources", nil); w.Code != 200 {
		t.Fatalf("quien la cambia sigue dentro: %d", w.Code)
	}
	if w := other.do("GET", "/admin/api/sources", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("las otras sesiones se cierran: %d", w.Code)
	}
	// Cambio desde la CLI (otro proceso): la sesión abierta caduca.
	SetPassword(e.db, "desde la cli 3")
	if w := mc.do("GET", "/admin/api/sources", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("tras cambiarla por CLI: %d", w.Code)
	}
	mc.login("desde la cli 3")
	if w := mc.do("POST", "/admin/api/logout", map[string]any{}); w.Code != http.StatusNoContent {
		t.Fatalf("salir: %d", w.Code)
	}
	if w := mc.do("GET", "/admin/api/sources", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("tras salir: %d", w.Code)
	}
	// Límite de intentos.
	att := e.client("192.168.1.66:1")
	for i := 0; i < maxFails; i++ {
		att.login("no-es")
	}
	if code := att.login("desde la cli 3"); code != http.StatusTooManyRequests {
		t.Fatalf("tras %d fallos: %d", maxFails, code)
	}
}

func TestBrowse(t *testing.T) {
	e := setup(t)
	root := filepath.Join(e.dir, "disco")
	os.MkdirAll(filepath.Join(root, "Películas"), 0o755)
	os.MkdirAll(filepath.Join(root, "series"), 0o755)
	os.MkdirAll(filepath.Join(root, ".oculta"), 0o755)
	os.MkdirAll(filepath.Join(root, "$RECYCLE.BIN"), 0o755)
	writeRandom(t, filepath.Join(root, "suelta.mkv"), 10)
	writeRandom(t, filepath.Join(root, "notas.txt"), 10)
	lc := e.client(local)
	type out struct {
		Path, Parent string
		Dirs         []Place
		Roots        []Place
		Videos       int
	}
	o := decode[out](t, lc.do("GET", "/admin/api/browse", nil))
	if o.Path != "" || len(o.Dirs) != 1 || o.Dirs[0].Path != e.dir {
		t.Fatalf("raíces: %+v", o)
	}
	o = decode[out](t, lc.do("GET", "/admin/api/browse?path="+root, nil))
	if len(o.Dirs) != 2 || o.Dirs[0].Name != "Películas" || o.Dirs[1].Name != "series" || o.Videos != 1 || o.Parent != e.dir {
		t.Fatalf("carpeta: %+v", o)
	}
	if w := lc.do("GET", "/admin/api/browse?path=relativa/no", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("ruta relativa: %d", w.Code)
	}
	if w := lc.do("GET", "/admin/api/browse?path="+filepath.Join(root, "no-existe"), nil); w.Code != http.StatusNotFound {
		t.Fatalf("no existe: %d", w.Code)
	}
	if w := lc.do("GET", "/admin/api/browse?path="+filepath.Join(root, "notas.txt"), nil); w.Code == 200 {
		t.Fatal("un archivo no se lista como carpeta")
	}
	top := decode[out](t, lc.do("GET", "/admin/api/browse?path=/", nil))
	if top.Parent != "" {
		t.Fatalf("la raíz no tiene padre: %+v", top)
	}
}

func waitJob(t *testing.T, c *client, id string) *Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		j := decode[Job](t, c.do("GET", "/admin/api/jobs/"+id, nil))
		if j.State == "done" || j.State == "failed" {
			return &j
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("el trabajo %s no termina", id)
	return nil
}

func TestSourcesAndScan(t *testing.T) {
	e := setup(t)
	lib := filepath.Join(e.dir, "T7")
	writeRandom(t, filepath.Join(lib, "Platoon (1986).mkv"), 1<<20+5)
	writeRandom(t, filepath.Join(lib, "Blade Runner (1982).mkv"), 1<<20+7)
	writeRandom(t, filepath.Join(lib, "extras", "sample.mkv"), 100)
	e.db.UpsertTitle(&store.Title{ID: "t7-2", Kind: "movie", Title: "Platoon", Year: "1986", MatchSource: "seed", Norm: "platoon"}, false)
	lc := e.client(local)
	lc.session()

	if w := lc.do("POST", "/admin/api/sources", map[string]any{"path": filepath.Join(e.dir, "no-existe")}); w.Code != http.StatusBadRequest {
		t.Fatalf("carpeta inexistente: %d", w.Code)
	}
	w := lc.do("POST", "/admin/api/sources", map[string]any{"path": lib, "label": "Samsung T7", "kind": "auto", "excludes": []string{"*sample*", " "}, "scan": true})
	if w.Code != http.StatusCreated {
		t.Fatalf("añadir: %d %s", w.Code, w.Body.String())
	}
	add := decode[struct {
		Source store.Source
		Job    *Job
	}](t, w)
	if add.Source.Label != "Samsung T7" || len(add.Source.Excludes) != 1 || add.Job == nil {
		t.Fatalf("fuente: %+v", add)
	}
	if w := lc.do("POST", "/admin/api/sources", map[string]any{"path": lib}); w.Code != http.StatusConflict {
		t.Fatalf("duplicada: %d", w.Code)
	}
	j := waitJob(t, lc, add.Job.ID)
	if j.State != "done" || j.Processed != 2 || len(j.Summary) == 0 || !strings.Contains(j.Summary[0], "2 vídeos") {
		t.Fatalf("escaneo: %+v", j)
	}
	// Tras escanear se identifica con TMDB (como serve): Blade Runner se asigna sola.
	if !strings.Contains(strings.Join(j.Summary, "\n"), "1 identificadas") {
		t.Fatalf("identificación tras escanear: %v", j.Summary)
	}
	list := decode[[]store.Source](t, lc.do("GET", "/admin/api/sources", nil))
	if len(list) != 1 || list[0].Files != 2 || list[0].LastScanAt == "" || list[0].Path != lib {
		t.Fatalf("lista: %+v", list)
	}
	id := list[0].ID

	// Cambiar etiqueta, tipo y exclusiones.
	w = lc.do("PATCH", "/admin/api/sources/"+id, map[string]any{"label": "Ally", "kind": "movies", "excludes": []string{"*sample*", "extras/"}})
	if s := decode[store.Source](t, w); w.Code != 200 || s.Label != "Ally" || s.Kind != "movies" || len(s.Excludes) != 2 {
		t.Fatalf("editar: %d %s", w.Code, w.Body.String())
	}
	if w := lc.do("PATCH", "/admin/api/sources/"+id, map[string]any{"label": "  "}); w.Code != http.StatusBadRequest {
		t.Fatalf("etiqueta vacía: %d", w.Code)
	}
	if w := lc.do("PATCH", "/admin/api/sources/"+id, map[string]any{"kind": "pelis"}); w.Code != http.StatusBadRequest {
		t.Fatalf("tipo no válido: %d", w.Code)
	}
	if w := lc.do("PATCH", "/admin/api/sources/Ally", map[string]any{"label": "x"}); w.Code != http.StatusNotFound {
		t.Fatalf("solo IDs, no etiquetas: %d", w.Code)
	}

	// Pausar y reanudar.
	if s := decode[store.Source](t, lc.do("POST", "/admin/api/sources/"+id+"/pause", map[string]any{})); s.State != "paused" {
		t.Fatalf("pausar: %+v", s)
	}
	j = waitJob(t, lc, decode[Job](t, lc.do("POST", "/admin/api/sources/"+id+"/scan", map[string]any{})).ID)
	if !strings.Contains(j.Summary[0], "pausada") {
		t.Fatalf("una fuente pausada no se escanea: %v", j.Summary)
	}
	if s := decode[store.Source](t, lc.do("POST", "/admin/api/sources/"+id+"/resume", map[string]any{})); s.State != "active" {
		t.Fatalf("reanudar: %+v", s)
	}
	j = waitJob(t, lc, decode[Job](t, lc.do("POST", "/admin/api/scan", map[string]any{})).ID)
	if j.State != "done" {
		t.Fatalf("escanear todo: %+v", j)
	}

	// Estado: últimos escaneos, discos y registro.
	st := decode[struct {
		Version string
		Tools   []toolInfo
		Disks   []diskOut
		Scans   []store.ScanRun
		Sources []struct{ LastScanAt string }
		Log     []string
		TMDB    bool
	}](t, lc.do("GET", "/admin/api/status", nil))
	if st.Version != "test" || len(st.Tools) != 2 || st.Tools[0].Found || len(st.Scans) != 2 || len(st.Disks) == 0 || st.Disks[0].Total == 0 || !st.TMDB {
		t.Fatalf("estado: %+v", st)
	}
	if len(st.Disks[0].Used) != 2 {
		t.Fatalf("datos y fuente están en el mismo disco: %+v", st.Disks)
	}
	if len(st.Log) == 0 {
		t.Fatal("el registro debería tener líneas")
	}

	// Quitar no borra nada del disco.
	if w := lc.do("DELETE", "/admin/api/sources/"+id, nil); w.Code != 200 {
		t.Fatalf("quitar: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(lib, "Platoon (1986).mkv")); err != nil {
		t.Fatal("quitar una fuente no debe borrar archivos")
	}
	if list := decode[[]store.Source](t, lc.do("GET", "/admin/api/sources", nil)); len(list) != 0 {
		t.Fatalf("tras quitar: %+v", list)
	}
	if _, err := e.db.Title("t7-2"); err != nil {
		t.Fatal("la ficha de la semilla se conserva")
	}
}

func TestJobsEventsSSE(t *testing.T) {
	e := setup(t)
	srv := httptest.NewServer(e.a)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/admin/api/jobs/events", nil)
	req.Host = "localhost"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("SSE: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	sc := bufio.NewScanner(resp.Body)
	got := make(chan string, 10)
	go func() {
		for sc.Scan() {
			if l := sc.Text(); strings.HasPrefix(l, "data: ") {
				got <- l
			}
		}
	}()
	if l := <-got; !strings.Contains(l, `"jobs":[]`) {
		t.Fatalf("primer evento: %s", l)
	}
	e.a.jobs.enqueue("identify", "x", "Prueba", func(ctx context.Context, p *progress) error { p.Line(true, "hola"); return nil })
	timeout := time.After(5 * time.Second)
	for {
		select {
		case l := <-got:
			if strings.Contains(l, `"state":"done"`) && strings.Contains(l, "hola") {
				return
			}
		case <-timeout:
			t.Fatal("no llega el trabajo terminado por SSE")
		}
	}
}

func TestPendingMatchAndNoTokenLeak(t *testing.T) {
	e := setup(t)
	src, _ := e.db.AddSource("T7", e.dir, "auto", nil)
	e.db.UpsertTitle(&store.Title{ID: "x", Kind: "movie", Title: "Una copia mal nombrada", MatchSource: "filename", Norm: "una copia mal nombrada",
		Meta: []byte(`{"candidates":[{"tmdb":78,"title":"Blade Runner","year":"1982","score":0.7,"votes":14000}],"identifyTried":"2026-10-24T10:00:00Z"}`)}, false)
	e.db.SaveFile(&store.File{ID: "f-x", SourceID: src.ID, RelPath: "copia.mkv", Fingerprint: "x", TitleID: "x", MatchSource: "filename"})
	var all strings.Builder
	keep := func(w *httptest.ResponseRecorder) *httptest.ResponseRecorder {
		all.WriteString(w.Body.String())
		return w
	}

	mc := e.client(mac)
	SetPassword(e.db, "cine en casa 1")
	mc.login("cine en casa 1")
	p := decode[struct {
		Items []pendingOut
		TMDB  bool
	}](t, keep(mc.do("GET", "/admin/api/pending", nil)))
	if len(p.Items) != 1 || p.Items[0].Candidates[0].TMDB != 78 || p.Items[0].Files[0].Path != "copia.mkv" || !p.TMDB {
		t.Fatalf("pendientes: %+v", p)
	}
	s := decode[struct{ Items []map[string]any }](t, keep(mc.do("GET", "/admin/api/tmdb/search?q=Blade+Runner&year=1982", nil)))
	if len(s.Items) != 1 || s.Items[0]["tmdb"].(float64) != 78 || !strings.HasPrefix(s.Items[0]["poster"].(string), "https://image.tmdb.org/") {
		t.Fatalf("búsqueda: %+v", s)
	}
	if w := keep(mc.do("GET", "/admin/api/tmdb/search?q=", nil)); w.Code != http.StatusBadRequest {
		t.Fatalf("búsqueda vacía: %d", w.Code)
	}
	if w := keep(mc.do("POST", "/admin/api/titles/x/match", map[string]any{"tmdb": 78})); w.Code != 200 {
		t.Fatalf("emparejar: %d %s", w.Code, w.Body.String())
	}
	x, _ := e.db.Title("x")
	if !x.Locked || x.MatchSource != "manual" || x.Title != "Blade Runner" || x.TMDB != 78 {
		t.Fatalf("como «yourflix match»: %+v", x)
	}
	if p := decode[struct{ Items []pendingOut }](t, keep(mc.do("GET", "/admin/api/pending", nil))); len(p.Items) != 0 {
		t.Fatalf("ya no está pendiente: %+v", p)
	}
	// Un resultado de serie elegido para una ficha de película: se usa el tipo elegido
	// (los IDs de películas y series de TMDB no son el mismo espacio).
	e.db.UpsertTitle(&store.Title{ID: "y", Kind: "movie", Title: "andor s1", MatchSource: "filename", Norm: "andor s1"}, false)
	e.db.SaveFile(&store.File{ID: "f-y", SourceID: src.ID, RelPath: "andor.mkv", Fingerprint: "y", TitleID: "y", MatchSource: "filename"})
	if w := keep(mc.do("POST", "/admin/api/titles/y/match", map[string]any{"tmdb": 83867, "kind": "series"})); w.Code != 200 {
		t.Fatalf("emparejar serie: %d %s", w.Code, w.Body.String())
	}
	if y, _ := e.db.Title("y"); y.Kind != "series" || y.TMDB != 83867 || y.Title != "Andor" || !y.Locked {
		t.Fatalf("debe pasar a serie con la ficha de la serie: %+v", y)
	}
	if w := keep(mc.do("POST", "/admin/api/titles/y/match", map[string]any{"tmdb": 83867, "kind": "pelicula"})); w.Code != http.StatusBadRequest {
		t.Fatalf("tipo no válido: %d", w.Code)
	}
	if w := keep(mc.do("POST", "/admin/api/titles/nope/match", map[string]any{"tmdb": 78})); w.Code != http.StatusNotFound {
		t.Fatalf("ficha desconocida: %d", w.Code)
	}
	ts := decode[struct{ Items []map[string]any }](t, keep(mc.do("GET", "/admin/api/titles?q=blade", nil)))
	if len(ts.Items) != 1 || ts.Items[0]["locked"] != true {
		t.Fatalf("buscar fichas: %+v", ts)
	}
	j := waitJob(t, mc, decode[Job](t, keep(mc.do("POST", "/admin/api/identify", map[string]any{"retry": true}))).ID)
	if j.State != "done" {
		t.Fatalf("identify: %+v", j)
	}
	all.WriteString(strings.Join(j.Lines, "\n") + strings.Join(j.Summary, "\n"))
	keep(mc.do("GET", "/admin/api/status", nil))
	keep(mc.do("GET", "/admin/api/log?n=500", nil))
	// Con TMDB rechazando las credenciales el error tampoco lleva el token.
	e.a.cfg.TMDB.Token = token + "-x"
	keep(mc.do("GET", "/admin/api/tmdb/search?q=Alien", nil))
	e.a.cfg.TMDB.Token = token
	if strings.Contains(all.String(), token) || strings.Contains(e.logs.String(), token) {
		t.Fatal("el token de TMDB no debe salir en respuestas ni registros")
	}
	// Sin credenciales: aviso claro, sin romper.
	e.a.cfg.TMDB.Token = ""
	if w := mc.do("GET", "/admin/api/tmdb/search?q=x", nil); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("sin TMDB: %d", w.Code)
	}
}

func TestCompatSummary(t *testing.T) {
	e := setup(t)
	src, _ := e.db.AddSource("T7", e.dir, "auto", nil)
	e.db.UpsertTitle(&store.Title{ID: "t1", Kind: "movie", Title: "Con DTS", MatchSource: "seed"}, false)
	e.db.SaveFile(&store.File{ID: "f1", SourceID: src.ID, RelPath: "dts.mkv", Fingerprint: "1", Container: "mkv", TitleID: "t1",
		Video: []byte(`{"codec":"h264","width":1920,"height":1080}`), Audio: []byte(`[{"codec":"dts","channels":6}]`)})
	e.db.SaveFile(&store.File{ID: "f2", SourceID: src.ID, RelPath: "bien.mkv", Fingerprint: "2", Container: "mkv", TitleID: "t1",
		Video: []byte(`{"codec":"h264","width":1920,"height":1080}`), Audio: []byte(`[{"codec":"ac3","channels":6}]`)})
	lc := e.client(local)
	c := decode[struct {
		Counts  map[string]int
		Reasons []struct {
			Reason string
			Count  int
		}
		Items []struct {
			FileID, Status, Title string
			Reasons               []string
		}
	}](t, lc.do("GET", "/admin/api/compat", nil))
	if c.Counts["optimize"] != 1 || c.Counts["direct"] != 1 || len(c.Items) != 1 || c.Items[0].FileID != "f1" || c.Items[0].Title != "Con DTS" {
		t.Fatalf("compatibilidad: %+v", c)
	}
	if len(c.Reasons) != 1 || c.Reasons[0].Reason != "sin audio compatible (sin sonido)" {
		t.Fatalf("motivos: %+v", c.Reasons)
	}
}

func TestStaticAndRing(t *testing.T) {
	e := setup(t)
	c := e.client(tv) // la web estática no tiene datos: se sirve a la LAN
	w := c.do("GET", "/admin/", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "admin.js") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatalf("index: %d %s", w.Code, w.Header())
	}
	for _, f := range []string{"admin.js", "admin.css", "logo.png"} {
		if w := c.do("GET", "/admin/"+f, nil); w.Code != 200 {
			t.Fatalf("%s: %d", f, w.Code)
		}
	}
	if w := c.do("GET", "/admin/../go.mod", nil); w.Code == 200 {
		t.Fatal("no debe salir de la carpeta admin/")
	}
	if w := c.do("GET", "/admin/api/nada", nil); w.Code != 404 {
		t.Fatalf("API desconocida: %d", w.Code)
	}

	r := NewRing(3, "secreto123")
	io.WriteString(r, "uno\ndos con secreto123 dentro\n")
	io.WriteString(r, "tr")
	io.WriteString(r, "es\ncuatro\n")
	got := strings.Join(r.Last(0), "|")
	if got != "dos con [oculto] dentro|tres|cuatro" {
		t.Fatalf("anillo: %q", got)
	}
}

// Arreglos de la revisión del PR (Sonnet, 24-10-2026).
func TestReviewHardening(t *testing.T) {
	e := setup(t)
	if err := SetPassword(e.db, "contraseña buena"); err != nil {
		t.Fatal(err)
	}
	// 20 intentos a la vez desde la misma IP: el límite se cuenta antes de comprobar la contraseña.
	codes := make(chan int, 20)
	for i := 0; i < 20; i++ {
		go func() { codes <- e.client("192.168.1.77:1").login("mala") }()
	}
	tried := 0
	for i := 0; i < 20; i++ {
		if <-codes != http.StatusTooManyRequests {
			tried++
		}
	}
	if tried > maxFails {
		t.Fatalf("%d intentos comprobados en paralelo (máximo %d)", tried, maxFails)
	}

	// Un proxy local con Host: localhost no entra sin contraseña.
	r := httptest.NewRequest("GET", "/admin/api/sources", nil)
	r.RemoteAddr, r.Host = local, "localhost:8787"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()
	e.a.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("detrás de un proxy: %d", w.Code)
	}

	// JavaScript con su tipo aunque el registro de Windows diga otra cosa.
	if w := e.client(tv).do("GET", "/admin/admin.js", nil); !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("admin.js: %q", w.Header().Get("Content-Type"))
	}

	// Quitar una fuente durante un escaneo responde enseguida con 409 en vez de esperar.
	src, _ := e.db.AddSource("T7", e.dir, "auto", nil)
	mu := &sync.Mutex{}
	e.a.cfg.Lock = mu
	mu.Lock()
	c := e.client(local)
	c.session()
	if w := c.do("DELETE", "/admin/api/sources/"+src.ID, nil); w.Code != http.StatusConflict {
		t.Fatalf("quitar durante un escaneo: %d", w.Code)
	}
	mu.Unlock()
	if w := c.do("DELETE", "/admin/api/sources/"+src.ID, nil); w.Code != http.StatusOK {
		t.Fatalf("quitar sin escaneo: %d %s", w.Code, w.Body)
	}

	if runtime.GOOS == "windows" {
		if w := c.do("GET", "/admin/api/browse?path="+url.QueryEscape(`\\equipo\carpeta`), nil); w.Code != http.StatusBadRequest {
			t.Fatalf("carpeta de red: %d", w.Code)
		}
	}

	// Reiniciar: solo con sesión de admin y CSRF; llama a Restart.
	if w := c.do("POST", "/admin/api/restart", map[string]any{}); w.Code != http.StatusNotImplemented {
		t.Fatalf("reiniciar sin Restart: %d", w.Code)
	}
	restarted := make(chan bool, 1)
	e.a.cfg.Restart = func() { restarted <- true }
	if w := e.client(tv).do("POST", "/admin/api/restart", map[string]any{}); w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("reiniciar desde la TV: %d", w.Code)
	}
	if w := c.do("POST", "/admin/api/restart", map[string]any{}); w.Code != http.StatusAccepted {
		t.Fatalf("reiniciar: %d", w.Code)
	}
	select {
	case <-restarted:
	case <-time.After(3 * time.Second):
		t.Fatal("no se llamó a Restart")
	}
}

func TestPasswordHash(t *testing.T) {
	h, err := hashPassword("cine en casa")
	if err != nil || !checkPassword(h, "cine en casa") || checkPassword(h, "cine en casa ") || checkPassword("basura", "x") {
		t.Fatalf("hash: %v %s", err, h)
	}
	h2, _ := hashPassword("cine en casa")
	if h == h2 {
		t.Fatal("cada hash lleva su sal")
	}
}
