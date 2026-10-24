package main

// Administración web en /admin/ (fase 5, internal/admin). Para activarla, main.go necesita:
//
//	// en serve, justo después de crear hs:
//	hs.Handler = a.withAdmin(hs.Handler, prof)
//
//	// en el switch de main:
//	case "admin":
//		err = a.adminCmd(args[1:])
//
// Para que un escaneo de la admin no coincida con el re-escaneo periódico, scanOne
// debería empezar con «scanMu.Lock(); defer scanMu.Unlock()».

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/eduardoddddddd/YourFlix/server/internal/admin"
	"github.com/eduardoddddddd/YourFlix/server/internal/compat"
)

// scanMu: un escaneo a la vez entre la admin y el re-escaneo periódico de serve.
var scanMu sync.Mutex

// withAdmin monta /admin/ delante del resto del servidor y copia el registro a la página
// de estado (sin el token de TMDB).
func (a *app) withAdmin(rest http.Handler, prof *compat.Profile) http.Handler {
	ring := admin.NewRing(500, a.tmdb.Token, a.tmdb.APIKey)
	log.SetOutput(io.MultiWriter(os.Stderr, ring))
	if pw := os.Getenv("YOURFLIX_ADMIN_PASSWORD"); pw != "" && !admin.HasPassword(a.db) {
		if err := admin.SetPassword(a.db, pw); err != nil {
			log.Printf("YOURFLIX_ADMIN_PASSWORD: %v", err)
		} else {
			log.Printf("Contraseña de administración guardada (desde YOURFLIX_ADMIN_PASSWORD)")
		}
	}
	web := admin.FindWebDir(os.Getenv("YOURFLIX_ADMIN_WEB"), exeRel("admin"), exeRel("../admin"), "admin")
	adm, err := admin.New(admin.Config{DB: a.db, TMDB: a.tmdb, Profile: prof, Version: version, DataDir: a.dataDir,
		FFprobe: a.ffprobe, FFmpeg: a.ffmpeg, WebDir: web, Log: log.Default(), Ring: ring, Lock: &scanMu})
	if err != nil {
		log.Printf("administración desactivada: %v", err)
		return rest
	}
	if web == "" {
		log.Printf("Administración: no encuentro la carpeta admin/ (YOURFLIX_ADMIN_WEB)")
	} else {
		log.Printf("Administración en /admin/ · web en %s", web)
	}
	if !admin.HasPassword(a.db) {
		log.Printf("Administración sin contraseña: solo se entra desde este equipo (http://localhost:8787/admin/). Defínela allí o con «yourflix admin password»")
	}
	mux := http.NewServeMux()
	mux.Handle("/admin/", adm)
	mux.Handle("/", rest)
	return mux
}

func exeRel(rel string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return filepath.Join(filepath.Dir(exe), filepath.FromSlash(rel))
}

// adminCmd: yourflix admin password (lee la contraseña nueva de la entrada estándar).
func (a *app) adminCmd(args []string) error {
	if len(args) != 1 || args[0] != "password" {
		return errors.New("uso: yourflix admin password   (lee la contraseña nueva de la entrada estándar)")
	}
	fmt.Fprintf(os.Stderr, "Contraseña nueva de administración (mínimo %d caracteres; se verá al escribirla): ", admin.MinPassword)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return fmt.Errorf("no se pudo leer la contraseña: %w", err)
	}
	if err := admin.SetPassword(a.db, strings.TrimRight(line, "\r\n")); err != nil {
		return err
	}
	fmt.Println("Contraseña de administración guardada (con hash). Las sesiones abiertas con la anterior se cierran.")
	return nil
}
