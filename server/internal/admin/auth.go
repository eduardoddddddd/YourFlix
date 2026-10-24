package admin

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

// Contraseña de administración: PBKDF2-SHA256 (biblioteca estándar, sin dependencias
// nuevas) guardada en settings.adminPassword como «pbkdf2-sha256$iter$sal$hash».

const passwordKey = "adminPassword"

// Iterations es variable solo para que los tests no tarden; en servicio, 600 000 (OWASP 2023).
var Iterations = 600_000

const MinPassword = 8

func hashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, pw, salt, Iterations, 32)
	if err != nil {
		return "", err
	}
	b64 := base64.RawStdEncoding
	return "pbkdf2-sha256$" + strconv.Itoa(Iterations) + "$" + b64.EncodeToString(salt) + "$" + b64.EncodeToString(key), nil
}

func checkPassword(stored, pw string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 {
		return false
	}
	b64 := base64.RawStdEncoding
	salt, err1 := b64.DecodeString(parts[2])
	want, err2 := b64.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// SetPassword guarda (o cambia) la contraseña de administración. Lo usan la CLI
// («yourflix admin password») y la propia web.
func SetPassword(db *store.DB, pw string) error {
	if len([]rune(pw)) < MinPassword {
		return fmt.Errorf("la contraseña debe tener al menos %d caracteres", MinPassword)
	}
	h, err := hashPassword(pw)
	if err != nil {
		return err
	}
	return db.SetSetting(passwordKey, h)
}

// HasPassword dice si ya hay contraseña de administración.
func HasPassword(db *store.DB) bool {
	v, ok, _ := db.Setting(passwordKey)
	return ok && v != ""
}

// Sesiones en memoria: un reinicio del servidor obliga a volver a entrar.
const (
	cookieName  = "yf_admin"
	sessionTTL  = 7 * 24 * time.Hour
	maxSessions = 200
)

type session struct {
	id, csrf string
	local    bool   // creada desde el propio equipo, sin contraseña
	pwTag    string // huella del hash vigente al entrar: si cambia la contraseña, la sesión caduca
	expires  time.Time
}

type sessions struct {
	mu   sync.Mutex
	byID map[string]*session
}

func randomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func pwTag(hash string) string {
	if hash == "" {
		return ""
	}
	s := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(s[:8])
}

func (s *sessions) create(local bool, tag string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, x := range s.byID {
		if now.After(x.expires) {
			delete(s.byID, id)
		}
	}
	if len(s.byID) >= maxSessions { // tope de memoria: se olvida la que antes caduque
		var old *session
		for _, x := range s.byID {
			if old == nil || x.expires.Before(old.expires) {
				old = x
			}
		}
		delete(s.byID, old.id)
	}
	x := &session{id: randomToken(), csrf: randomToken(), local: local, pwTag: tag, expires: now.Add(sessionTTL)}
	s.byID[x.id] = x
	c := *x
	return &c
}

// get devuelve una copia de la sesión (nil si no existe o caducó).
func (s *sessions) get(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	x := s.byID[id]
	if x == nil {
		return nil
	}
	if time.Now().After(x.expires) {
		delete(s.byID, id)
		return nil
	}
	c := *x
	return &c
}

func (s *sessions) retag(id, tag string) {
	s.mu.Lock()
	if x := s.byID[id]; x != nil {
		x.pwTag = tag
	}
	s.mu.Unlock()
}

func (s *sessions) drop(id string) {
	s.mu.Lock()
	delete(s.byID, id)
	s.mu.Unlock()
}

// dropRemote cierra las sesiones con contraseña salvo keep (tras cambiarla).
func (s *sessions) dropRemote(keep string) {
	s.mu.Lock()
	for id, x := range s.byID {
		if !x.local && id != keep {
			delete(s.byID, id)
		}
	}
	s.mu.Unlock()
}

// Límite de intentos de contraseña por IP: 5 fallos seguidos bloquean un minuto.
type throttle struct {
	mu   sync.Mutex
	byIP map[string]*attempts
}

type attempts struct {
	fails int
	until time.Time
}

const maxFails = 5

func (t *throttle) blocked(ip string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if a := t.byIP[ip]; a != nil && time.Now().Before(a.until) {
		return time.Until(a.until)
	}
	return 0
}

func (t *throttle) fail(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	a := t.byIP[ip]
	if a == nil {
		a = &attempts{}
		t.byIP[ip] = a
	}
	a.fails++
	if a.fails >= maxFails {
		a.fails, a.until = 0, time.Now().Add(time.Minute)
	}
}

func (t *throttle) ok(ip string) {
	t.mu.Lock()
	delete(t.byIP, ip)
	t.mu.Unlock()
}

func remoteIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

// isLocal: la petición sale del propio equipo y va dirigida a «localhost». Comprobar el
// Host evita que una web cualquiera abierta en el Ally entre sin contraseña con un
// dominio que resuelva a 127.0.0.1 (DNS rebinding).
func isLocal(r *http.Request) bool {
	ip := remoteIP(r)
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	hip := net.ParseIP(host)
	return hip != nil && hip.IsLoopback()
}

var errCSRF = errors.New("falta la protección CSRF: recarga la página")
