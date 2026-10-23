package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
)

type Source struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Path       string   `json:"path"`
	Kind       string   `json:"kind"`
	Excludes   []string `json:"excludes"`
	State      string   `json:"state"`
	CreatedAt  string   `json:"createdAt"`
	LastScanAt string   `json:"lastScanAt,omitempty"`
	Files      int      `json:"files"`
}

var ErrNotFound = errors.New("no encontrado")

func (db *DB) AddSource(label, path, kind string, excludes []string) (*Source, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "", "auto":
		kind = "auto"
	case "movies", "series":
	default:
		return nil, fmt.Errorf("tipo %q no válido (auto, movies, series)", kind)
	}
	if label == "" {
		label = filepath.Base(abs)
	}
	if excludes == nil {
		excludes = []string{}
	}
	ex, _ := json.Marshal(excludes)
	s := &Source{ID: NewID(), Label: label, Path: abs, Kind: kind, Excludes: excludes, State: "active", CreatedAt: Now()}
	_, err = db.Exec(`INSERT INTO sources(id,label,path,kind,excludes,state,created_at) VALUES(?,?,?,?,?,?,?)`,
		s.ID, s.Label, s.Path, s.Kind, string(ex), s.State, s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("no se pudo añadir %s: %w", abs, err)
	}
	return s, nil
}

func (db *DB) Sources() ([]Source, error) {
	rows, err := db.Query(`SELECT s.id,s.label,s.path,s.kind,s.excludes,s.state,s.created_at,COALESCE(s.last_scan_at,''),
		(SELECT COUNT(*) FROM files f WHERE f.source_id=s.id AND f.missing_since IS NULL)
		FROM sources s ORDER BY s.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var s Source
		var ex string
		if err := rows.Scan(&s.ID, &s.Label, &s.Path, &s.Kind, &ex, &s.State, &s.CreatedAt, &s.LastScanAt, &s.Files); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(ex), &s.Excludes)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Source busca por ID o, si no coincide, por etiqueta exacta (cómodo en la CLI).
func (db *DB) Source(idOrLabel string) (*Source, error) {
	all, err := db.Sources()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == idOrLabel || all[i].Label == idOrLabel {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("fuente %q: %w", idOrLabel, ErrNotFound)
}

func (db *DB) SetSourceState(id, state string) error {
	return checkOne(db.Exec(`UPDATE sources SET state=? WHERE id=?`, state, id))
}

func (db *DB) MarkScanned(id string) error {
	_, err := db.Exec(`UPDATE sources SET last_scan_at=? WHERE id=?`, Now(), id)
	return err
}

// RemoveSource saca la fuente del catálogo (y sus archivos de la base de datos).
// Nunca toca el disco.
func (db *DB) RemoveSource(id string) error {
	return checkOne(db.Exec(`DELETE FROM sources WHERE id=?`, id))
}

func checkOne(r sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
