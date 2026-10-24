package admin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/probe"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

type toolInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Found   bool   `json:"found"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// toolCache: ffmpeg/ffprobe se comprueban como mucho una vez por minuto.
type toolCache struct {
	mu    sync.Mutex
	at    time.Time
	tools []toolInfo
}

func checkTool(name, path string) toolInfo {
	t := toolInfo{Name: name, Path: path}
	if path == "" {
		path = name
	}
	p, err := exec.LookPath(path)
	if err != nil {
		t.Error = "no encontrado"
		return t
	}
	t.Path = p
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, p, "-version").Output()
	if err != nil {
		t.Error = "no arranca: " + err.Error()
		return t
	}
	t.Found = true
	line, _, _ := bufio.NewReader(bytes.NewReader(out)).ReadLine()
	v := string(line)
	if i := strings.Index(v, " Copyright"); i > 0 {
		v = v[:i]
	}
	t.Version = strings.TrimSpace(v)
	return t
}

func (a *Admin) toolsInfo() []toolInfo {
	a.tools.mu.Lock()
	defer a.tools.mu.Unlock()
	if a.tools.tools == nil || time.Since(a.tools.at) > time.Minute {
		a.tools.tools = []toolInfo{checkTool("ffprobe", a.cfg.FFprobe), checkTool("ffmpeg", a.cfg.FFmpeg)}
		a.tools.at = time.Now()
	}
	return append([]toolInfo{}, a.tools.tools...)
}

type diskOut struct {
	Key   string   `json:"-"`
	Path  string   `json:"path"`
	Used  []string `json:"usedBy"` // etiquetas de fuentes (y «datos»)
	Free  uint64   `json:"free"`
	Total uint64   `json:"total"`
	Error string   `json:"error,omitempty"`
}

// disks: espacio de cada disco con fuentes y del de la carpeta de datos, sin bloquear
// más de 3 s por un disco dormido.
func (a *Admin) disks(sources []store.Source) []*diskOut {
	type item struct{ label, path string }
	items := []item{}
	if a.cfg.DataDir != "" {
		items = append(items, item{"datos de YourFlix", a.cfg.DataDir})
	}
	for _, s := range sources {
		items = append(items, item{s.Label, s.Path})
	}
	type res struct {
		i           int
		key         string
		free, total uint64
		err         error
	}
	ch := make(chan res, len(items))
	for i, it := range items {
		go func(i int, p string) {
			k, f, t, err := diskUsage(p)
			ch <- res{i, k, f, t, err}
		}(i, it.path)
	}
	results := make([]*res, len(items))
	deadline := time.After(3 * time.Second)
wait:
	for n := 0; n < len(items); n++ {
		select {
		case r := <-ch:
			results[r.i] = &r
		case <-deadline:
			break wait
		}
	}
	out := []*diskOut{}
	byKey := map[string]*diskOut{}
	for i, it := range items {
		r := results[i]
		switch {
		case r == nil:
			out = append(out, &diskOut{Path: it.path, Used: []string{it.label}, Error: "no responde"})
		case r.err != nil:
			out = append(out, &diskOut{Path: it.path, Used: []string{it.label}, Error: errText(r.err)})
		case byKey[r.key] != nil:
			byKey[r.key].Used = append(byKey[r.key].Used, it.label)
		default:
			d := &diskOut{Key: r.key, Path: it.path, Used: []string{it.label}, Free: r.free, Total: r.total}
			if runtime.GOOS == "windows" && len(r.key) == 2 {
				d.Path = r.key + `\`
			}
			byKey[r.key] = d
			out = append(out, d)
		}
	}
	return out
}

func (a *Admin) status(w http.ResponseWriter, r *http.Request) {
	srcs, err := a.cfg.DB.Sources()
	if err != nil {
		a.fail(w, err)
		return
	}
	runs, err := a.cfg.DB.RecentScanRuns(15)
	if err != nil {
		a.fail(w, err)
		return
	}
	type srcScan struct {
		ID         string `json:"id"`
		Label      string `json:"label"`
		State      string `json:"state"`
		Files      int    `json:"files"`
		LastScanAt string `json:"lastScanAt,omitempty"`
	}
	last := []srcScan{}
	for _, s := range srcs {
		last = append(last, srcScan{s.ID, s.Label, s.State, s.Files, s.LastScanAt})
	}
	var titles, pending int
	a.cfg.DB.QueryRow(`SELECT COUNT(*) FROM titles t WHERE EXISTS (SELECT 1 FROM files f WHERE f.title_id=t.id AND f.missing_since IS NULL)`).Scan(&titles)
	if p, err := a.cfg.DB.PendingTitles(); err == nil {
		pending = len(p)
	}
	up := time.Since(a.cfg.Started)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":       a.cfg.Version,
		"startedAt":     a.cfg.Started.UTC().Format(time.RFC3339),
		"uptimeSeconds": int64(up.Seconds()),
		"dataDir":       a.cfg.DataDir,
		"os":            runtime.GOOS + "/" + runtime.GOARCH,
		"go":            runtime.Version(),
		"tools":         a.toolsInfo(),
		"tmdb":          a.cfg.TMDB.Enabled(), // solo si hay credenciales; nunca el token
		"passwordSet":   HasPassword(a.cfg.DB),
		"disks":         a.disks(srcs),
		"sources":       last,
		"scans":         runs,
		"titles":        titles,
		"pending":       pending,
		"log":           a.cfg.Ring.Last(40),
	})
}

func (a *Admin) logLines(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n <= 0 || n > 1000 {
		n = 200
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": a.cfg.Ring.Last(n)})
}

// compatSummary: archivos que el perfil de la TV marca con avisos u «optimizar».
func (a *Admin) compatSummary(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Profile == nil {
		writeError(w, http.StatusServiceUnavailable, "sin perfil de la TV")
		return
	}
	files, err := a.cfg.DB.AvailableFiles()
	if err != nil {
		a.fail(w, err)
		return
	}
	names, err := a.cfg.DB.TitleNames()
	if err != nil {
		a.fail(w, err)
		return
	}
	type item struct {
		FileID  string   `json:"fileId"`
		TitleID string   `json:"titleId,omitempty"`
		Title   string   `json:"title"`
		Year    string   `json:"year,omitempty"`
		Source  string   `json:"source"`
		Path    string   `json:"path"`
		Season  *int     `json:"season,omitempty"`
		Episode *int     `json:"episode,omitempty"`
		Status  string   `json:"status"`
		Reasons []string `json:"reasons"`
	}
	counts := map[string]int{"direct": 0, "warn": 0, "optimize": 0}
	reasons := map[string]int{}
	items := []item{}
	for _, f := range files {
		var v *probe.Video
		var au []probe.Audio
		var sb []probe.Subtitle
		json.Unmarshal(f.Video, &v)
		json.Unmarshal(f.Audio, &au)
		json.Unmarshal(f.Subs, &sb)
		verdict := a.cfg.Profile.Check(f.Container, f.Bitrate, v, au, sb)
		counts[verdict.Status]++
		if verdict.Status == "direct" {
			continue
		}
		for _, rs := range verdict.Reasons {
			reasons[reasonKind(rs)]++
		}
		n := names[f.TitleID]
		title := n.Title
		if title == "" {
			title = f.RelPath
		}
		rs := verdict.Reasons
		if rs == nil {
			rs = []string{}
		}
		items = append(items, item{FileID: f.ID, TitleID: f.TitleID, Title: title, Year: n.Year, Source: f.SourceLabel, Path: f.RelPath, Season: f.Season, Episode: f.Episode, Status: verdict.Status, Reasons: rs})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Status != items[j].Status {
			return items[i].Status == "optimize"
		}
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
	type rc struct {
		Reason string `json:"reason"`
		Count  int    `json:"count"`
	}
	byReason := []rc{}
	for k, v := range reasons {
		byReason = append(byReason, rc{k, v})
	}
	sort.Slice(byReason, func(i, j int) bool {
		if byReason[i].Count != byReason[j].Count {
			return byReason[i].Count > byReason[j].Count
		}
		return byReason[i].Reason < byReason[j].Reason
	})
	writeJSON(w, http.StatusOK, map[string]any{"profile": a.cfg.Profile.ID, "total": len(files), "counts": counts, "reasons": byReason, "items": items})
}

// reasonKind agrupa motivos parecidos («3 subtítulos en imagen…», «45 Mb/s: …»).
func reasonKind(r string) string {
	switch {
	case strings.Contains(r, "subtítulos en imagen"):
		return "subtítulos en imagen (PGS/DVD)"
	case strings.Contains(r, "Mb/s"):
		return "caudal alto (Wi-Fi)"
	case strings.HasPrefix(r, "resolución"):
		return "resolución mayor que la TV"
	case strings.HasPrefix(r, "pistas de audio no disponibles"):
		return "alguna pista de audio no suena"
	case strings.HasPrefix(r, "audio ") && strings.HasSuffix(r, "sin sonido en la TV"):
		return "sin audio compatible (sin sonido)"
	}
	return r
}
