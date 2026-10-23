package store

import (
	"database/sql"
	"encoding/json"
)

type File struct {
	ID           string          `json:"id"`
	SourceID     string          `json:"sourceId"`
	SourceLabel  string          `json:"source"`
	RelPath      string          `json:"-"`
	Size         int64           `json:"size"`
	Mtime        int64           `json:"-"`
	Fingerprint  string          `json:"-"`
	Duration     float64         `json:"duration"`
	Container    string          `json:"container"`
	Bitrate      int64           `json:"bitrate"`
	Video        json.RawMessage `json:"video"`
	Audio        json.RawMessage `json:"audio"`
	Subs         json.RawMessage `json:"subtitles"`
	TitleID      string          `json:"titleId,omitempty"`
	Season       *int            `json:"season,omitempty"`
	Episode      *int            `json:"episode,omitempty"`
	Label        string          `json:"label,omitempty"`
	MatchSource  string          `json:"matchSource,omitempty"`
	MissingSince string          `json:"missingSince,omitempty"`
	AddedAt      string          `json:"addedAt"`
}

const fileCols = `f.id,f.source_id,s.label,f.relpath,f.size,f.mtime,f.fingerprint,f.duration,f.container,f.bitrate,f.video,f.audio,f.subs,
	COALESCE(f.title_id,''),f.season,f.episode,f.label,f.match_source,COALESCE(f.missing_since,''),f.added_at`

func scanFile(sc interface{ Scan(...any) error }) (*File, error) {
	var f File
	var v, a, s string
	var season, episode sql.NullInt64
	if err := sc.Scan(&f.ID, &f.SourceID, &f.SourceLabel, &f.RelPath, &f.Size, &f.Mtime, &f.Fingerprint, &f.Duration, &f.Container, &f.Bitrate,
		&v, &a, &s, &f.TitleID, &season, &episode, &f.Label, &f.MatchSource, &f.MissingSince, &f.AddedAt); err != nil {
		return nil, err
	}
	f.Video, f.Audio, f.Subs = json.RawMessage(v), json.RawMessage(a), json.RawMessage(s)
	if season.Valid {
		n := int(season.Int64)
		f.Season = &n
	}
	if episode.Valid {
		n := int(episode.Int64)
		f.Episode = &n
	}
	return &f, nil
}

func (db *DB) queryFiles(where string, args ...any) ([]*File, error) {
	rows, err := db.Query(`SELECT `+fileCols+` FROM files f JOIN sources s ON s.id=f.source_id WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (db *DB) File(id string) (*File, error) {
	fs, err := db.queryFiles(`f.id=?`, id)
	if err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		return nil, ErrNotFound
	}
	return fs[0], nil
}

func (db *DB) FilesOfSource(sourceID string) ([]*File, error) {
	return db.queryFiles(`f.source_id=?`, sourceID)
}

func (db *DB) FilesOfTitle(titleID string) ([]*File, error) {
	return db.queryFiles(`f.title_id=? AND f.missing_since IS NULL ORDER BY f.season, f.episode, f.relpath`, titleID)
}

func (db *DB) FilesByFingerprint(fp string) ([]*File, error) {
	return db.queryFiles(`f.fingerprint=?`, fp)
}

func (db *DB) UnmatchedFiles() ([]*File, error) {
	return db.queryFiles(`(f.title_id IS NULL OR f.match_source='filename') AND f.missing_since IS NULL`)
}

// SaveFile inserta o actualiza el archivo (clave: ID).
func (db *DB) SaveFile(f *File) error {
	if f.AddedAt == "" {
		f.AddedAt = Now()
	}
	for _, p := range []*json.RawMessage{&f.Video, &f.Audio, &f.Subs} {
		if len(*p) == 0 {
			*p = json.RawMessage("null")
		}
	}
	var title any
	if f.TitleID != "" {
		title = f.TitleID
	}
	var missing any
	if f.MissingSince != "" {
		missing = f.MissingSince
	}
	_, err := db.Exec(`INSERT INTO files(id,source_id,relpath,size,mtime,fingerprint,duration,container,bitrate,video,audio,subs,title_id,season,episode,label,match_source,missing_since,added_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET relpath=excluded.relpath,size=excluded.size,mtime=excluded.mtime,fingerprint=excluded.fingerprint,
		duration=excluded.duration,container=excluded.container,bitrate=excluded.bitrate,video=excluded.video,audio=excluded.audio,subs=excluded.subs,
		title_id=excluded.title_id,season=excluded.season,episode=excluded.episode,label=excluded.label,match_source=excluded.match_source,missing_since=excluded.missing_since`,
		f.ID, f.SourceID, f.RelPath, f.Size, f.Mtime, f.Fingerprint, f.Duration, f.Container, f.Bitrate, string(f.Video), string(f.Audio), string(f.Subs),
		title, f.Season, f.Episode, f.Label, f.MatchSource, missing, f.AddedAt)
	return err
}

// DeleteOrphanTitles borra las fichas creadas a partir del nombre de archivo que se han quedado sin archivos.
// Las de la semilla, TMDB o manuales se conservan aunque su disco esté desconectado.
func (db *DB) DeleteOrphanTitles() (int64, error) {
	r, err := db.Exec(`DELETE FROM titles WHERE match_source='filename' AND locked=0 AND NOT EXISTS (SELECT 1 FROM files f WHERE f.title_id=titles.id)`)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

type Progress struct {
	FileID    string  `json:"fileId"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	Watched   bool    `json:"watched"`
	UpdatedAt string  `json:"updatedAt"`
}

func (db *DB) SaveProgress(p Progress) error {
	w := 0
	if p.Watched {
		w = 1
	}
	_, err := db.Exec(`INSERT INTO progress(file_id,position,duration,watched,updated_at) VALUES(?,?,?,?,?)
		ON CONFLICT(file_id) DO UPDATE SET position=excluded.position,duration=excluded.duration,watched=excluded.watched,updated_at=excluded.updated_at`,
		p.FileID, p.Position, p.Duration, w, Now())
	return err
}

func (db *DB) ProgressOf(fileID string) (*Progress, error) {
	var p Progress
	var w int
	err := db.QueryRow(`SELECT file_id,position,duration,watched,updated_at FROM progress WHERE file_id=?`, fileID).Scan(&p.FileID, &p.Position, &p.Duration, &w, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	p.Watched = w != 0
	return &p, err
}
