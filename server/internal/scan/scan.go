// Package scan recorre las fuentes, mide los vídeos y los empareja con su ficha.
package scan

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/parse"
	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

var VideoExt = map[string]bool{".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".ts": true, ".m2ts": true, ".mpg": true, ".mpeg": true, ".wmv": true, ".webm": true}

var skipDirs = map[string]bool{"$RECYCLE.BIN": true, "System Volume Information": true, ".Trashes": true, ".Spotlight-V100": true, ".fseventsd": true, "@eaDir": true}

type Options struct {
	FFprobe string
	Workers int
	Log     func(format string, args ...any)
}

type Report struct {
	Source           string `json:"source"`
	Seen             int    `json:"seen"`
	New              int    `json:"new"`
	Changed          int    `json:"changed"`
	Moved            int    `json:"moved"`
	Missing          int    `json:"missing"`
	ProbeErrors      int    `json:"probeErrors"`
	MatchedSeedBytes int    `json:"matchedSeedBytes"`
	MatchedSeedTitle int    `json:"matchedSeedTitle"`
	FromFilename     int    `json:"fromFilename"`
	Offline          bool   `json:"offline,omitempty"`
}

// Source escanea una fuente. Una fuente pausada no se toca; una cuya carpeta no
// existe (disco desconectado) pasa a «offline» y sus archivos se marcan ausentes.
func Source(ctx context.Context, db *store.DB, src *store.Source, opt Options) (*Report, error) {
	if opt.Workers <= 0 {
		opt.Workers = 4
	}
	if opt.Log == nil {
		opt.Log = func(string, ...any) {}
	}
	rep := &Report{Source: src.Label}
	if src.State == "paused" {
		return rep, nil
	}
	known, err := db.FilesOfSource(src.ID)
	if err != nil {
		return nil, err
	}
	byPath := map[string]*store.File{}
	for _, f := range known {
		byPath[f.RelPath] = f
	}

	if st, err := statTimeout(src.Path, 10*time.Second); err != nil || !st.IsDir() {
		if err != nil {
			opt.Log("  «%s» no accesible: %v", src.Label, err)
		}
		rep.Offline = true
		for _, f := range known {
			if f.MissingSince == "" {
				f.MissingSince = store.Now()
				db.SaveFile(f)
				rep.Missing++
			}
		}
		return rep, db.SetSourceState(src.ID, "offline")
	}
	if src.State == "offline" {
		db.SetSourceState(src.ID, "active")
	}

	type job struct {
		rel     string
		info    fs.FileInfo
		old     *store.File
		rematch bool // sin cambios en disco: solo se vuelve a emparejar
	}
	var jobs []job
	seen := map[string]bool{}
	err = filepath.WalkDir(src.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			opt.Log("  no se puede leer %s: %v", p, err)
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != src.Path && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !VideoExt[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		rel, _ := filepath.Rel(src.Path, p)
		rel = filepath.ToSlash(rel)
		if excluded(rel, src.Excludes) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		seen[rel] = true
		rep.Seen++
		old := byPath[rel]
		if old != nil && old.Size == info.Size() && old.Mtime == info.ModTime().Unix() && old.MissingSince == "" && old.TitleID != "" {
			if old.MatchSource == "filename" {
				jobs = append(jobs, job{rel, info, old, true}) // quizá ya hay ficha (semilla o TMDB)
			}
			return nil
		}
		jobs = append(jobs, job{rel, info, old, false})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Huella + ffprobe en paralelo; escritura en la base de datos en serie.
	type result struct {
		job
		fp  string
		res *probe.Result
		err error
	}
	in := make(chan job)
	out := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < opt.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range in {
				abs := filepath.Join(src.Path, filepath.FromSlash(j.rel))
				r := result{job: j}
				if j.rematch {
					r.fp = j.old.Fingerprint
					out <- r
					continue
				}
				r.fp, r.err = Fingerprint(abs, j.info.Size())
				if r.err == nil {
					r.res, r.err = probe.Run(ctx, opt.FFprobe, abs)
				}
				out <- r
			}
		}()
	}
	go func() {
		for _, j := range jobs {
			in <- j
		}
		close(in)
		wg.Wait()
		close(out)
	}()

	missingByFP := map[string]*store.File{}
	for _, f := range known {
		if !seen[f.RelPath] {
			missingByFP[f.Fingerprint] = f
		}
	}
	m := &matcher{db: db, kind: src.Kind}
	for r := range out {
		f := r.old
		switch {
		case f != nil && f.Fingerprint == r.fp:
			// mismo contenido (solo cambió la fecha, o volvió el disco)
		case f != nil:
			rep.Changed++
		case missingByFP[r.fp] != nil:
			f = missingByFP[r.fp] // movido o renombrado: conserva ID y progreso
			delete(missingByFP, r.fp)
			rep.Moved++
		default:
			f = &store.File{ID: store.NewID(), SourceID: src.ID}
			rep.New++
		}
		f.RelPath, f.Size, f.Mtime, f.Fingerprint, f.MissingSince = r.rel, r.info.Size(), r.info.ModTime().Unix(), r.fp, ""
		before := f.TitleID
		switch {
		case r.err != nil:
			rep.ProbeErrors++
			opt.Log("  %s: %v", r.rel, r.err)
		case r.res != nil:
			applyProbe(f, r.res)
		}
		if f.MatchSource != "manual" {
			switch m.match(f, parse.File(r.rel, src.Kind)) {
			case "seed-bytes":
				rep.MatchedSeedBytes++
			case "seed-title":
				rep.MatchedSeedTitle++
			case "filename":
				rep.FromFilename++
			}
		}
		if err := db.SaveFile(f); err != nil {
			return nil, fmt.Errorf("%s: %w", r.rel, err)
		}
		if !r.rematch || f.TitleID != before {
			opt.Log("  %s → %s", r.rel, f.MatchSource)
		}
	}

	// Los movidos ya tienen su nueva ruta (mismo puntero), así que aquí solo quedan los que faltan.
	for _, f := range known {
		if !seen[f.RelPath] && f.MissingSince == "" {
			f.MissingSince = store.Now()
			db.SaveFile(f)
			rep.Missing++
		}
	}
	db.DeleteOrphanTitles()
	return rep, db.MarkScanned(src.ID)
}

func applyProbe(f *store.File, r *probe.Result) {
	f.Duration, f.Container, f.Bitrate = r.Duration, r.Container, r.Bitrate
	f.Video, _ = json.Marshal(r.Video)
	f.Audio, _ = json.Marshal(nonNil(r.Audio))
	f.Subs, _ = json.Marshal(nonNil(r.Subs))
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func excluded(rel string, patterns []string) bool {
	base := filepath.Base(rel)
	for _, p := range patterns {
		p = filepath.ToSlash(p)
		if strings.HasSuffix(p, "/") && (strings.HasPrefix(rel, p) || strings.Contains(rel, "/"+p)) {
			return true
		}
		if ok, _ := filepath.Match(strings.ToLower(p), strings.ToLower(base)); ok {
			return true
		}
	}
	return false
}

// statTimeout evita que una fuente bloquee el escaneo de las demás: un disco USB que no
// despierta, una unidad de red caída o macOS esperando permiso de privacidad (Descargas,
// Documentos, volúmenes externos) dejan os.Stat esperando indefinidamente.
func statTimeout(path string, d time.Duration) (fs.FileInfo, error) {
	type res struct {
		fi  fs.FileInfo
		err error
	}
	ch := make(chan res, 1)
	go func() {
		fi, err := os.Stat(path)
		if err == nil && fi.IsDir() {
			// macOS deja hacer stat de ~/Downloads pero bloquea leer su contenido
			// hasta que se concede el permiso: se comprueba leyendo una entrada.
			var d *os.File
			if d, err = os.Open(path); err == nil {
				_, err = d.Readdirnames(1)
				d.Close()
				if err == io.EOF {
					err = nil
				}
			}
		}
		ch <- res{fi, err}
	}()
	select {
	case r := <-ch:
		return r.fi, r.err
	case <-time.After(d):
		return nil, fmt.Errorf("sin respuesta en %s (¿disco dormido o permiso del sistema pendiente?)", d)
	}
}

const chunk = 1 << 20

// Fingerprint identifica el contenido: tamaño + SHA-1 del primer y el último MiB.
// Sobrevive a renombrados, a mover de carpeta o de disco y a cambios de letra de unidad.
func Fingerprint(path string, size int64) (string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	h := sha1.New()
	binary.Write(h, binary.LittleEndian, size)
	buf := make([]byte, chunk)
	n, err := io.ReadFull(fh, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	h.Write(buf[:n])
	if size > 2*chunk {
		if _, err := fh.Seek(size-chunk, io.SeekStart); err != nil {
			return "", err
		}
		n, err = io.ReadFull(fh, buf)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return "", err
		}
		h.Write(buf[:n])
	}
	return hex.EncodeToString(h.Sum(nil))[:24], nil
}
