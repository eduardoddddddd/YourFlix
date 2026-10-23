# YourFlix

Servidor de películas y series para casa (Asus ROG Ally con Windows 11, o Mac) y app nativa para la Samsung The Serif (Tizen 6.5). El servidor reúne varias carpetas elegidas por el usuario, identifica cada vídeo con fichas y carátulas y lo sirve por la red local; la tele lo reproduce con Samsung AVPlay.

- [PLAN.md](PLAN.md): arquitectura, fases y decisiones.
- [AGENTS.md](AGENTS.md): reglas para cualquier modelo que trabaje en el repo.

## Uso del servidor

```bash
cd server && go build -o ../private/yourflix ./cmd/yourflix && cd ..
Y="private/yourflix --data private/data"
$Y seed import RUTA/cine-t7-catalogo            # carga inicial: fichas y portadas del T7
$Y source add "E:\pelis" --label "Samsung T7"   # tantas fuentes como quieras
$Y source add "F:\" --label "USB Patriot" --exclude "*sample*"
$Y source scan --all
$Y titles                                        # catálogo; --pending: sin ficha verificada
$Y serve                                         # API en :8787 (re-escanea cada 30 min)
```

Compilar para el ROG Ally: `cd server && GOOS=windows GOARCH=amd64 go build -o yourflix.exe ./cmd/yourflix`; poner `ffprobe.exe` junto al ejecutable. API: [docs/API.md](docs/API.md).

**macOS:** si una fuente está en Descargas, Documentos o un disco externo, el sistema pide permiso de privacidad al proceso que lanza el servidor. Hasta concederlo, la fuente aparece como desconectada (el servidor no se bloquea).

## Estado

**23-10-2026 — fase 1A hecha y carga inicial del T7 (parte de 1B).** Servidor `server/` 0.1.0:

- Hecho: SQLite sin cgo; fuentes múltiples por CLI (`add/list/scan/pause/resume/remove`); escaneo con huella (tamaño + 1 MiB inicial y final), ffprobe en paralelo, archivos movidos (conservan ID), ausentes y fuentes desconectadas (con tiempo límite de 10 s); análisis de nombres (película/serie, año, `S01E02`, `1x02`, `Cap.105`); **semilla T7** (255 fichas, 499 copias medidas, 259 portadas) con emparejado por bytes, por título + año y por huella entre fuentes (un título, varias copias); compatibilidad con el perfil de la TV; API `/api/v1` (títulos, ficha, imágenes redimensionadas y cacheadas, streaming con Range, progreso, fuentes); binario Windows x64.
- Probado: `go test ./...` (análisis, ffprobe con salida real, compatibilidad, escaneo, semilla, API con Range y solo-LAN). En la TV real: búsqueda → ficha → reproducción por la API nueva de «No Hay Salida» (114 min), salto al 60 % correcto. Real con dos fuentes en el Mac (pruebas + aMule `incoming`).
- No probado: el T7 conectado (emparejado por bytes con los 499 archivos reales), servidor en el ROG Ally.
- Pendiente: **TMDB** para lo que no está en la semilla (resto de 1B), `yourflix.toml` (de momento opciones y variables de entorno), web de admin (fase 5), cliente TV (fase 2).
- Siguiente paso: completar 1B (TMDB) y empezar la fase 2 contra esta API.

### Fase 0 (23-10-2026) — G0 superada ([ADR-001](docs/DECISIONES.md))

- Hecho: plan v1; servidor de prueba `tools/spike-server` (Go), app de prueba `tools/spike-avplay` (`Z8YFSPIKE1.YourFlixSpike`, instalada en la TV), generador de vídeos `tools/spike-media`, monitor CDP `tools/tv-monitor`, perfil `profiles/samsung-ls01b-2022.json`.
- **Probado en la TV real** ([resultados](docs/PRUEBAS-TV.md)): reproducción directa por HTTP Range de MKV/MP4/AVI, H.264/HEVC 10 bits/XviD/4K, saltos en archivos de 2,4 y 7,6 GB, cambio de audio, SRT como texto; 0 cuelgues. DTS suena mudo (sin error); PGS no llega.
- No probado: HDR real, servidor en el ROG Ally, DTS con otra pista compatible.
- Pendiente de decidir: Go (lo usado en la fase 0) o Python para el servidor, perfiles de usuario, modelo de Ally y dock; completar el perfil de la TV con las memorias de Codex.
- Siguiente paso: **fase 1A** (núcleo del servidor) y, en paralelo, **1B** (semilla T7 + TMDB).
