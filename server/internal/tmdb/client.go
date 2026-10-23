// Package tmdb consulta The Movie Database para identificar lo que no está en la semilla.
// Credenciales: token de lectura (v4, «Bearer») o clave v3. Nunca se guardan en la caché,
// en los registros ni en la API: la clave de caché es la ruta sin credenciales.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const DefaultBase = "https://api.themoviedb.org/3"
const ImageBase = "https://image.tmdb.org/t/p/"

type Cache interface {
	Get(key string) ([]byte, bool)
	Put(key string, body []byte)
}

type Client struct {
	Token  string // token de lectura v4
	APIKey string // o clave v3
	Base   string
	HTTP   *http.Client
	Cache  Cache

	mu   sync.Mutex
	last time.Time
}

var ErrNoCredentials = errors.New("sin credenciales de TMDB (TMDB_ACCESS_TOKEN, TMDB_API_KEY o --tmdb-token-file)")

func (c *Client) Enabled() bool { return c != nil && (c.Token != "" || c.APIKey != "") }

// wait espacia las peticiones (~20/s, por debajo del límite de TMDB).
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	next := c.last.Add(50 * time.Millisecond)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	c.last = next
	c.mu.Unlock()
	select {
	case <-time.After(time.Until(next)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// get pide path?q (sin credenciales) y decodifica en v. Usa la caché.
func (c *Client) get(ctx context.Context, path string, q url.Values, v any) error {
	if !c.Enabled() {
		return ErrNoCredentials
	}
	key := path + "?" + q.Encode()
	if c.Cache != nil {
		if body, ok := c.Cache.Get(key); ok {
			return json.Unmarshal(body, v)
		}
	}
	base := c.Base
	if base == "" {
		base = DefaultBase
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return err
		}
		qq := url.Values{}
		for k, vs := range q {
			qq[k] = vs
		}
		if c.Token == "" {
			qq.Set("api_key", c.APIKey)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path+"?"+qq.Encode(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		resp, err := hc.Do(req)
		if err != nil {
			var ue *url.Error
			if errors.As(err, &ue) {
				err = ue.Err // el error de url.Error lleva la URL, que con clave v3 incluye api_key
			}
			return fmt.Errorf("TMDB %s: %w", path, err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			return err
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests && attempt < 3:
			wait, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
			select {
			case <-time.After(time.Duration(max(wait, 1)) * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		case resp.StatusCode == http.StatusUnauthorized:
			return errors.New("TMDB rechaza las credenciales (401)")
		case resp.StatusCode == http.StatusNotFound:
			return fmt.Errorf("TMDB %s: no existe", path)
		case resp.StatusCode != http.StatusOK:
			return fmt.Errorf("TMDB %s: %s", path, resp.Status)
		}
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("TMDB %s: respuesta ilegible: %w", path, err)
		}
		if c.Cache != nil {
			c.Cache.Put(key, body)
		}
		return nil
	}
}

type Result struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	Popularity    float64 `json:"popularity"`
	VoteCount     int     `json:"vote_count"`
	PosterPath    string  `json:"poster_path"`
}

func (r Result) DisplayTitle() string { return first(r.Title, r.Name) }
func (r Result) Original() string     { return first(r.OriginalTitle, r.OriginalName) }
func (r Result) Year() string {
	d := first(r.ReleaseDate, r.FirstAirDate)
	if len(d) >= 4 {
		return d[:4]
	}
	return ""
}

// Search busca películas (kind=movie) o series (kind=series) en el idioma dado (es-ES, en-US).
func (c *Client) Search(ctx context.Context, kind, query, year, lang string) ([]Result, error) {
	q := url.Values{"query": {query}, "language": {lang}, "include_adult": {"false"}}
	path := "/search/movie"
	if kind == "series" {
		path = "/search/tv"
		if year != "" {
			q.Set("first_air_date_year", year)
		}
	} else if year != "" {
		q.Set("year", year)
	}
	var out struct {
		Results []Result `json:"results"`
	}
	err := c.get(ctx, path, q, &out)
	return out.Results, err
}

type person struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

type Details struct {
	ID            int64   `json:"id"`
	IMDB          string  `json:"imdb_id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	Tagline       string  `json:"tagline"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	Runtime       int     `json:"runtime"`
	EpisodeRun    []int   `json:"episode_run_time"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	Genres        []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Countries []struct {
		Name string `json:"name"`
	} `json:"production_countries"`
	CreatedBy []person `json:"created_by"`
	Credits   struct {
		Cast []person `json:"cast"`
		Crew []person `json:"crew"`
	} `json:"credits"`
	ExternalIDs struct {
		IMDB string `json:"imdb_id"`
	} `json:"external_ids"`
	NumberOfSeasons int `json:"number_of_seasons"`
}

// Details trae la ficha en español; si no hay sinopsis en español, la completa en inglés.
func (c *Client) Details(ctx context.Context, kind string, id int64) (*Details, error) {
	path := "/movie/" + strconv.FormatInt(id, 10)
	if kind == "series" {
		path = "/tv/" + strconv.FormatInt(id, 10)
	}
	var d Details
	if err := c.get(ctx, path, url.Values{"language": {"es-ES"}, "append_to_response": {"credits,external_ids"}}, &d); err != nil {
		return nil, err
	}
	if d.Overview == "" {
		var en Details
		if c.get(ctx, path, url.Values{"language": {"en-US"}}, &en) == nil {
			d.Overview = en.Overview
			if d.Tagline == "" {
				d.Tagline = en.Tagline
			}
		}
	}
	return &d, nil
}

type Episode struct {
	Season   int    `json:"season_number"`
	Episode  int    `json:"episode_number"`
	Name     string `json:"name"`
	Overview string `json:"overview"`
	Still    string `json:"still_path"`
	AirDate  string `json:"air_date"`
	Runtime  int    `json:"runtime"`
}

func (c *Client) Season(ctx context.Context, id int64, season int) ([]Episode, error) {
	var out struct {
		Episodes []Episode `json:"episodes"`
	}
	err := c.get(ctx, fmt.Sprintf("/tv/%d/season/%d", id, season), url.Values{"language": {"es-ES"}}, &out)
	return out.Episodes, err
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
