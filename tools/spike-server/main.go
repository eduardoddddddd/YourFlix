// Servidor mínimo de la fase 0: sirve los vídeos de una carpeta con Range
// (http.ServeContent), registra cada petición que hace AVPlay y recibe las
// trazas de la app de prueba. Solo para la matriz de docs/PRUEBAS-TV.md.
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type track struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Codec    string `json:"codec"`
	Profile  string `json:"profile,omitempty"`
	PixFmt   string `json:"pixFmt,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Channels int    `json:"channels,omitempty"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
}

type mediaFile struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Duration float64 `json:"duration"`
	Bitrate  int64   `json:"bitrate"`
	Tracks   []track `json:"tracks"`
	URL      string  `json:"url"`
	path     string
}

var videoExt = map[string]string{
	".mkv": "video/x-matroska",
	".mp4": "video/mp4",
	".m4v": "video/mp4",
	".avi": "video/x-msvideo",
	".ts":  "video/mp2t",
}

func probe(ffprobe, path string) (float64, int64, []track) {
	out, err := exec.Command(ffprobe, "-v", "error", "-show_format", "-show_streams", "-of", "json", path).Output()
	if err != nil {
		log.Printf("ffprobe %s: %v", filepath.Base(path), err)
		return 0, 0, nil
	}
	var p struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			Index     int               `json:"index"`
			CodecType string            `json:"codec_type"`
			CodecName string            `json:"codec_name"`
			Profile   string            `json:"profile"`
			PixFmt    string            `json:"pix_fmt"`
			Width     int               `json:"width"`
			Height    int               `json:"height"`
			Channels  int               `json:"channels"`
			Tags      map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &p) != nil {
		return 0, 0, nil
	}
	var dur float64
	var br int64
	fmt.Sscan(p.Format.Duration, &dur)
	fmt.Sscan(p.Format.BitRate, &br)
	tracks := make([]track, 0, len(p.Streams))
	for _, s := range p.Streams {
		tracks = append(tracks, track{s.Index, s.CodecType, s.CodecName, s.Profile, s.PixFmt, s.Width, s.Height, s.Channels, s.Tags["language"], s.Tags["title"]})
	}
	return dur, br, tracks
}

func scan(dir, ffprobe string) ([]*mediaFile, error) {
	var files []*mediaFile
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || strings.HasPrefix(fi.Name(), ".") {
			return err
		}
		if _, ok := videoExt[strings.ToLower(filepath.Ext(p))]; !ok {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		h := sha1.Sum([]byte(rel))
		id := hex.EncodeToString(h[:6])
		dur, br, tracks := probe(ffprobe, p)
		files = append(files, &mediaFile{ID: id, Name: rel, Size: fi.Size(), Duration: dur, Bitrate: br, Tracks: tracks, URL: "/media/" + id + "/" + filepath.Base(p), path: p})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, err
}

// countingWriter mide los bytes realmente enviados: AVPlay corta conexiones a mitad.
type countingWriter struct {
	http.ResponseWriter
	status int
	n      int64
}

func (w *countingWriter) WriteHeader(s int) { w.status = s; w.ResponseWriter.WriteHeader(s) }
func (w *countingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.n += int64(n)
	return n, err
}

type jsonl struct {
	mu sync.Mutex
	f  *os.File
}

func (j *jsonl) write(v map[string]any) {
	v["at"] = time.Now().Format("15:04:05.000")
	b, _ := json.Marshal(v)
	j.mu.Lock()
	defer j.mu.Unlock()
	j.f.Write(append(b, '\n'))
	fmt.Println(string(b))
}

func cors(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Range")
	h.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")
	h.Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
}

func main() {
	dir := flag.String("dir", "private/testmedia", "carpeta de vídeos")
	addr := flag.String("addr", ":8787", "dirección de escucha")
	logPath := flag.String("log", "private/spike-log.jsonl", "registro JSONL de peticiones y trazas")
	app := flag.String("app", "tools/spike-avplay", "carpeta de la app de prueba (para abrirla en un navegador)")
	ffprobe := flag.String("ffprobe", "ffprobe", "ruta de ffprobe")
	flag.Parse()

	files, err := scan(*dir, *ffprobe)
	if err != nil {
		log.Fatal(err)
	}
	byID := map[string]*mediaFile{}
	for _, f := range files {
		byID[f.ID] = f
		log.Printf("%s  %-28s %8.1f MB  %6.0f s", f.ID, f.Name, float64(f.Size)/1e6, f.Duration)
	}
	lf, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	rec := &jsonl{f: lf}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		json.NewEncoder(w).Encode(map[string]any{"name": "yourflix-spike", "files": len(files)})
	})
	mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(files)
	})
	mux.HandleFunc("/api/log", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		if r.Method == http.MethodOptions {
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var events []map[string]any
		if json.Unmarshal(body, &events) != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		for _, e := range events {
			e["src"] = "tv"
			rec.write(e)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		cors(w)
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/media/"), "/", 2)
		f := byID[parts[0]]
		if f == nil {
			http.NotFound(w, r)
			return
		}
		fh, err := os.Open(f.path)
		if err != nil {
			http.Error(w, "no se puede abrir", http.StatusInternalServerError)
			return
		}
		defer fh.Close()
		w.Header().Set("Content-Type", videoExt[strings.ToLower(filepath.Ext(f.path))])
		cw := &countingWriter{ResponseWriter: w, status: 200}
		start := time.Now()
		http.ServeContent(cw, r, "", time.Time{}, fh)
		rec.write(map[string]any{"src": "http", "method": r.Method, "file": f.Name, "range": r.Header.Get("Range"),
			"status": cw.status, "sent": cw.n, "ms": time.Since(start).Milliseconds(), "ua": r.UserAgent()})
	})
	mux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir(*app))))

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("yourflix-spike: %d archivos en %s, escuchando en %s (registro %s)", len(files), *dir, *addr, *logPath)
	log.Fatal(http.Serve(ln, mux))
}
