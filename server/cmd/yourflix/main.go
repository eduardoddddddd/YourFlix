// yourflix: servidor de películas y series para la Samsung The Serif.
//
//	yourflix [--data DIR] serve [--addr :8787] [--rescan 30m]
//	yourflix source add RUTA [--label L] [--kind auto|movies|series] [--exclude PATRÓN]...
//	yourflix source list | scan [ID|--all] | pause ID | resume ID | remove ID
//	yourflix seed import RUTA-CLON-cine-t7-catalogo
//	yourflix titles [--q TEXTO] [--kind movie|series] [--pending]
//	yourflix identify [--retry]            (TMDB para lo que no está en la semilla)
//	yourflix match ID_FICHA --tmdb ID      (elegir a mano; queda bloqueada)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/eduardoddddddd/YourFlix/server/internal/api"
	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
	"github.com/eduardoddddddd/YourFlix/server/internal/identify"
	"github.com/eduardoddddddd/YourFlix/server/internal/images"
	"github.com/eduardoddddddd/YourFlix/server/internal/scan"
	"github.com/eduardoddddddd/YourFlix/server/internal/seed"
	"github.com/eduardoddddddd/YourFlix/server/internal/store"
	"github.com/eduardoddddddd/YourFlix/server/internal/tmdb"
)

const version = "0.1.0"

type app struct {
	dataDir, ffprobe, profile string
	tmdbTokenFile             string
	tmdb                      *tmdb.Client
	db                        *store.DB
}

func main() {
	log.SetFlags(log.Ltime)
	g := flag.NewFlagSet("yourflix", flag.ExitOnError)
	a := &app{}
	g.StringVar(&a.dataDir, "data", envOr("YOURFLIX_DATA", "data"), "carpeta de datos (base de datos, imágenes)")
	g.StringVar(&a.ffprobe, "ffprobe", envOr("YOURFLIX_FFPROBE", ""), "ruta de ffprobe (por defecto: junto al ejecutable o en el PATH)")
	g.StringVar(&a.profile, "profile", "", "perfil de la TV (por defecto el incrustado)")
	g.StringVar(&a.tmdbTokenFile, "tmdb-token-file", envOr("YOURFLIX_TMDB_TOKEN_FILE", ""), "fichero con el token de lectura de TMDB (o TMDB_ACCESS_TOKEN / TMDB_API_KEY)")
	g.Usage = usage
	g.Parse(os.Args[1:])
	args := g.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	if args[0] == "version" {
		fmt.Println("yourflix", version)
		return
	}
	// Absoluta: las rutas de imágenes guardadas en la base de datos no deben depender
	// de la carpeta desde la que se lance (servicio de Windows, launchd, terminal).
	if abs, err := filepath.Abs(a.dataDir); err == nil {
		a.dataDir = abs
	}
	if err := os.MkdirAll(a.dataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(filepath.Join(a.dataDir, "yourflix.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	a.db = db
	if a.ffprobe == "" {
		a.ffprobe = findFFprobe()
	}
	if err := a.setupTMDB(); err != nil {
		log.Fatal(err)
	}

	switch args[0] {
	case "serve":
		err = a.serve(args[1:])
	case "source", "sources":
		err = a.source(args[1:])
	case "seed":
		err = a.seed(args[1:])
	case "identify":
		err = a.identify(args[1:])
	case "match":
		err = a.match(args[1:])
	case "titles":
		err = a.titles(args[1:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `yourflix `+version+`

  yourflix [--data DIR] serve [--addr :8787] [--rescan 30m]
  yourflix source add RUTA [--label "Samsung T7"] [--kind auto|movies|series] [--exclude "*sample*"]
  yourflix source list
  yourflix source scan [ID|ETIQUETA|--all]
  yourflix source pause|resume|remove ID|ETIQUETA
  yourflix seed import RUTA       (clon de cine-t7-catalogo)
  yourflix titles [--q TEXTO] [--kind movie|series] [--pending]
  yourflix identify [--retry]     (TMDB para lo que no está en la semilla)
  yourflix match ID_FICHA --tmdb ID   (elegir la ficha a mano; queda bloqueada)

Opciones globales: --data (o YOURFLIX_DATA), --ffprobe (o YOURFLIX_FFPROBE), --profile,
  --tmdb-token-file (o YOURFLIX_TMDB_TOKEN_FILE, TMDB_ACCESS_TOKEN, TMDB_API_KEY)
`)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// findFFprobe: primero junto al ejecutable (instalación en Windows), luego el PATH.
func findFFprobe() string {
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// parseInterleaved admite opciones antes o después de los argumentos posicionales.
func parseInterleaved(fs *flag.FlagSet, args []string) []string {
	var pos []string
	for {
		fs.Parse(args)
		args = fs.Args()
		if len(args) == 0 {
			return pos
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func (a *app) source(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("source add", flag.ExitOnError)
		label := fs.String("label", "", "etiqueta visible en la TV")
		kind := fs.String("kind", "auto", "auto | movies | series")
		var ex multi
		fs.Var(&ex, "exclude", "patrón a excluir (repetible)")
		pos := parseInterleaved(fs, args[1:])
		if len(pos) != 1 {
			return errors.New("uso: source add RUTA [--label L] [--kind K] [--exclude P]")
		}
		if st, err := os.Stat(pos[0]); err != nil || !st.IsDir() {
			return fmt.Errorf("%s no es una carpeta accesible", pos[0])
		}
		s, err := a.db.AddSource(*label, pos[0], *kind, ex)
		if err != nil {
			return err
		}
		fmt.Printf("Fuente añadida: %s «%s» %s\nEscanea con: yourflix source scan %s\n", s.ID, s.Label, s.Path, s.ID)
		return nil
	case "list":
		list, err := a.db.Sources()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tETIQUETA\tTIPO\tESTADO\tARCHIVOS\tÚLTIMO ESCANEO\tRUTA")
		for _, s := range list {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n", s.ID, s.Label, s.Kind, s.State, s.Files, s.LastScanAt, s.Path)
		}
		return tw.Flush()
	case "scan":
		var list []*store.Source
		if len(args) < 2 || args[1] == "--all" {
			all, err := a.db.Sources()
			if err != nil {
				return err
			}
			for i := range all {
				list = append(list, &all[i])
			}
		} else {
			s, err := a.db.Source(args[1])
			if err != nil {
				return err
			}
			list = append(list, s)
		}
		for _, s := range list {
			if err := a.scanOne(context.Background(), s, true); err != nil {
				return err
			}
		}
		return nil
	case "pause", "resume", "remove":
		if len(args) != 2 {
			return fmt.Errorf("uso: source %s ID", args[0])
		}
		s, err := a.db.Source(args[1])
		if err != nil {
			return err
		}
		switch args[0] {
		case "pause":
			err = a.db.SetSourceState(s.ID, "paused")
		case "resume":
			err = a.db.SetSourceState(s.ID, "active")
		case "remove":
			err = a.db.RemoveSource(s.ID)
			if err == nil {
				a.db.DeleteOrphanTitles()
				fmt.Println("Quitada del catálogo (no se ha borrado ningún archivo del disco).")
			}
		}
		return err
	}
	return fmt.Errorf("subcomando desconocido: source %s", args[0])
}

func (a *app) scanOne(ctx context.Context, s *store.Source, verbose bool) error {
	logf := func(string, ...any) {}
	if verbose {
		logf = func(f string, v ...any) { fmt.Printf(f+"\n", v...) }
	}
	fmt.Printf("Escaneando «%s» (%s)…\n", s.Label, s.Path)
	t0 := time.Now()
	rep, err := scan.Source(ctx, a.db, s, scan.Options{FFprobe: a.ffprobe, Log: logf})
	if err != nil {
		return err
	}
	if rep.Offline {
		fmt.Printf("«%s» no está conectada: %d archivos marcados como ausentes.\n", s.Label, rep.Missing)
		return nil
	}
	fmt.Printf("«%s»: %d vídeos · %d nuevos · %d cambiados · %d movidos · %d ausentes · fichas: %d semilla (bytes), %d semilla/TMDB (título), %d solo nombre · %d errores de ffprobe · %s\n",
		s.Label, rep.Seen, rep.New, rep.Changed, rep.Moved, rep.Missing, rep.MatchedSeedBytes, rep.MatchedSeedTitle, rep.FromFilename, rep.ProbeErrors, time.Since(t0).Round(time.Millisecond))
	return nil
}

func (a *app) seed(args []string) error {
	if len(args) != 2 || args[0] != "import" {
		return errors.New("uso: seed import RUTA-CLON-cine-t7-catalogo")
	}
	rep, err := seed.Import(a.db, args[1], a.dataDir)
	if err != nil {
		return err
	}
	fmt.Printf("Semilla importada (catálogo del %s): %d fichas, %d copias medidas, %d portadas copiadas, %d bloqueadas sin tocar.\n",
		rep.Updated, rep.Titles, rep.Copies, rep.Images, rep.Skipped)
	fmt.Println("Vuelve a escanear las fuentes para emparejar: yourflix source scan --all")
	return nil
}

func (a *app) titles(args []string) error {
	fs := flag.NewFlagSet("titles", flag.ExitOnError)
	q := fs.String("q", "", "buscar")
	kind := fs.String("kind", "", "movie | series")
	pending := fs.Bool("pending", false, "solo archivos sin ficha verificada (pendientes de TMDB)")
	fs.Parse(args)
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if *pending {
		ts, err := a.db.PendingTitles()
		if err != nil {
			return err
		}
		fmt.Fprintln(tw, "ID_FICHA\tTIPO\tDEDUCIDO DEL NOMBRE\tARCHIVO\tCANDIDATOS TMDB (ID título año puntuación)")
		for _, t := range ts {
			files, _ := a.db.FilesOfTitle(t.ID)
			name := ""
			if len(files) > 0 {
				name = files[0].SourceLabel + ": " + files[0].RelPath
			}
			var m struct {
				Candidates []identify.Candidate `json:"candidates"`
			}
			json.Unmarshal(t.Meta, &m)
			var cs []string
			for i, c := range m.Candidates {
				if i == 3 {
					break
				}
				cs = append(cs, fmt.Sprintf("%d «%s» %s %.2f", c.TMDB, c.Title, c.Year, c.Score))
			}
			if len(cs) == 0 {
				cs = []string{"(sin buscar aún: yourflix identify)"}
			}
			fmt.Fprintf(tw, "%s\t%s\t%s %s\t%s\t%s\n", t.ID, t.Kind, t.Title, t.Year, name, strings.Join(cs, " · "))
		}
		tw.Flush()
		fmt.Printf("%d fichas pendientes\n", len(ts))
		return nil
	}
	items, total, err := a.db.ListTitles(store.TitleQuery{Q: *q, Kind: *kind, Limit: 500})
	if err != nil {
		return err
	}
	fmt.Fprintln(tw, "ID\tTIPO\tTÍTULO\tAÑO\tARCHIVOS\tFUENTES\tFICHA")
	for _, t := range items {
		st := "verificada"
		if !t.Matched {
			st = "pendiente"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n", t.ID, t.Kind, t.Title, t.Year, t.Files, strings.Join(t.Sources, ", "), st)
	}
	tw.Flush()
	fmt.Printf("%d títulos disponibles\n", total)
	return nil
}

func (a *app) serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":8787", "dirección de escucha")
	rescan := fs.Duration("rescan", 30*time.Minute, "re-escaneo periódico de las fuentes (0 = desactivado)")
	fs.Parse(args)
	prof, err := compat.Load(a.profile)
	if err != nil {
		return err
	}
	srv := &api.Server{DB: a.db, Images: images.New(filepath.Join(a.dataDir, "images")), Profile: prof, Version: version, ServerID: a.serverID(), Log: log.Default()}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go a.scanLoop(ctx, *rescan)
	hs := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() { <-ctx.Done(); hs.Shutdown(context.Background()) }()
	log.Printf("YourFlix %s escuchando en %s · datos en %s · perfil %s · ffprobe %s", version, *addr, a.dataDir, prof.ID, a.ffprobe)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (a *app) scanLoop(ctx context.Context, every time.Duration) {
	for {
		list, err := a.db.Sources()
		if err == nil {
			for i := range list {
				if err := a.scanOne(ctx, &list[i], false); err != nil {
					log.Printf("escaneo «%s»: %v", list[i].Label, err)
				}
			}
		}
		if a.tmdb.Enabled() {
			if rep, err := identify.Pending(ctx, a.db, a.tmdb, identify.Options{}); err != nil {
				log.Printf("TMDB: %v", err)
			} else if rep.Tried > 0 {
				log.Printf("TMDB: %d identificadas, %d fusionadas con fichas existentes, %d pendientes, %d errores", rep.Identified, rep.Merged, rep.Pending, len(rep.Errors))
			}
		}
		if every <= 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func (a *app) serverID() string {
	var id string
	if err := a.db.QueryRow(`SELECT value FROM settings WHERE key='serverId'`).Scan(&id); err == nil {
		return id
	}
	id = store.NewID()
	a.db.Exec(`INSERT INTO settings(key,value) VALUES('serverId',?)`, id)
	return id
}

func (a *app) setupTMDB() error {
	c := &tmdb.Client{Token: os.Getenv("TMDB_ACCESS_TOKEN"), APIKey: os.Getenv("TMDB_API_KEY"), Cache: store.TMDBCache{DB: a.db, TTL: 30 * 24 * time.Hour}}
	if a.tmdbTokenFile != "" {
		b, err := os.ReadFile(a.tmdbTokenFile)
		if err != nil {
			return fmt.Errorf("token de TMDB: %w", err)
		}
		c.Token = strings.TrimSpace(string(b))
	}
	a.tmdb = c
	return nil
}

func (a *app) identify(args []string) error {
	fs := flag.NewFlagSet("identify", flag.ExitOnError)
	retry := fs.Bool("retry", false, "reintentar también las pendientes de otras pasadas")
	fs.Parse(args)
	if !a.tmdb.Enabled() {
		return tmdb.ErrNoCredentials
	}
	rep, err := identify.Pending(context.Background(), a.db, a.tmdb, identify.Options{Retry: *retry, Log: func(f string, v ...any) { fmt.Printf(f+"\n", v...) }})
	if err != nil {
		return err
	}
	fmt.Printf("TMDB: %d probadas · %d identificadas · %d fusionadas con fichas existentes · %d pendientes · %d sin reintentar · %d series con capítulos nuevos · %d errores\n",
		rep.Tried, rep.Identified, rep.Merged, rep.Pending, rep.Skipped, rep.Episodes, len(rep.Errors))
	if rep.Pending > 0 {
		fmt.Println("Ver candidatos: yourflix titles --pending · elegir: yourflix match ID_FICHA --tmdb ID")
	}
	return nil
}

func (a *app) match(args []string) error {
	fs := flag.NewFlagSet("match", flag.ExitOnError)
	id := fs.Int64("tmdb", 0, "ID de TMDB")
	pos := parseInterleaved(fs, args)
	if len(pos) != 1 || *id == 0 {
		return errors.New("uso: match ID_FICHA --tmdb ID")
	}
	t, err := a.db.Title(pos[0])
	if err != nil {
		return err
	}
	if !a.tmdb.Enabled() {
		return tmdb.ErrNoCredentials
	}
	out, err := identify.Apply(context.Background(), a.db, a.tmdb, t, *id, 1, true)
	if err != nil {
		return err
	}
	fmt.Printf("«%s» → %s (%s), TMDB %d. Ficha bloqueada: ni la semilla ni TMDB la cambiarán.\n", t.Title, out.Title, out.Year, *id)
	return nil
}
