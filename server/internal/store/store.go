// Package store guarda el catálogo de YourFlix en SQLite (modernc, sin cgo).
//
// Identificadores: opacos y estables. Un archivo conserva su ID aunque cambie de
// ruta dentro de su fuente (se re-enlaza por huella), así no se pierde el progreso.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS sources (
	id TEXT PRIMARY KEY,
	label TEXT NOT NULL,
	path TEXT NOT NULL UNIQUE,
	kind TEXT NOT NULL DEFAULT 'auto',          -- auto | movies | series
	excludes TEXT NOT NULL DEFAULT '[]',        -- JSON: patrones glob
	state TEXT NOT NULL DEFAULT 'active',       -- active | paused | offline
	created_at TEXT NOT NULL,
	last_scan_at TEXT
);
CREATE TABLE IF NOT EXISTS titles (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,                         -- movie | series
	title TEXT NOT NULL,
	original TEXT NOT NULL DEFAULT '',
	year TEXT NOT NULL DEFAULT '',
	tmdb_id INTEGER,
	imdb_id TEXT NOT NULL DEFAULT '',
	meta TEXT NOT NULL DEFAULT '{}',            -- JSON: ficha (sinopsis, géneros, reparto…)
	poster TEXT NOT NULL DEFAULT '',            -- URL https o ruta local de la imagen
	backdrop TEXT NOT NULL DEFAULT '',
	match_source TEXT NOT NULL,                 -- seed | tmdb | manual | filename
	norm TEXT NOT NULL DEFAULT '',              -- título normalizado para emparejar
	norm_original TEXT NOT NULL DEFAULT '',
	locked INTEGER NOT NULL DEFAULT 0,
	added_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS titles_norm ON titles(norm, year);
CREATE INDEX IF NOT EXISTS titles_norm_original ON titles(norm_original);
CREATE TABLE IF NOT EXISTS seed_copies (
	title_id TEXT NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
	bytes INTEGER NOT NULL,
	seconds REAL,
	season INTEGER,
	episode INTEGER,
	label TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS seed_copies_bytes ON seed_copies(bytes);
CREATE TABLE IF NOT EXISTS files (
	id TEXT PRIMARY KEY,
	source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
	relpath TEXT NOT NULL,
	size INTEGER NOT NULL,
	mtime INTEGER NOT NULL,
	fingerprint TEXT NOT NULL,
	duration REAL NOT NULL DEFAULT 0,
	container TEXT NOT NULL DEFAULT '',
	bitrate INTEGER NOT NULL DEFAULT 0,
	video TEXT NOT NULL DEFAULT '{}',           -- JSON probe.Video
	audio TEXT NOT NULL DEFAULT '[]',           -- JSON []probe.Audio
	subs TEXT NOT NULL DEFAULT '[]',            -- JSON []probe.Subtitle
	title_id TEXT REFERENCES titles(id) ON DELETE SET NULL,
	season INTEGER,
	episode INTEGER,
	label TEXT NOT NULL DEFAULT '',
	match_source TEXT NOT NULL DEFAULT '',
	missing_since TEXT,
	added_at TEXT NOT NULL,
	UNIQUE(source_id, relpath)
);
CREATE INDEX IF NOT EXISTS files_title ON files(title_id);
CREATE INDEX IF NOT EXISTS files_fp ON files(fingerprint);
CREATE TABLE IF NOT EXISTS progress (
	file_id TEXT PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
	position REAL NOT NULL,
	duration REAL NOT NULL,
	watched INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tmdb_cache (url TEXT PRIMARY KEY, body BLOB NOT NULL, fetched_at TEXT NOT NULL);
`

// Open abre (o crea) la base de datos y aplica el esquema.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // un único escritor: evita SQLITE_BUSY entre escaneo y API
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("esquema: %w", err)
	}
	return &DB{db}, nil
}

// NewID devuelve 12 caracteres hexadecimales aleatorios.
func NewID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// TMDBCache guarda respuestas de TMDB (sin credenciales en la clave) para no repetir consultas.
type TMDBCache struct {
	DB  *DB
	TTL time.Duration
}

func (c TMDBCache) Get(key string) ([]byte, bool) {
	var body []byte
	var at string
	if err := c.DB.QueryRow(`SELECT body,fetched_at FROM tmdb_cache WHERE url=?`, key).Scan(&body, &at); err != nil {
		return nil, false
	}
	if t, err := time.Parse(time.RFC3339, at); err != nil || (c.TTL > 0 && time.Since(t) > c.TTL) {
		return nil, false
	}
	return body, true
}

func (c TMDBCache) Put(key string, body []byte) {
	c.DB.Exec(`INSERT INTO tmdb_cache(url,body,fetched_at) VALUES(?,?,?) ON CONFLICT(url) DO UPDATE SET body=excluded.body,fetched_at=excluded.fetched_at`, key, body, Now())
}
