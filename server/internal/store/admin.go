package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Consultas de la web de administración (fase 5). La tabla de escaneos es propia de la
// admin y se crea con InitAdmin, sin tocar el esquema común.

const adminSchema = `
CREATE TABLE IF NOT EXISTS scan_runs (
	id TEXT PRIMARY KEY,
	source_id TEXT NOT NULL,
	label TEXT NOT NULL,
	started_at TEXT NOT NULL,
	seconds REAL NOT NULL,
	report TEXT NOT NULL DEFAULT '{}',          -- JSON scan.Report
	error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS scan_runs_started ON scan_runs(started_at);
`

func (db *DB) InitAdmin() error {
	if _, err := db.Exec(adminSchema); err != nil {
		return fmt.Errorf("esquema de administración: %w", err)
	}
	return nil
}

// Setting devuelve el valor de la clave (ok=false si no existe).
func (db *DB) Setting(key string) (string, bool, error) {
	var v string
	err := db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return v, err == nil, err
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// UpdateSource cambia etiqueta, tipo y exclusiones. La ruta no se cambia: para otra
// carpeta se añade otra fuente (así el ID y el progreso de los archivos no se mezclan).
func (db *DB) UpdateSource(id, label, kind string, excludes []string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return fmt.Errorf("la etiqueta no puede quedar vacía")
	}
	switch kind {
	case "", "auto":
		kind = "auto"
	case "movies", "series":
	default:
		return fmt.Errorf("tipo %q no válido (auto, movies, series)", kind)
	}
	clean := []string{}
	for _, e := range excludes {
		if e = strings.TrimSpace(e); e != "" {
			clean = append(clean, e)
		}
	}
	ex, _ := json.Marshal(clean)
	return checkOne(db.Exec(`UPDATE sources SET label=?, kind=?, excludes=? WHERE id=?`, label, kind, string(ex), id))
}

type ScanRun struct {
	ID        string          `json:"id"`
	SourceID  string          `json:"sourceId"`
	Label     string          `json:"label"`
	StartedAt string          `json:"startedAt"`
	Seconds   float64         `json:"seconds"`
	Report    json.RawMessage `json:"report"`
	Error     string          `json:"error,omitempty"`
}

// AddScanRun guarda el resultado de un escaneo lanzado desde la admin (conserva los 200 últimos).
func (db *DB) AddScanRun(r ScanRun) error {
	if r.ID == "" {
		r.ID = NewID()
	}
	if len(r.Report) == 0 {
		r.Report = json.RawMessage("{}")
	}
	if _, err := db.Exec(`INSERT INTO scan_runs(id,source_id,label,started_at,seconds,report,error) VALUES(?,?,?,?,?,?,?)`,
		r.ID, r.SourceID, r.Label, r.StartedAt, r.Seconds, string(r.Report), r.Error); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM scan_runs WHERE id NOT IN (SELECT id FROM scan_runs ORDER BY started_at DESC LIMIT 200)`)
	return err
}

func (db *DB) RecentScanRuns(n int) ([]ScanRun, error) {
	rows, err := db.Query(`SELECT id,source_id,label,started_at,seconds,report,error FROM scan_runs ORDER BY started_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ScanRun{}
	for rows.Next() {
		var r ScanRun
		var rep string
		if err := rows.Scan(&r.ID, &r.SourceID, &r.Label, &r.StartedAt, &r.Seconds, &rep, &r.Error); err != nil {
			return nil, err
		}
		r.Report = json.RawMessage(rep)
		out = append(out, r)
	}
	return out, rows.Err()
}

// AvailableFiles: todos los archivos presentes (para el resumen de compatibilidad).
func (db *DB) AvailableFiles() ([]*File, error) {
	return db.queryFiles(`f.missing_since IS NULL ORDER BY s.label, f.relpath`)
}

type TitleName struct {
	ID, Kind, Title, Year string
}

// TitleNames: nombre y año de cada ficha, por ID.
func (db *DB) TitleNames() (map[string]TitleName, error) {
	rows, err := db.Query(`SELECT id,kind,title,year FROM titles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]TitleName{}
	for rows.Next() {
		var t TitleName
		if err := rows.Scan(&t.ID, &t.Kind, &t.Title, &t.Year); err != nil {
			return nil, err
		}
		out[t.ID] = t
	}
	return out, rows.Err()
}
