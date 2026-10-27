package store

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/eduardoddddddd/YourFlix/server/internal/parse"
)

type Title struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Original    string          `json:"original,omitempty"`
	Year        string          `json:"year,omitempty"`
	TMDB        int64           `json:"tmdb,omitempty"`
	IMDB        string          `json:"imdb,omitempty"`
	Meta        json.RawMessage `json:"meta,omitempty"`
	Poster      string          `json:"-"`
	Backdrop    string          `json:"-"`
	MatchSource string          `json:"matchSource"`
	Norm        string          `json:"-"`
	NormOrig    string          `json:"-"`
	Locked      bool            `json:"locked,omitempty"`
	AddedAt     string          `json:"addedAt"`
}

const titleCols = `id,kind,title,original,year,COALESCE(tmdb_id,0),imdb_id,meta,poster,backdrop,match_source,norm,norm_original,locked,added_at`

func scanTitle(sc interface{ Scan(...any) error }) (*Title, error) {
	var t Title
	var meta string
	var locked int
	if err := sc.Scan(&t.ID, &t.Kind, &t.Title, &t.Original, &t.Year, &t.TMDB, &t.IMDB, &meta, &t.Poster, &t.Backdrop, &t.MatchSource, &t.Norm, &t.NormOrig, &locked, &t.AddedAt); err != nil {
		return nil, err
	}
	t.Meta = json.RawMessage(meta)
	t.Locked = locked != 0
	return &t, nil
}

// UpsertTitle inserta o reemplaza la ficha. Respeta los títulos bloqueados salvo force.
func (db *DB) UpsertTitle(t *Title, force bool) error {
	if t.AddedAt == "" {
		t.AddedAt = Now()
	}
	if len(t.Meta) == 0 {
		t.Meta = json.RawMessage("{}")
	}
	var tmdb any
	if t.TMDB != 0 {
		tmdb = t.TMDB
	}
	locked := 0
	if t.Locked {
		locked = 1
	}
	q := `INSERT INTO titles(` + strings.ReplaceAll(titleCols, "COALESCE(tmdb_id,0)", "tmdb_id") + `) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET kind=excluded.kind,title=excluded.title,original=excluded.original,year=excluded.year,
		tmdb_id=excluded.tmdb_id,imdb_id=excluded.imdb_id,meta=excluded.meta,poster=excluded.poster,backdrop=excluded.backdrop,
		match_source=excluded.match_source,norm=excluded.norm,norm_original=excluded.norm_original`
	if force {
		q += `,locked=excluded.locked` // una edición manual puede bloquear o desbloquear
	} else {
		q += ` WHERE titles.locked=0`
	}
	_, err := db.Exec(q, t.ID, t.Kind, t.Title, t.Original, t.Year, tmdb, t.IMDB, string(t.Meta), t.Poster, t.Backdrop, t.MatchSource, t.Norm, t.NormOrig, locked, t.AddedAt)
	return err
}

func (db *DB) Title(id string) (*Title, error) {
	t, err := scanTitle(db.QueryRow(`SELECT `+titleCols+` FROM titles WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return t, err
}

// TitlesByNorm devuelve los títulos con ese nombre normalizado (y año, si se da).
func (db *DB) TitlesByNorm(kind, norm, year string) ([]*Title, error) {
	q := `SELECT ` + titleCols + ` FROM titles WHERE kind=? AND (norm=? OR norm_original=?)`
	args := []any{kind, norm, norm}
	if year != "" {
		q += ` AND (year=? OR year='')`
		args = append(args, year)
	}
	return db.queryTitles(q, args...)
}

func (db *DB) queryTitles(q string, args ...any) ([]*Title, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Title
	for rows.Next() {
		t, err := scanTitle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type SeedCopy struct {
	TitleID string
	Bytes   int64
	Seconds float64
	Season  *int
	Episode *int
	Label   string
}

func (db *DB) ReplaceSeedCopies(titleID string, copies []SeedCopy) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM seed_copies WHERE title_id=?`, titleID); err != nil {
		return err
	}
	for _, c := range copies {
		if _, err := tx.Exec(`INSERT INTO seed_copies(title_id,bytes,seconds,season,episode,label) VALUES(?,?,?,?,?,?)`,
			titleID, c.Bytes, c.Seconds, c.Season, c.Episode, c.Label); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SeedCopiesByBytes: copias medidas en la semilla con exactamente ese tamaño.
func (db *DB) SeedCopiesByBytes(n int64) ([]SeedCopy, error) {
	rows, err := db.Query(`SELECT title_id,bytes,COALESCE(seconds,0),season,episode,label FROM seed_copies WHERE bytes=?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SeedCopy
	for rows.Next() {
		var c SeedCopy
		var s, e sql.NullInt64
		if err := rows.Scan(&c.TitleID, &c.Bytes, &c.Seconds, &s, &e, &c.Label); err != nil {
			return nil, err
		}
		if s.Valid {
			v := int(s.Int64)
			c.Season = &v
		}
		if e.Valid {
			v := int(e.Int64)
			c.Episode = &v
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TitleSummary es una fila del catálogo para la TV: solo títulos con algún archivo disponible.
type TitleSummary struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Title    string   `json:"title"`
	Year     string   `json:"year,omitempty"`
	Genres   []string `json:"genres,omitempty"`
	Rating   float64  `json:"rating,omitempty"`
	Poster   string   `json:"poster,omitempty"`
	Backdrop string   `json:"backdrop,omitempty"`
	Files    int      `json:"files"`
	Sources  []string `json:"sources"`
	AddedAt  string   `json:"addedAt"` // llegada del archivo disponible más reciente (ver lastFileAt)
	Matched  bool     `json:"matched"`
}

// lastFileAt: fecha en que llegó el archivo disponible más reciente del título. Es la
// «novedad» que ve el usuario: una ficha de la semilla creada días antes sin archivo, o una
// serie a la que se le añade un capítulo, suben a «Recién añadidas» cuando llega el archivo.
// titles.added_at solo dice cuándo se creó la ficha.
const lastFileAt = `(SELECT MAX(f.added_at) FROM files f WHERE f.title_id=t.id AND f.missing_since IS NULL)`

type TitleQuery struct {
	Kind, Q, Sort, Genre string
	Offset, Limit        int
	// IDs restringe la lista a esos títulos (nil = sin restricción; vacío = ninguno). Lo
	// rellena el índice de categorías. Con Sort "ids" se respeta su orden (aleatorio del kiosco).
	IDs []string
}

func (db *DB) ListTitles(tq TitleQuery) ([]TitleSummary, int, error) {
	where := `EXISTS (SELECT 1 FROM files f WHERE f.title_id=t.id AND f.missing_since IS NULL)`
	var args []any
	from := `titles t`
	if tq.IDs != nil {
		ids, _ := json.Marshal(tq.IDs)
		if tq.Sort == "ids" {
			from = `titles t JOIN json_each(?) j ON j.value=t.id`
		} else {
			where += ` AND t.id IN (SELECT value FROM json_each(?))`
		}
		args = append(args, string(ids))
	}
	if tq.Kind != "" {
		where += ` AND t.kind=?`
		args = append(args, tq.Kind)
	}
	if tq.Genre != "" {
		where += ` AND EXISTS (SELECT 1 FROM json_each(t.meta,'$.genres') WHERE value=?)`
		args = append(args, tq.Genre)
	}
	if tq.Q != "" {
		// Título (normalizado, sin tildes), original, director y reparto; no la sinopsis:
		// «casa» no debe traer «El laberinto del fauno».
		where += ` AND (t.norm LIKE ? OR t.norm_original LIKE ? OR lower(COALESCE(json_extract(t.meta,'$.director'),'')) LIKE ? OR lower(COALESCE(json_extract(t.meta,'$.cast'),'')) LIKE ?)`
		norm := "%" + parse.Normalize(tq.Q) + "%"
		like := "%" + strings.ToLower(tq.Q) + "%"
		args = append(args, norm, norm, like, like)
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM `+from+` WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := `t.norm`
	switch tq.Sort {
	case "year":
		order = `t.year DESC, t.norm`
	case "added":
		order = lastFileAt + ` DESC, t.norm`
	case "rating":
		order = `COALESCE(json_extract(t.meta,'$.rating'),0) DESC, t.norm`
	case "runtime": // de la más corta a la más larga; sin duración, al final
		order = `COALESCE(json_extract(t.meta,'$.runtime'),0)<=0, json_extract(t.meta,'$.runtime'), t.norm`
	case "ids":
		if tq.IDs != nil {
			order = `j.key`
		}
	}
	if tq.Limit <= 0 || tq.Limit > 500 {
		tq.Limit = 60
	}
	q := `SELECT t.id,t.kind,t.title,t.year,t.meta,t.poster,t.backdrop,COALESCE(` + lastFileAt + `,t.added_at),t.match_source,
		(SELECT COUNT(*) FROM files f WHERE f.title_id=t.id AND f.missing_since IS NULL),
		(SELECT group_concat(DISTINCT s.label) FROM files f JOIN sources s ON s.id=f.source_id WHERE f.title_id=t.id AND f.missing_since IS NULL)
		FROM ` + from + ` WHERE ` + where + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	rows, err := db.Query(q, append(args, tq.Limit, tq.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []TitleSummary{}
	for rows.Next() {
		var s TitleSummary
		var meta, src, labels string
		if err := rows.Scan(&s.ID, &s.Kind, &s.Title, &s.Year, &meta, &s.Poster, &s.Backdrop, &s.AddedAt, &src, &s.Files, &labels); err != nil {
			return nil, 0, err
		}
		var m struct {
			Genres []string `json:"genres"`
			Rating float64  `json:"rating"`
		}
		json.Unmarshal([]byte(meta), &m)
		s.Genres, s.Rating = m.Genres, m.Rating
		s.Sources = strings.Split(labels, ",")
		s.Matched = src != "filename"
		out = append(out, s)
	}
	return out, total, rows.Err()
}

// TitleByTMDB: ficha existente con ese ID de TMDB (para fusionar en vez de duplicar).
func (db *DB) TitleByTMDB(kind string, id int64) (*Title, error) {
	ts, err := db.queryTitles(`SELECT `+titleCols+` FROM titles WHERE kind=? AND tmdb_id=? AND match_source!='filename' LIMIT 1`, kind, id)
	if err != nil || len(ts) == 0 {
		return nil, err
	}
	return ts[0], nil
}

// PendingTitles: fichas provisionales (deducidas del nombre) con algún archivo disponible.
func (db *DB) PendingTitles() ([]*Title, error) {
	return db.queryTitles(`SELECT ` + titleCols + ` FROM titles t WHERE match_source='filename' AND locked=0
		AND EXISTS (SELECT 1 FROM files f WHERE f.title_id=t.id AND f.missing_since IS NULL) ORDER BY norm`)
}

// MoveFiles pasa los archivos de una ficha a otra (fusión) y borra la de origen.
func (db *DB) MoveFiles(from, to, matchSource string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE files SET title_id=?, match_source=? WHERE title_id=?`, to, matchSource, from); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM titles WHERE id=? AND locked=0`, from); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) SetFilesMatchSource(titleID, matchSource string) error {
	_, err := db.Exec(`UPDATE files SET match_source=? WHERE title_id=? AND match_source!='manual'`, matchSource, titleID)
	return err
}

// SeriesMissingEpisodes: series con ficha (semilla, TMDB o manual) y algún capítulo en
// disco cuyo «T×E» aún no está en meta.episodes.
func (db *DB) SeriesMissingEpisodes() ([]*Title, error) {
	ts, err := db.queryTitles(`SELECT ` + titleCols + ` FROM titles WHERE kind='series' AND match_source!='filename' AND tmdb_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	var out []*Title
	for _, t := range ts {
		var m struct {
			Episodes map[string]json.RawMessage `json:"episodes"`
		}
		json.Unmarshal(t.Meta, &m)
		files, err := db.FilesOfTitle(t.ID)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.Season != nil && f.Episode != nil {
				if _, ok := m.Episodes[strconv.Itoa(*f.Season)+"x"+strconv.Itoa(*f.Episode)]; !ok {
					out = append(out, t)
					break
				}
			}
		}
	}
	return out, nil
}

type GenreCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Genres: géneros de los títulos disponibles, de más a menos frecuente.
func (db *DB) Genres() ([]GenreCount, error) {
	rows, err := db.Query(`SELECT g.value, COUNT(*) FROM titles t, json_each(t.meta,'$.genres') g
		WHERE EXISTS (SELECT 1 FROM files f WHERE f.title_id=t.id AND f.missing_since IS NULL)
		GROUP BY g.value ORDER BY COUNT(*) DESC, g.value`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GenreCount{}
	for rows.Next() {
		var g GenreCount
		if err := rows.Scan(&g.Name, &g.Count); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ContinueItem: lo último que se dejó a medias de cada título o, en una serie cuyo último
// capítulo visto se terminó, el siguiente capítulo (Next=true, desde el principio).
type ContinueItem struct {
	TitleID   string  `json:"titleId"`
	FileID    string  `json:"fileId"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	UpdatedAt string  `json:"updatedAt"`
	Next      bool    `json:"next,omitempty"`
}

func (db *DB) Continue(limit int) ([]ContinueItem, error) {
	rows, err := db.Query(`SELECT f.title_id, p.file_id, p.position, p.duration, p.updated_at, p.watched, t.kind
		FROM progress p JOIN files f ON f.id=p.file_id JOIN titles t ON t.id=f.title_id
		WHERE f.missing_since IS NULL AND (p.watched=1 OR p.position>=60)
		ORDER BY p.updated_at DESC LIMIT 400`)
	if err != nil {
		return nil, err
	}
	type rec struct {
		ContinueItem
		watched bool
		kind    string
	}
	var recs []rec
	for rows.Next() {
		var r rec
		var w int
		if err := rows.Scan(&r.TitleID, &r.FileID, &r.Position, &r.Duration, &r.UpdatedAt, &w, &r.kind); err != nil {
			rows.Close()
			return nil, err
		}
		r.watched = w != 0
		recs = append(recs, r)
	}
	rows.Close()
	seen := map[string]bool{}
	out := []ContinueItem{}
	for _, r := range recs {
		if len(out) >= limit {
			break
		}
		if seen[r.TitleID] {
			continue
		}
		seen[r.TitleID] = true // solo cuenta lo más reciente de cada título
		if !r.watched {
			out = append(out, r.ContinueItem)
			continue
		}
		if r.kind != "series" {
			continue // película terminada: fuera de «Seguir viendo»
		}
		if next, err := db.nextEpisode(r.TitleID, r.FileID); err == nil && next != nil {
			out = append(out, ContinueItem{TitleID: r.TitleID, FileID: next.ID, Duration: next.Duration, UpdatedAt: r.UpdatedAt, Next: true})
		}
	}
	return out, nil
}

// nextEpisode: el capítulo siguiente en orden (temporada, capítulo) que no esté visto.
func (db *DB) nextEpisode(titleID, fileID string) (*File, error) {
	files, err := db.FilesOfTitle(titleID)
	if err != nil {
		return nil, err
	}
	for i, f := range files {
		if f.ID != fileID {
			continue
		}
		for _, n := range files[i+1:] {
			if p, _ := db.ProgressOf(n.ID); p == nil || !p.Watched {
				return n, nil
			}
		}
	}
	return nil, nil
}
