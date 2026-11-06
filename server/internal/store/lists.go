package store

import (
	"database/sql"
	"strconv"
	"strings"
)

// Listas del usuario: «Favoritos» y «Pendientes de ver», comunes a la TV y a la web (un
// único perfil, como el progreso). Un elemento es un título entero (season = episode = -1)
// o un capítulo de una serie. Se guardan por título y número de capítulo, no por archivo:
// sobreviven a sustituir el archivo por una copia mejor o a un disco desconectado.
const (
	ListFavorites = "favorites"
	ListWatchlist = "watchlist"
)

func ValidList(name string) bool { return name == ListFavorites || name == ListWatchlist }

type ListItem struct {
	List    string `json:"list"`
	TitleID string `json:"titleId"`
	Season  *int   `json:"season,omitempty"`
	Episode *int   `json:"episode,omitempty"`
	AddedAt string `json:"addedAt"`
}

// clave: -1/-1 para el título entero.
func listKey(season, episode *int) (int, int) {
	if season == nil || episode == nil {
		return -1, -1
	}
	return *season, *episode
}

// AddToList: idempotente (volver a añadir no cambia la fecha).
func (db *DB) AddToList(list, titleID string, season, episode *int) error {
	s, e := listKey(season, episode)
	_, err := db.Exec(`INSERT INTO lists(list,title_id,season,episode,added_at) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`, list, titleID, s, e, Now())
	return err
}

func (db *DB) RemoveFromList(list, titleID string, season, episode *int) error {
	s, e := listKey(season, episode)
	_, err := db.Exec(`DELETE FROM lists WHERE list=? AND title_id=? AND season=? AND episode=?`, list, titleID, s, e)
	return err
}

// ListItems: lo más reciente primero.
func (db *DB) ListItems(list string) ([]ListItem, error) {
	rows, err := db.Query(`SELECT list,title_id,season,episode,added_at FROM lists WHERE list=? ORDER BY added_at DESC, rowid DESC`, list)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListItem{}
	for rows.Next() {
		var it ListItem
		var s, e int
		if err := rows.Scan(&it.List, &it.TitleID, &s, &e, &it.AddedAt); err != nil {
			return nil, err
		}
		if s >= 0 && e >= 0 {
			it.Season, it.Episode = &s, &e
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// TitleLists: en qué listas está el título entero y cada capítulo («T×E» → listas).
func (db *DB) TitleLists(titleID string) ([]string, map[string][]string, error) {
	rows, err := db.Query(`SELECT list,season,episode FROM lists WHERE title_id=? ORDER BY list`, titleID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	whole := []string{}
	eps := map[string][]string{}
	for rows.Next() {
		var l string
		var s, e int
		if err := rows.Scan(&l, &s, &e); err != nil {
			return nil, nil, err
		}
		if s < 0 {
			whole = append(whole, l)
		} else {
			k := strconv.Itoa(s) + "x" + strconv.Itoa(e)
			eps[k] = append(eps[k], l)
		}
	}
	return whole, eps, rows.Err()
}

// ClearWatched: lo que se termina de ver sale de «Pendientes de ver» (la película o ese
// capítulo; una serie entera se queda hasta que la quite el usuario). Favoritos no cambia.
func (db *DB) ClearWatched(fileID string) error {
	var titleID, kind string
	var season, episode sql.NullInt64
	err := db.QueryRow(`SELECT f.title_id,t.kind,f.season,f.episode FROM files f JOIN titles t ON t.id=f.title_id WHERE f.id=?`, fileID).Scan(&titleID, &kind, &season, &episode)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if kind != "series" {
		return db.RemoveFromList(ListWatchlist, titleID, nil, nil)
	}
	if season.Valid && episode.Valid {
		s, e := int(season.Int64), int(episode.Int64)
		return db.RemoveFromList(ListWatchlist, titleID, &s, &e)
	}
	return nil
}

func splitLists(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return nil
	}
	return strings.Split(s.String, ",")
}
