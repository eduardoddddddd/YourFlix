package admin

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/scan"
)

// Explorador de carpetas del servidor: el navegador no puede dar rutas de otro equipo,
// así que el servidor lista sus unidades (Windows) o la raíz (macOS/Linux) y carpetas.
// Solo lista carpetas (y cuenta vídeos); no lee ni devuelve el contenido de ningún archivo.

type Place struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// systemRoots: unidades A:…Z: que responden en Windows; «/» y atajos en el resto.
func systemRoots() []Place {
	if runtime.GOOS == "windows" {
		type res struct {
			i  int
			ok bool
		}
		ch := make(chan res, 26)
		for i := 0; i < 26; i++ {
			go func(i int) {
				_, err := os.Stat(string(rune('A'+i)) + `:\`)
				ch <- res{i, err == nil}
			}(i)
		}
		found := make([]bool, 26)
		deadline := time.After(3 * time.Second) // una unidad dormida no bloquea la lista
	wait:
		for n := 0; n < 26; n++ {
			select {
			case r := <-ch:
				found[r.i] = r.ok
			case <-deadline:
				break wait
			}
		}
		var out []Place
		for i, ok := range found {
			if ok {
				d := string(rune('A' + i))
				out = append(out, Place{Name: d + ":", Path: d + `:\`})
			}
		}
		return out
	}
	out := []Place{{Name: "/", Path: "/"}}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, Place{Name: "Inicio (" + filepath.Base(home) + ")", Path: home})
	}
	for _, p := range []string{"/Volumes", "/media", "/mnt"} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, Place{Name: p, Path: p})
		}
	}
	return out
}

var hiddenDirs = map[string]bool{"$RECYCLE.BIN": true, "System Volume Information": true, "@eaDir": true, "lost+found": true}

type listing struct {
	dirs   []Place
	videos int
	more   bool
}

const maxEntries = 3000

const errNetworkPath = "las carpetas de red (\\\\equipo\\carpeta) no se admiten: copia los vídeos a un disco del servidor"

// networkPath: rutas UNC o de dispositivo en Windows. Abrirlas haría que Windows se
// autentique contra ese equipo y puede colgarse si no responde.
func networkPath(p string) bool {
	return runtime.GOOS == "windows" && (strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, `//`))
}

func readDirTimeout(path string, d time.Duration) (*listing, error) {
	type res struct {
		l   *listing
		err error
	}
	ch := make(chan res, 1)
	go func() {
		entries, err := os.ReadDir(path)
		if err != nil {
			ch <- res{nil, err}
			return
		}
		l := &listing{dirs: []Place{}}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") || hiddenDirs[name] {
				continue
			}
			isDir := e.IsDir()
			if e.Type()&os.ModeSymlink != 0 { // enlaces y uniones de Windows: seguir solo si llevan a una carpeta
				if st, err := os.Stat(filepath.Join(path, name)); err == nil && st.IsDir() {
					isDir = true
				}
			}
			switch {
			case isDir:
				if len(l.dirs) >= maxEntries {
					l.more = true
					continue
				}
				l.dirs = append(l.dirs, Place{Name: name, Path: filepath.Join(path, name)})
			case scan.VideoExt[strings.ToLower(filepath.Ext(name))]:
				l.videos++
			}
		}
		sort.Slice(l.dirs, func(i, j int) bool { return strings.ToLower(l.dirs[i].Name) < strings.ToLower(l.dirs[j].Name) })
		ch <- res{l, nil}
	}()
	select {
	case r := <-ch:
		return r.l, r.err
	case <-time.After(d):
		return nil, errors.New("la carpeta no responde (¿disco dormido o permiso del sistema pendiente?)")
	}
}

// browse: GET /admin/api/browse?path=  (vacío = unidades / raíz)
func (a *Admin) browse(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimSpace(r.URL.Query().Get("path"))
	roots := a.cfg.Roots()
	if roots == nil {
		roots = []Place{}
	}
	out := map[string]any{"roots": roots, "sep": string(filepath.Separator)}
	if p == "" {
		out["path"], out["parent"], out["dirs"], out["videos"] = "", "", roots, 0
		writeJSON(w, http.StatusOK, out)
		return
	}
	if runtime.GOOS == "windows" && len(p) == 2 && p[1] == ':' {
		p += `\` // «C:» a secas es la carpeta actual de esa unidad, no su raíz
	}
	if !filepath.IsAbs(p) {
		writeError(w, http.StatusBadRequest, "la ruta debe ser absoluta")
		return
	}
	if networkPath(p) {
		writeError(w, http.StatusBadRequest, errNetworkPath)
		return
	}
	p = filepath.Clean(p)
	l, err := readDirTimeout(p, 10*time.Second)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, os.ErrNotExist) {
			code = http.StatusNotFound
		}
		writeError(w, code, "no se puede abrir "+p+": "+errText(err))
		return
	}
	parent := filepath.Dir(p)
	if parent == p {
		parent = "" // raíz de la unidad: volver a la lista de unidades
	}
	out["path"], out["parent"], out["dirs"], out["videos"], out["truncated"] = p, parent, l.dirs, l.videos, l.more
	writeJSON(w, http.StatusOK, out)
}

// errText quita la ruta repetida de los *PathError («open X: permiso denegado» → «permiso denegado»).
func errText(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
