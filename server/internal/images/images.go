// Package images prepara pósteres y fondos para la TV: los descarga (o copia) una vez,
// los redimensiona al ancho pedido y los guarda en caché como JPEG.
package images

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Widths admitidos: póster rejilla/ficha y fondo 720p/1080p.
var Widths = map[int]bool{185: true, 300: true, 600: true, 1280: true, 1920: true}

type Cache struct {
	Dir    string
	Client *http.Client
	mu     sync.Mutex
	locks  map[string]*sync.Mutex
}

func New(dir string) *Cache {
	return &Cache{Dir: dir, Client: &http.Client{Timeout: 30 * time.Second}, locks: map[string]*sync.Mutex{}}
}

func (c *Cache) lock(key string) func() {
	c.mu.Lock()
	l := c.locks[key]
	if l == nil {
		l = &sync.Mutex{}
		c.locks[key] = l
	}
	c.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// Get devuelve la ruta del JPEG de ancho w para ref (URL https o ruta local).
func (c *Cache) Get(ctx context.Context, ref string, w int) (string, error) {
	if !Widths[w] {
		return "", fmt.Errorf("ancho %d no admitido", w)
	}
	sum := sha1.Sum([]byte(ref))
	key := hex.EncodeToString(sum[:8])
	out := filepath.Join(c.Dir, "cache", fmt.Sprintf("%s-%d.jpg", key, w))
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	defer c.lock(key)()
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	src, err := c.original(ctx, ref, key)
	if err != nil {
		return "", err
	}
	fh, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	img, _, err := image.Decode(fh)
	if err != nil {
		return "", fmt.Errorf("imagen ilegible: %w", err)
	}
	if b := img.Bounds(); b.Dx() > w {
		h := b.Dy() * w / b.Dx()
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		img = dst
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	tmp := out + ".tmp"
	of, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if err := jpeg.Encode(of, img, &jpeg.Options{Quality: 82}); err != nil {
		of.Close()
		return "", err
	}
	if err := of.Close(); err != nil {
		return "", err
	}
	return out, os.Rename(tmp, out)
}

func (c *Cache) original(ctx context.Context, ref, key string) (string, error) {
	if !strings.HasPrefix(ref, "https://") && !strings.HasPrefix(ref, "http://") {
		return ref, nil
	}
	dst := filepath.Join(c.Dir, "orig", key)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("descarga %s: %s", ref, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 25<<20)); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return dst, os.Rename(tmp, dst)
}
