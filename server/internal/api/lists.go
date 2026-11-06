package api

import (
	"net/http"
	"strconv"

	"github.com/eduardoddddddd/YourFlix/server/internal/store"
)

// Listas «Favoritos» y «Pendientes de ver» (store/lists.go). Un capítulo se indica con
// ?season=&episode=; sin ellos, el título entero.

// episodeParams: (nil, nil) si no hay capítulo; error si falta uno de los dos o no es un número.
func episodeParams(r *http.Request) (*int, *int, bool) {
	q := r.URL.Query()
	ss, es := q.Get("season"), q.Get("episode")
	if ss == "" && es == "" {
		return nil, nil, true
	}
	s, err1 := strconv.Atoi(ss)
	e, err2 := strconv.Atoi(es)
	if err1 != nil || err2 != nil || s < 0 || e < 0 {
		return nil, nil, false
	}
	return &s, &e, true
}

func (s *Server) changeList(w http.ResponseWriter, r *http.Request) {
	list := r.PathValue("list")
	if !store.ValidList(list) {
		http.Error(w, "lista desconocida", http.StatusNotFound)
		return
	}
	season, episode, ok := episodeParams(r)
	if !ok {
		http.Error(w, "season y episode van juntos y son números", http.StatusBadRequest)
		return
	}
	t, err := s.DB.Title(r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if season != nil && t.Kind != "series" {
		http.Error(w, "solo las series tienen capítulos", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodDelete {
		err = s.DB.RemoveFromList(list, t.ID, season, episode)
	} else {
		err = s.DB.AddToList(list, t.ID, season, episode)
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listEpisode: un capítulo de una lista, con lo necesario para pintar la tarjeta y reproducirlo.
type listEpisode struct {
	TitleID  string  `json:"titleId"`
	FileID   string  `json:"fileId"`
	Title    string  `json:"title"`
	Year     string  `json:"year,omitempty"`
	Season   int     `json:"season"`
	Episode  int     `json:"episode"`
	Name     string  `json:"episodeName,omitempty"`
	Poster   string  `json:"poster,omitempty"`
	Backdrop string  `json:"backdrop,omitempty"`
	Still    string  `json:"still,omitempty"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Watched  bool    `json:"watched,omitempty"`
	AddedAt  string  `json:"addedAt"`
}

type userList struct {
	Titles   []store.TitleSummary `json:"titles"`
	Episodes []listEpisode        `json:"episodes"`
}

// lists: GET /lists. Las dos listas, lo último añadido primero. Solo lo que tiene algún
// archivo disponible (como el resto de listas); lo demás sigue guardado y vuelve al
// reconectar el disco.
func (s *Server) lists(w http.ResponseWriter, r *http.Request) {
	out := map[string]userList{}
	for _, name := range []string{store.ListFavorites, store.ListWatchlist} {
		items, err := s.DB.ListItems(name)
		if err != nil {
			s.fail(w, err)
			return
		}
		lo := userList{Titles: []store.TitleSummary{}, Episodes: []listEpisode{}}
		ids := []string{}
		for _, it := range items {
			if it.Season == nil {
				ids = append(ids, it.TitleID)
				continue
			}
			if e, ok := s.listEpisode(it); ok {
				lo.Episodes = append(lo.Episodes, e)
			}
		}
		if len(ids) > 0 {
			ts, _, err := s.DB.ListTitles(store.TitleQuery{IDs: ids, Sort: "ids", Limit: 500})
			if err != nil {
				s.fail(w, err)
				return
			}
			for i := range ts {
				ts[i].Poster = imgURL(ts[i].ID, "poster", ts[i].Poster, 300)
				ts[i].Backdrop = imgURL(ts[i].ID, "backdrop", ts[i].Backdrop, 1280)
			}
			lo.Titles = ts
		}
		out[name] = lo
	}
	writeJSON(w, out)
}

func (s *Server) listEpisode(it store.ListItem) (listEpisode, bool) {
	t, err := s.DB.Title(it.TitleID)
	if err != nil {
		return listEpisode{}, false
	}
	files, err := s.DB.FilesOfTitle(t.ID)
	if err != nil {
		return listEpisode{}, false
	}
	// La copia de ese capítulo: la que tenga progreso a medias o, si no, la primera.
	var f *store.File
	var p *store.Progress
	for _, x := range files {
		if x.Season == nil || x.Episode == nil || *x.Season != *it.Season || *x.Episode != *it.Episode {
			continue
		}
		xp, _ := s.DB.ProgressOf(x.ID)
		if f == nil || (xp != nil && p == nil) {
			f, p = x, xp
		}
	}
	if f == nil {
		return listEpisode{}, false
	}
	e := listEpisode{TitleID: t.ID, FileID: f.ID, Title: t.Title, Year: t.Year, Season: *it.Season, Episode: *it.Episode,
		Poster: imgURL(t.ID, "poster", t.Poster, 300), Backdrop: imgURL(t.ID, "backdrop", t.Backdrop, 1280),
		Duration: f.Duration, AddedAt: it.AddedAt}
	if p != nil {
		e.Position, e.Watched = p.Position, p.Watched
		if p.Duration > 0 {
			e.Duration = p.Duration
		}
	}
	if m, ok := episodes(t.Meta)[epKey(f)]; ok {
		e.Name = m.Name
		if m.Still != "" {
			e.Still = "/api/v1/files/" + f.ID + "/still?w=300"
		}
	}
	return e, true
}
