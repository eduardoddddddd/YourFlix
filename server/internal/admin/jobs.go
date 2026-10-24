package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

// Trabajos largos (escaneos, identificación con TMDB): uno a la vez, en cola, con su
// progreso visible en vivo por SSE (/admin/api/jobs/events) o por sondeo (/admin/api/jobs).

type Job struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"` // scan | identify
	Title      string   `json:"title"`
	State      string   `json:"state"` // queued | running | done | failed
	QueuedAt   string   `json:"queuedAt"`
	StartedAt  string   `json:"startedAt,omitempty"`
	FinishedAt string   `json:"finishedAt,omitempty"`
	Processed  int      `json:"processed"`
	Current    string   `json:"current,omitempty"`
	Lines      []string `json:"lines"`
	Summary    []string `json:"summary"`
	Error      string   `json:"error,omitempty"`

	key string
	run func(ctx context.Context, p *progress) error
}

const keepLines = 40

// progress lo usa el trabajo en marcha para informar.
type progress struct {
	r   *runner
	job *Job
}

// Line añade una línea de detalle (cuenta como un archivo procesado si count).
func (p *progress) Line(count bool, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	p.r.mu.Lock()
	if count {
		p.job.Processed++
	}
	p.job.Current = line
	p.job.Lines = append(p.job.Lines, line)
	if over := len(p.job.Lines) - keepLines; over > 0 {
		p.job.Lines = append(p.job.Lines[:0:0], p.job.Lines[over:]...)
	}
	p.r.mu.Unlock()
	p.r.changed()
}

func (p *progress) Summary(format string, args ...any) {
	p.r.mu.Lock()
	p.job.Summary = append(p.job.Summary, fmt.Sprintf(format, args...))
	p.r.mu.Unlock()
	p.r.changed()
}

type runner struct {
	mu    sync.Mutex
	jobs  []*Job // historial, el más reciente al final
	queue chan *Job
	subs  map[chan struct{}]bool
	lock  sync.Locker
	ctx   context.Context
}

var errQueueFull = errors.New("hay demasiados trabajos en cola")

func newRunner(ctx context.Context, lock sync.Locker) *runner {
	r := &runner{queue: make(chan *Job, 32), subs: map[chan struct{}]bool{}, lock: lock, ctx: ctx}
	go r.loop()
	return r
}

// enqueue añade el trabajo, salvo que ya haya uno igual esperando o en marcha (lo devuelve).
func (r *runner) enqueue(kind, key, title string, run func(context.Context, *progress) error) (*Job, error) {
	r.mu.Lock()
	for _, j := range r.jobs {
		if j.key == key && (j.State == "queued" || j.State == "running") {
			c := j.copy()
			r.mu.Unlock()
			return c, nil
		}
	}
	j := &Job{ID: store.NewID(), Kind: kind, Title: title, State: "queued", QueuedAt: store.Now(), Lines: []string{}, Summary: []string{}, key: key, run: run}
	select {
	case r.queue <- j:
	default:
		r.mu.Unlock()
		return nil, errQueueFull
	}
	r.jobs = append(r.jobs, j)
	for i := 0; len(r.jobs) > 20 && i < len(r.jobs); { // olvida los terminados más antiguos
		if s := r.jobs[i].State; s == "done" || s == "failed" {
			r.jobs = append(r.jobs[:i], r.jobs[i+1:]...)
		} else {
			i++
		}
	}
	c := j.copy()
	r.mu.Unlock()
	r.changed()
	return c, nil
}

func (r *runner) loop() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case j := <-r.queue:
			r.runOne(j)
		}
	}
}

func (r *runner) runOne(j *Job) {
	if r.lock != nil {
		r.lock.Lock() // no pisar un re-escaneo periódico de serve
		defer r.lock.Unlock()
	}
	r.mu.Lock()
	j.State, j.StartedAt = "running", store.Now()
	r.mu.Unlock()
	r.changed()
	err := func() (err error) {
		defer func() {
			if v := recover(); v != nil {
				err = fmt.Errorf("fallo interno: %v", v)
			}
		}()
		return j.run(r.ctx, &progress{r: r, job: j})
	}()
	r.mu.Lock()
	j.FinishedAt, j.State = store.Now(), "done"
	if err != nil {
		j.State, j.Error = "failed", err.Error()
	}
	r.mu.Unlock()
	r.changed()
}

func (j *Job) copy() *Job {
	c := *j
	c.Lines = append([]string{}, j.Lines...)
	c.Summary = append([]string{}, j.Summary...)
	c.run = nil
	return &c
}

// snapshot: trabajos del más reciente al más antiguo.
func (r *runner) snapshot() []*Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Job, 0, len(r.jobs))
	for i := len(r.jobs) - 1; i >= 0; i-- {
		out = append(out, r.jobs[i].copy())
	}
	return out
}

func (r *runner) get(id string) *Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, j := range r.jobs {
		if j.ID == id {
			return j.copy()
		}
	}
	return nil
}

func (r *runner) changed() {
	r.mu.Lock()
	for ch := range r.subs {
		select {
		case ch <- struct{}{}:
		default: // ya tiene un aviso pendiente
		}
	}
	r.mu.Unlock()
}

func (r *runner) subscribe() (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	r.mu.Lock()
	r.subs[ch] = true
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs, ch)
		r.mu.Unlock()
	}
}

func (a *Admin) jobsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": a.jobs.snapshot()})
}

func (a *Admin) jobGet(w http.ResponseWriter, r *http.Request) {
	j := a.jobs.get(r.PathValue("id"))
	if j == nil {
		writeError(w, http.StatusNotFound, "trabajo desconocido")
		return
	}
	writeJSON(w, http.StatusOK, j)
}

// jobsEvents: Server-Sent Events con la lista de trabajos cada vez que cambia
// (como mucho 4 veces por segundo) y un latido cada 20 s.
func (a *Admin) jobsEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sin streaming")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	ch, cancel := a.jobs.subscribe()
	defer cancel()
	send := func() bool {
		b, _ := json.Marshal(map[string]any{"jobs": a.jobs.snapshot()})
		if _, err := fmt.Fprintf(w, "event: jobs\ndata: %s\n\n", b); err != nil {
			return false
		}
		fl.Flush()
		return true
	}
	if !send() {
		return
	}
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-a.ctx.Done():
			return
		case <-beat.C:
			if _, err := fmt.Fprint(w, ": latido\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-ch:
			if !send() {
				return
			}
			select {
			case <-time.After(250 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		}
	}
}
