package admin

import (
	"strings"
	"sync"
)

// Ring guarda las últimas líneas del registro del servidor para la página de estado.
// Se engancha a log con io.MultiWriter(os.Stderr, ring). Borra cualquier secreto que se
// le indique (token de TMDB) por si alguna línea lo arrastrara.
type Ring struct {
	mu      sync.Mutex
	lines   []string
	max     int
	partial strings.Builder
	redact  []string
}

func NewRing(max int, secrets ...string) *Ring {
	r := &Ring{max: max}
	for _, s := range secrets {
		if len(s) >= 6 {
			r.redact = append(r.redact, s)
		}
	}
	return r
}

func (r *Ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.partial.Write(p)
	buf := r.partial.String()
	for {
		i := strings.IndexByte(buf, '\n')
		if i < 0 {
			break
		}
		r.add(buf[:i])
		buf = buf[i+1:]
	}
	r.partial.Reset()
	if len(buf) > 4096 {
		r.add(buf) // línea sin fin: no crecer sin límite
		buf = ""
	}
	r.partial.WriteString(buf)
	return len(p), nil
}

func (r *Ring) add(line string) {
	line = strings.TrimRight(line, "\r")
	for _, s := range r.redact {
		line = strings.ReplaceAll(line, s, "[oculto]")
	}
	r.lines = append(r.lines, line)
	if over := len(r.lines) - r.max; over > 0 {
		r.lines = append(r.lines[:0:0], r.lines[over:]...)
	}
}

// Last devuelve las n últimas líneas (todas si n <= 0).
func (r *Ring) Last(n int) []string {
	if r == nil {
		return []string{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || n > len(r.lines) {
		n = len(r.lines)
	}
	return append([]string{}, r.lines[len(r.lines)-n:]...)
}
