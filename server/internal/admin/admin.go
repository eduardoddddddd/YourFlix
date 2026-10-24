// Package admin es la web de administración de YourFlix (fase 5), montada en /admin/.
//
// Seguridad (PLAN.md §3.3): solo LAN; contraseña con hash (PBKDF2) y cookie de sesión;
// desde localhost se entra sin contraseña; las peticiones que cambian algo exigen JSON y la
// cabecera X-CSRF-Token de la sesión. El token de TMDB nunca sale en las respuestas.
// La API de administración sí devuelve rutas (fuentes, explorador de carpetas), pero
// solo a una sesión de administración o a localhost, nunca a la TV ni a la web pública.
package admin

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
	"github.com/eduardoddddddd/YourFlix/server/internal/tmdb"
)

type Config struct {
	DB      *store.DB
	TMDB    *tmdb.Client // puede no tener credenciales: entonces no se identifica
	Profile *compat.Profile
	Version string
	DataDir string
	FFprobe string
	FFmpeg  string
	WebDir  string // carpeta admin/ con index.html; vacía = página de aviso
	Log     *log.Logger
	Ring    *Ring       // últimas líneas del registro (puede ser nil)
	Lock    sync.Locker // compartido con el re-escaneo periódico de serve (puede ser nil)
	Started time.Time
	// Roots sustituye a las unidades/raíz del explorador (tests).
	Roots func() []Place
}

type Admin struct {
	cfg      Config
	mux      *http.ServeMux
	sessions *sessions
	throttle *throttle
	hashMu   sync.Mutex // PBKDF2 de uno en uno: una ráfaga de intentos no ahoga el streaming
	jobs     *runner
	ctx      context.Context
	cancel   context.CancelFunc
	tools    toolCache
}

func New(cfg Config) (*Admin, error) {
	if cfg.DB == nil {
		return nil, errors.New("admin: falta la base de datos")
	}
	if err := cfg.DB.InitAdmin(); err != nil {
		return nil, err
	}
	if cfg.Log == nil {
		cfg.Log = log.New(io.Discard, "", 0)
	}
	if cfg.Started.IsZero() {
		cfg.Started = time.Now()
	}
	if cfg.Roots == nil {
		cfg.Roots = systemRoots
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Admin{cfg: cfg, sessions: &sessions{byID: map[string]*session{}}, throttle: &throttle{byIP: map[string]*attempts{}}, ctx: ctx, cancel: cancel}
	a.jobs = newRunner(ctx, cfg.Lock)
	a.routes()
	return a, nil
}

// Close para los trabajos en marcha.
func (a *Admin) Close() { a.cancel() }

func (a *Admin) routes() {
	m := http.NewServeMux()
	// Sin sesión
	m.HandleFunc("GET /admin/api/session", a.sessionInfo)
	m.HandleFunc("POST /admin/api/login", a.login)
	m.HandleFunc("POST /admin/api/logout", a.logout)
	m.HandleFunc("POST /admin/api/password", a.setPassword) // comprueba permisos dentro
	// Con sesión de administración o desde localhost
	auth := func(pattern string, h http.HandlerFunc) { m.Handle(pattern, a.requireAdmin(h)) }
	auth("GET /admin/api/sources", a.sourcesList)
	auth("POST /admin/api/sources", a.sourceAdd)
	auth("PATCH /admin/api/sources/{id}", a.sourceUpdate)
	auth("DELETE /admin/api/sources/{id}", a.sourceRemove)
	auth("POST /admin/api/sources/{id}/pause", a.sourceState("paused"))
	auth("POST /admin/api/sources/{id}/resume", a.sourceState("active"))
	auth("POST /admin/api/sources/{id}/scan", a.sourceScan)
	auth("POST /admin/api/scan", a.scanAll)
	auth("GET /admin/api/browse", a.browse)
	auth("GET /admin/api/jobs", a.jobsList)
	auth("GET /admin/api/jobs/events", a.jobsEvents)
	auth("GET /admin/api/jobs/{id}", a.jobGet)
	auth("GET /admin/api/pending", a.pending)
	auth("GET /admin/api/titles", a.titles)
	auth("GET /admin/api/tmdb/search", a.tmdbSearch)
	auth("POST /admin/api/titles/{id}/match", a.match)
	auth("POST /admin/api/identify", a.identify)
	auth("GET /admin/api/status", a.status)
	auth("GET /admin/api/compat", a.compatSummary)
	auth("GET /admin/api/log", a.logLines)
	m.HandleFunc("/admin/api/", func(w http.ResponseWriter, r *http.Request) { writeError(w, http.StatusNotFound, "no existe") })
	// Web estática (sin datos: el acceso lo controla la API)
	m.HandleFunc("/admin/", a.static)
	a.mux = m
}

// ServeHTTP: solo LAN, cabeceras de seguridad y CSRF en lo que cambia algo.
func (a *Admin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := remoteIP(r)
	if ip == nil || !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		http.Error(w, "solo red local", http.StatusForbidden)
		return
	}
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https://image.tmdb.org; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if strings.HasPrefix(r.URL.Path, "/admin/api/") {
		h.Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if err := a.checkCSRF(r); err != nil {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
		}
	}
	a.mux.ServeHTTP(w, r)
}

// checkCSRF: JSON obligatorio (un formulario de otra web no puede enviarlo sin permiso
// CORS, que aquí no se da), Origin del mismo sitio si viene, y token de la sesión salvo
// para entrar (aún no hay sesión).
func (a *Admin) checkCSRF(r *http.Request) error {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return errors.New("se espera Content-Type: application/json")
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host && o != "https://"+r.Host {
		return errors.New("origen no permitido")
	}
	if r.URL.Path == "/admin/api/login" {
		return nil
	}
	s := a.currentSession(r)
	if s == nil || !constEq(r.Header.Get("X-CSRF-Token"), s.csrf) {
		return errCSRF
	}
	return nil
}

func constEq(a, b string) bool {
	return a != "" && len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// currentSession: la sesión de la cookie, si sigue valiendo para esta petición.
func (a *Admin) currentSession(r *http.Request) *session {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	s := a.sessions.get(c.Value)
	if s == nil {
		return nil
	}
	if s.local {
		if !isLocal(r) {
			return nil // una sesión de localhost no vale desde otro equipo
		}
		return s
	}
	hash, _, _ := a.cfg.DB.Setting(passwordKey)
	if hash == "" || pwTag(hash) != s.pwTag {
		a.sessions.drop(s.id) // contraseña cambiada (web o CLI) o borrada
		return nil
	}
	return s
}

func (a *Admin) authorized(r *http.Request) bool {
	return isLocal(r) || a.currentSession(r) != nil
}

func (a *Admin) requireAdmin(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.authorized(r) {
			writeError(w, http.StatusUnauthorized, "inicia sesión de administración")
			return
		}
		h(w, r)
	})
}

func setCookie(w http.ResponseWriter, s *session) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.id, Path: "/admin/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTTL.Seconds())})
}

// sessionInfo: lo que la web necesita para decidir qué pantalla enseña. Desde localhost
// crea una sesión sin contraseña (para tener token CSRF).
func (a *Admin) sessionInfo(w http.ResponseWriter, r *http.Request) {
	local := isLocal(r)
	s := a.currentSession(r)
	if s == nil && local {
		s = a.sessions.create(true, "")
		setCookie(w, s)
	}
	out := map[string]any{"authenticated": s != nil, "local": local, "passwordSet": HasPassword(a.cfg.DB), "version": a.cfg.Version}
	if s != nil {
		out["csrf"] = s.csrf
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *Admin) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	ip := remoteIP(r).String()
	hash, _, _ := a.cfg.DB.Setting(passwordKey)
	if hash == "" {
		writeError(w, http.StatusConflict, "aún no hay contraseña: defínela desde el propio servidor (http://localhost:8787/admin/) o con «yourflix admin password»")
		return
	}
	if a.tooMany(w, ip) {
		return
	}
	if !a.checkPw(hash, in.Password) {
		a.cfg.Log.Printf("admin: contraseña incorrecta desde %s", ip)
		writeError(w, http.StatusUnauthorized, "contraseña incorrecta")
		return
	}
	a.throttle.ok(ip)
	s := a.sessions.create(false, pwTag(hash))
	setCookie(w, s)
	a.cfg.Log.Printf("admin: sesión iniciada desde %s", ip)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "csrf": s.csrf})
}

// tooMany cuenta un intento de contraseña y responde 429 si esa IP está bloqueada.
func (a *Admin) tooMany(w http.ResponseWriter, ip string) bool {
	d := a.throttle.attempt(ip)
	if d <= 0 {
		return false
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())+1))
	writeError(w, http.StatusTooManyRequests, "demasiados intentos: espera un minuto")
	return true
}

func (a *Admin) checkPw(hash, pw string) bool {
	a.hashMu.Lock()
	defer a.hashMu.Unlock()
	return checkPassword(hash, pw)
}

func (a *Admin) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		a.sessions.drop(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/admin/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

// setPassword: la primera contraseña solo desde localhost (nadie de la LAN puede
// adueñarse de la admin); para cambiarla desde otro equipo hay que dar la actual.
func (a *Admin) setPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	local := isLocal(r)
	hash, _, _ := a.cfg.DB.Setting(passwordKey)
	cur := a.currentSession(r) // antes de cambiarla: después su huella ya no coincide
	switch {
	case hash == "" && !local:
		writeError(w, http.StatusForbidden, "la primera contraseña se define desde el propio servidor (localhost) o con «yourflix admin password»")
		return
	case !local && cur == nil:
		writeError(w, http.StatusUnauthorized, "inicia sesión de administración")
		return
	case hash != "" && !local:
		if a.tooMany(w, remoteIP(r).String()) {
			return
		}
		if !a.checkPw(hash, in.Current) {
			writeError(w, http.StatusUnauthorized, "la contraseña actual no es correcta")
			return
		}
	}
	if err := SetPassword(a.cfg.DB, in.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	newHash, _, _ := a.cfg.DB.Setting(passwordKey)
	keep := ""
	if cur != nil {
		keep = cur.id
		if !cur.local {
			a.sessions.retag(cur.id, pwTag(newHash)) // quien la cambia sigue dentro
		}
	}
	a.sessions.dropRemote(keep)
	a.cfg.Log.Printf("admin: contraseña de administración cambiada desde %s", remoteIP(r))
	w.WriteHeader(http.StatusNoContent)
}

// static sirve admin/ (index.html, admin.js, admin.css, logo.png). http.Dir no deja
// salir de la carpeta.
func (a *Admin) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/admin/")
	if a.cfg.WebDir == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>YourFlix · administración</title><body style="background:#090b10;color:#f4f1e8;font:18px Arial,sans-serif;padding:40px"><h1 style="font-family:Georgia,serif">Falta la web de administración</h1><p>No encuentro la carpeta <code>admin/</code> junto al ejecutable. Indícala con la variable <code>YOURFLIX_ADMIN_WEB</code> y reinicia el servidor.</p>`)
		return
	}
	if rel == "" || strings.HasSuffix(rel, "/") {
		rel += "index.html"
	}
	if strings.HasPrefix(filepath.Base(rel), ".") {
		http.NotFound(w, r)
		return
	}
	f, err := http.Dir(a.cfg.WebDir).Open("/" + rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	// Tipo fijo por extensión: en Windows, Go lo saca del registro y algunos equipos dicen
	// text/plain para .js, que con nosniff y la CSP dejaría la admin sin JavaScript.
	if ct, ok := staticTypes[strings.ToLower(filepath.Ext(st.Name()))]; ok {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

var staticTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8",
	".png": "image/png", ".svg": "image/svg+xml", ".ico": "image/x-icon", ".json": "application/json",
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return false
	}
	return true
}

// fail: errores internos al registro, mensaje genérico al cliente salvo «no encontrado».
func (a *Admin) fail(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no encontrado")
		return
	}
	a.cfg.Log.Printf("admin: %v", err)
	writeError(w, http.StatusInternalServerError, "error interno (detalle en el registro)")
}

// FindWebDir busca la carpeta admin/ en las rutas dadas (la primera con index.html).
func FindWebDir(candidates ...string) string {
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(filepath.Join(c, "index.html")); err == nil && !st.IsDir() {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}
