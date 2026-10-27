# YourFlix — plan de proyecto

> Servidor de películas y series en un ordenador de casa (Windows o Mac) y cliente nativo para la Samsung The Serif. El repo y la app se llaman **YourFlix** (decidido el 23-10-2026).
>
> Redactado el 23-10-2026 por Claude Opus 5.5 a partir de: `AstroIPTV` (app Tizen con AVPlay, en producción en la TV), `CineT7` local (app Tizen que lee el T7 por USB, julio 2026) y el repo `eduardoddddddd/cine-t7-catalogo` (255 títulos, 499 archivos con mediciones ffprobe, fichas y portadas de TMDB).
>
> Este documento es la **fuente de decisiones**. El estado real de cada fase va en `README.md`; las reglas para cualquier modelo, en `AGENTS.md`.

---

## 1. Objetivo

Ver en la tele, desde el sofá y con el mando, las películas y series de **varias fuentes** (el disco T7, el USB Patriot, carpetas del propio ordenador, otros discos) **sin enchufar nada a la tele**, con una interfaz a la altura del catálogo web de Cine T7: portadas, fondos, fichas, reparto, «Seguir viendo», series por temporadas.

- **Servidor**: un ejecutable que corre en un PC de casa (el **Asus ROG Ally** con Windows 11; también el Mac). El usuario **elige las carpetas** de la biblioteca desde la web de administración o por línea de comandos (§3.5). El servidor escanea los vídeos, los identifica, prepara fichas y carátulas, y los **sirve por la red local** a la tele.
- **Carga inicial**: la colección del T7 entra con las fichas y portadas **ya verificadas** en `cine-t7-catalogo` (sin volver a consultar nada). Solo lo que se añada después se identifica con **TMDB** (§3.7).
- **Cliente TV**: app Tizen instalada en la The Serif (igual que Astro IPTV) que navega el catálogo y reproduce con **Samsung AVPlay**.
- **Formato**: se sirve el archivo **tal cual** cuando la tele lo admite (la gran mayoría). Lo que no admite se marca y se **optimiza una sola vez** en el servidor (copia compatible), en vez de transcodificar en tiempo real.

### Lo que NO es (v1)

- No es un Plex/Jellyfin: sin usuarios remotos, sin acceso desde fuera de casa, sin transcodificación en tiempo real para la TV (la web sí convierte en vivo: ADR-006), sin DRM, sin descargas.
- No se publica nada en Internet. El catálogo público `cine-t7-catalogo` sigue siendo independiente; YourFlix solo **lee** sus datos como semilla.

### Por qué no usar Jellyfin directamente

Es la alternativa razonable y conviene decirlo: Jellyfin hace casi todo esto y tiene cliente Tizen (que también hay que instalar en modo desarrollador). Se elige un desarrollo propio porque: (1) ya tenemos la cadena de firma/instalación y el conocimiento de AVPlay de esta tele concreta, incluidos sus cuelgues; (2) el diseño de Cine T7 es propio y no encaja en las pieles de Jellyfin; (3) la colección es pequeña (≈500 archivos) y casi toda reproducible en directo, así que el 80 % de la complejidad de Jellyfin (transcodificación en vivo) sobra; (4) control total de datos y de cómo se integra con `cine-t7-catalogo`. Si en la fase 0 AVPlay no reprodujera MKV por HTTP de forma fiable, **se reevalúa** esta decisión (ver §10, puerta G0).

---

## 2. Lo que ya sabemos (no re-descubrir)

### Hardware

| Elemento | Dato |
|---|---|
| TV | Samsung The Serif `QE43LS01BBUXXC` (modelo 2022), Tizen 6.5, panel 3840×2160, **conectada por Wi-Fi** |
| Motor web | Chromium **M85** (no `inset`, no `aspect-ratio`; comprobar todo en la tabla de Samsung) |
| SDB | `192.168.1.4:26101`; target `QE43LS01BBUXXC` (no la IP) para `-t` |
| Mac | Apple Silicon, `192.168.1.3`; Tizen Studio en `~/tizen-studio` (CLI 2.5.25, SDB 4.2.36), Go 1.26, ffmpeg/ffprobe Homebrew |
| Firma | Perfil Samsung `CineT7`, caduca **18-07-2027**. Nunca copiar certificados ni contraseñas al repo |
| Servidor | **Asus ROG Ally** (Windows 11, AMD Z1 x64, Wi-Fi 6E, **sin puerto Ethernet**, USB-C compartido con la carga). Ver §3.6. **IP actual `192.168.1.13` (Wi-Fi, reservada en el DHCP); con dock por cable tendrá otra IP fija (la dará el usuario).** Biblioteca copiada a `C:\YourFlix\T7` y `C:\YourFlix\Patriot` (ADR-005); relevo en `docs/ENTORNO-ALLY.md` |
| TMDB | Clave y token de lectura del usuario en Joplin (nota «TMDB API»); **comprobados el 23-10-2026** (búsqueda «Akira 1988» → 200). Copia local en `secrets/tmdb.token`, fuera de Git |

### Lecciones de AstroIPTV que se heredan tal cual

1. Secuencia AVPlay `close → open → setDisplayRect → setListener → prepareAsync → play`, con **token de reproducción** para descartar callbacks tardíos.
2. **Serializar** aperturas: nunca `stop/close/open` mientras hay un `prepareAsync` pendiente (`state.wanted` + `openWanted` + `nativeClose`). Esto colgó la TV en v0.1.7.
3. **Nunca `setSelectTrack('TEXT', …)`**: colgó AVPlay para siempre (v0.1.8). Los subtítulos los **dibuja la app**.
4. Vídeo en un plano detrás del HTML: `<object type="application/avplayer">` hijo directo de `body`, fondos transparentes durante la reproducción, `setDisplayRect(0,0,1920,1080)` aunque la TV sea UHD.
5. Trazas `[av] …` por cada llamada nativa e inspector remoto (`sdb shell 0 debug` + `forward` + CDP) para diagnosticar cuelgues.
6. `.wgt` con nombre físico **sin espacios**; reinstalar con el **mismo ID** para no perder `localStorage`; el icono lo cachea Smart Hub (reinicio en frío).
7. `textContent`, nunca `innerHTML`, para datos. JS puro, sin bundler.

### La colección (medida en `cine-t7-catalogo`, 499 archivos)

| Aspecto | Recuento | Lectura para la tele |
|---|---|---|
| Vídeo H.264 | 269 | Directo |
| Vídeo HEVC | 203 (175 de 10 bits) | Directo previsto (verificar 10 bits en G0) |
| Vídeo MPEG-4 ASP (XviD) | 27 | Probable; verificar en G0 |
| 4K | 3 | Directo previsto; puede necesitar `SET_MODE_4K` y Ethernet/Wi-Fi bueno |
| Audio AC3 / E-AC3 / AAC / MP3 | 240 / 94 / 190 / 6 | Directo |
| Audio **DTS** | 9 | **No admitido** por Samsung desde 2018 → optimizar |
| Subtítulos SRT | 421 | Extraer en servidor y dibujar en la app |
| Subtítulos ASS/SSA | 18 | Convertir a texto plano en servidor |
| Subtítulos **PGS** / DVD (imagen) | 27 / 2 | AVPlay no los muestra → OCR a SRT (offline) o quemar |

> El perfil exacto de formatos que admite la tele (las «memorias de Codex» del usuario) se incorpora en la fase 0 como `profiles/samsung-ls01b-2022.json`. **La compatibilidad es un dato, no código**: si mañana cambia la tele, se cambia el perfil.

---

## 3. Arquitectura

```
┌──────────────── PC de casa (Windows / macOS) ────────────────┐        ┌──── Samsung The Serif ────┐
│                                                              │        │                           │
│  Fuentes ───► Escáner ──► ffprobe ──► SQLite ◄── Semilla T7  │  LAN   │  App Tizen «YourFlix»     │
│  T7, Patriot…     │                     │   ▲   + TMDB (resto)│◄─────►│  · Catálogo (JS puro)     │
│                   ▼                     │   │   cine-t7-     │  HTTP  │  · AVPlay ◄── vídeo por   │
│           Motor de compatibilidad       │   └── catalogo     │        │     HTTP Range            │
│           (perfil de la TV)             ▼                    │        │  · Subtítulos dibujados   │
│                               API REST /api/v1  ─────────────┼───────►│    por la app (JSON)      │
│  Optimizador (ffmpeg, cola) ◄── Admin web (navegador)        │        │  · Progreso → servidor    │
│  Caché de imágenes (redimensionadas)                         │        │                           │
└──────────────────────────────────────────────────────────────┘        └───────────────────────────┘
```

### 3.1 Servidor

**Decisión recomendada: Go** (un solo ejecutable).

- Compila desde el Mac para `windows/amd64` (ROG Ally) y `darwin/arm64` sin instalar nada en el destino (salvo ffmpeg).
- `net/http` + `http.ServeContent` resuelven **Range/206, If-Range, HEAD** correctamente: es justo lo que necesita AVPlay para buscar dentro de un MKV.
- SQLite sin cgo (`modernc.org/sqlite`) para que la compilación cruzada funcione.
- Servicio de Windows nativo (`golang.org/x/sys/windows/svc`) y `launchd` en Mac.
- Alternativa: Python 3.12 + FastAPI, más cercano a `cine-t7-catalogo`. Penaliza el despliegue en Windows (venv, servicio, versión de Python). Si el usuario lo prefiere, el plan no cambia salvo §3.1 y §8.

Módulos (`server/internal/…`):

| Módulo | Responsabilidad |
|---|---|
| `config` | `yourflix.toml`: puerto (8787), rutas de ffmpeg/ffprobe, carpeta de datos, token TMDB (`TMDB_ACCESS_TOKEN` o `secrets/tmdb.token`). Las **fuentes no viven aquí** sino en la base de datos, porque se gestionan desde la interfaz (§3.5) |
| `sources` | Alta, baja, pausa y re-escaneo de fuentes; explorador de carpetas del servidor para la web de admin; soporte desconectado/reconectado (§3.5) |
| `scan` | Recorre cada fuente, ignora `._*`, `$RECYCLE.BIN`, `System Volume Information` y patrones excluidos; identifica cada archivo por **tamaño + hash de 1 MB inicial + 1 MB final** (sobrevive a renombrados, a moverlo de carpeta o de disco y a cambios de letra de unidad). Vigila cambios (`fsnotify`) con re-escaneo periódico de respaldo |
| `probe` | `ffprobe -show_streams -show_format -of json`; guarda el JSON bruto y campos normalizados (códec, perfil, bits, HDR, pistas de audio/subtítulos con idioma y `default/forced`) |
| `parse` | Título/año/temporada/episodio desde nombre y carpeta. Reutiliza `clean_title` y el diccionario `ALIASES` de `CineT7/work/enrich_metadata.py` |
| `seed` | **Carga inicial** desde `cine-t7-catalogo` (`data/catalog.json` + `web/assets/`): importa títulos, fichas, portadas y copias medidas. Comando `yourflix seed import <ruta-o-url>`; se puede repetir sin duplicar (§3.7) |
| `meta` | Identificación de lo que **no** está en la semilla: (1) emparejar con la semilla por **bytes exactos + duración ±1 s**; (2) **TMDB** con el token del usuario, título+año, `es-ES` con respaldo `en-US`, puntuación de confianza; (3) cola «pendientes» para elegir a mano. Una corrección manual es **pegajosa** (no la pisa un re-escaneo). Respeta el límite de TMDB y cachea respuestas en SQLite |
| `images` | Descarga póster/fondo/foto de episodio/reparto, redimensiona a tamaños TV (póster 300×450 y 600×900, fondo 1280×720 y 1920×1080, JPEG q≈82), sirve con `Cache-Control` largo y nombre por hash |
| `compat` | Aplica el perfil de la TV a cada archivo → `directo` / `directo con avisos` (p. ej. subtítulos PGS no visibles) / `optimizar` (DTS, TrueHD, códec no admitido) con el motivo |
| `subs` | Extrae pistas de texto embebidas (SRT/ASS) y sidecar `.srt` a **cues JSON** `[{s,e,t}]`; ASS → texto sin estilos |
| `stream` | `GET /files/{id}/stream` con Range; tipo MIME por contenedor; **nunca** acepta rutas del cliente, solo IDs |
| `optimize` | Cola de trabajos ffmpeg: DTS → E-AC3 (`-c:v copy`), XviD → H.264 si G0 lo exige, PGS → SRT por OCR (herramienta externa, opcional). Escribe la copia en una carpeta de salida configurable, **nunca** sobrescribe el original; la copia sustituye al original en el catálogo |
| `progress` | Posición, duración y visto por archivo; «Seguir viendo»; siguiente episodio |
| `api` | REST JSON versionada `/api/v1` (contrato en `docs/API.md`) |
| `admin` | Web servida por el propio servidor con el diseño de Cine T7: biblioteca, pendientes de identificar, compatibilidad, cola de optimización, re-escaneo, registro |
| `service` | Instalar/desinstalar como servicio, arranque con el sistema, log rotado |

### 3.2 Cliente TV (Tizen 6.5)

App Web Tizen nueva, **ID propio** (propuesta `Z8YOURFLX1.YourFlix`), firmada con el perfil `CineT7`. La app Cine T7 actual (`Z8CINE0T7A.CineT7`, lee el T7 por USB) **no se toca** y sigue siendo el plan B.

| Módulo (`tv/js/…`) | Responsabilidad |
|---|---|
| `api.js` | Cliente REST con timeout, reintento y caché en memoria; descubre el servidor (§3.4) |
| `nav.js` | Navegación espacial con el mando (filas, rejillas, foco visible, memoria del foco por pantalla), registro de teclas `tizen.tvinputdevice` (MediaPlayPause, FF/RW, colores, números) |
| `player.js` | Controlador AVPlay **heredado de AstroIPTV** (serialización, token, trazas, vigilante de congelación), más reanudación (`seekTo` tras `prepare`), ±10/±30 s, barra de búsqueda |
| `subs.js` | Carga cues JSON del servidor y dibuja el subtítulo en HTML sincronizado con `getCurrentTime` — permite **elegir idioma con seguridad** sin `setSelectTrack TEXT` |
| `tracks.js` | Cambio de audio con `setSelectTrack('AUDIO')` serializado (como AstroIPTV), solo si G0 lo valida en MKV |
| `ui/*` | Pantallas (§5) |
| `images.js` | Carga diferida: solo fila visible ± 1; descarga imágenes fuera de pantalla para no agotar memoria |

### 3.3 Red y seguridad

- Solo LAN. El servidor escucha en la interfaz privada y rechaza IP fuera de rangos privados.
- **Emparejamiento** en v1: la TV pide un código; el servidor lo muestra en la web de admin; la TV guarda un token. Antes de v1 (fases 1–3) basta una lista de IP permitidas en la configuración.
- La biblioteca se abre **solo lectura**; la única escritura en disco es la carpeta de optimizados y la de datos (`data/`: SQLite, imágenes, log).
- Sin rutas en la API, sin path traversal posible (ID → ruta solo en el servidor).
- Secretos (TMDB) fuera del repo: variable de entorno o `secrets/tmdb.token` ignorado. La fuente original es Joplin; el token **nunca** sale del servidor (ni a la TV ni por la API).
- Firewall de Windows: el instalador crea la regla de entrada solo para perfil «Privado».
- El explorador de carpetas de la admin solo responde a `localhost` o a una sesión de admin, nunca a la TV.

### 3.4 Descubrimiento del servidor

Las apps web de Tizen no tienen UDP. Orden: (1) dirección guardada; (2) `http://<ip-reservada>:8787/api/v1/ping` configurada de fábrica en la app; (3) barrido de la `/24` de la TV en el puerto 8787 con XHR en paralelo (≈254 peticiones, tiempo de espera corto); (4) introducir IP a mano. **Recomendado**: reserva DHCP para el ROG Ally, como ya se hizo con Mac y TV.

### 3.5 Fuentes (carpetas de la biblioteca)

Una **fuente** es una carpeta que el usuario añade a la biblioteca. Puede haber tantas como quiera, en discos distintos.

| Campo | Ejemplo |
|---|---|
| Ruta | `E:\pelis`, `E:\pelis2`, `F:\`, `C:\Users\Edu\Videos`, `/Volumes/T7/pelis` |
| Etiqueta | «Samsung T7», «USB Patriot», «Ally» (se ve en la TV: fila y filtro «por soporte») |
| Tipo | `auto` (por defecto), `películas`, `series` — fuerza la interpretación si los nombres engañan |
| Exclusiones | patrones (`*sample*`, `Extras/`) |
| Volumen | etiqueta/serie del volumen, para reconocer el disco aunque cambie la letra |
| Estado | activa · pausada · **desconectada** (el disco no está: sus títulos se ocultan pero conservan ficha y progreso) |

**Desde la web de admin**: botón «Añadir carpeta» con un explorador de carpetas del servidor (unidades → carpetas; el navegador no puede dar rutas de otro equipo, así que el explorador lo sirve el propio servidor), etiqueta y tipo; después, escaneo con progreso en vivo. Pausar, re-escanear o quitar (quitar **no borra** ningún archivo, solo lo saca del catálogo).

**Por CLI** (mismo efecto):

```
yourflix source add "E:\pelis" --label "Samsung T7" --kind auto
yourflix source add "F:\" --label "USB Patriot" --exclude "*sample*"
yourflix source list
yourflix source scan <id|--all>
yourflix source pause <id> · resume <id> · remove <id>
```

Un mismo archivo en dos fuentes (p. ej. Platoon en T7 y Patriot) es **un título con dos copias**, como ya hace `cine-t7-catalogo`; la TV elige la copia disponible.

### 3.6 Servidor en el ROG Ally

- x64 con Windows 11: binario `windows/amd64`, ffmpeg de gyan.dev junto al ejecutable.
- **Sin Ethernet** y con un único USB-C (en el Ally original): para disco + carga + red hace falta un **dock USB-C** (o el cargador/dock oficial con USB y Ethernet). Recomendado cable de red en el dock; si no, Wi-Fi 6E en 5/6 GHz y medir caudal en la fase 0 (S10).
- Es una consola portátil: el instalador desactiva suspensión e hibernación **con cargador**, y se recomienda el límite de carga al 80 % de Armoury Crate para la batería. El servicio arranca sin iniciar sesión.
- La GPU tiene codificador por hardware (AMF): ffmpeg `h264_amf`/`hevc_amf` acelera las optimizaciones de vídeo de la fase 4 (las de audio DTS → E-AC3 no lo necesitan).
- Discos USB: Windows puede dormirlos; se desactiva la suspensión selectiva de USB y la TV muestra «Despertando el disco…» si el primer acceso tarda.

### 3.7 Carga inicial y metadatos

1. **Semilla T7** (`yourflix seed import`): lee `cine-t7-catalogo/data/catalog.json` y sus portadas de `web/assets/` (o las URL HTTPS de TMDB que ya trae). Crea los 255 títulos con su ficha, reparto, sinopsis y portadas, y guarda la **huella** de cada copia (bytes + duración). No hace ninguna llamada a TMDB.
2. **Escaneo de la fuente T7**: cada archivo encontrado se empareja con la semilla por **bytes exactos** (y duración ±1 s si hay duda). El catálogo público no tiene rutas, así que es el escaneo local el que aporta la ruta real. Lo que no empareje pasa al paso 3.
3. **TMDB** para todo lo demás (archivos nuevos del T7, Patriot si no estaba, otras fuentes): búsqueda por título+año limpiados con `parse`, ficha en español, póster, fondo, reparto y, en series, temporadas/episodios con sus fotos. Confianza alta → se asigna; baja → «pendientes» en la admin.
4. Las imágenes de la semilla y de TMDB se redimensionan y cachean igual; la TV nunca sale a Internet.
5. La semilla **no** se re-importa encima de cambios manuales hechos en YourFlix (los bloqueos mandan).

---

## 4. Modelo de datos (SQLite)

```
sources    (id, label, path, kind, excludes, volume_id, state, last_scan_at)
files      (id, source_id, relpath, size, mtime, fingerprint, probe_json, duration_s,
            vcodec, vprofile, bit_depth, width, height, hdr, container,
            audio_json, subs_json, compat, compat_reasons, optimized_of, missing_since)
titles     (id, kind movie|series, tmdb_id, imdb_id, title, original_title, year,
            meta_json, poster_img, backdrop_img, match_source seed|tmdb|manual,
            match_confidence, locked)
episodes   (title_id, season, episode, file_id, name, still_img, overview)
title_files(title_id, file_id, label)          -- versiones/copias de una película
images     (hash, source_url, w, h, path)
progress   (file_id, position_s, duration_s, watched, updated_at)
jobs       (id, kind, file_id, status, log, created_at, finished_at)
settings   (key, value)
```

IDs estables y opacos (no autoincrementos expuestos). Si un archivo desaparece se marca `missing_since` y se oculta, pero no se borra su progreso ni su ficha (el disco puede estar desconectado).

---

## 5. Experiencia en la TV

Diseño heredado de Cine T7 (oscuro, dorado `#e5b95c`, Georgia para títulos), adaptado a 10 pies: textos ≥ 28 px, foco muy visible, nada de hover.

| Pantalla | Contenido |
|---|---|
| **Inicio** | Fondo del título destacado (rota cada 10 s), filas: Seguir viendo · Añadidas recientemente · Películas · Series · por género · por soporte (T7, Patriot) |
| **Películas / Series** | Rejilla de pósteres paginada (60), orden A-Z / año / añadidas / duración, filtros por género |
| **Ficha** | Fondo, póster, título, año, duración, género, director, reparto (fotos), sinopsis en español, distintivos técnicos (4K · HDR · HEVC 10 bits · 5.1 · ES/EN · Sub ES), estado de compatibilidad, botones Reproducir / Reanudar desde mm:ss / Empezar de nuevo / Marcar visto; si hay varias copias, elegir copia |
| **Serie** | Temporadas en pestañas, episodios con foto, título, sinopsis y progreso; «Siguiente episodio» |
| **Buscar** | Teclado en pantalla propio (A-Z, 0-9) con resultados al vuelo por título, original, director y reparto |
| **Reproductor** | OSD con título, barra, tiempo, ±10/±30 s, audio, subtítulos (idioma, tamaño, desactivar); al terminar, siguiente episodio con cuenta atrás; progreso enviado cada 15 s y al salir |
| **Ajustes** | Servidor, emparejar, idioma de subtítulos preferido, tamaño de subtítulos, acerca de (versiones de app y servidor) |

Teclas: flechas, OK, Atrás (salir en dos pasos, como AstroIPTV), Play/Pause, FF/RW (±30 s), Stop, colores (rojo = visto, verde = buscar…), CH ▲▼ = página.

---

## 6. Contrato de API (resumen; detalle en `docs/API.md` en la fase 1)

```
GET  /api/v1/ping                          {name, version, serverId}
GET  /api/v1/home                          filas de Inicio
GET  /api/v1/titles?kind=&genre=&sort=&q=&offset=&limit=
GET  /api/v1/titles/{id}                   ficha + copias/temporadas/episodios + compat
GET  /api/v1/files/{id}/stream             vídeo con Range (206), HEAD
GET  /api/v1/files/{id}/subtitles          pistas disponibles (idioma, origen, forced)
GET  /api/v1/files/{id}/subtitles/{sid}    cues JSON
GET  /api/v1/img/{hash}/{size}.jpg         imagen redimensionada
GET  /api/v1/continue                      seguir viendo
PUT  /api/v1/progress/{fileId}             {position, duration, watched}
POST /api/v1/pair                          emparejamiento
---- admin (solo desde la web de admin / localhost) ----
GET/POST/PATCH/DELETE /api/v1/admin/sources · POST /admin/sources/{id}/scan · GET /admin/browse?path=
POST /api/v1/admin/seed · GET /admin/pending · PUT /admin/titles/{id}/match
POST /api/v1/admin/optimize/{fileId} · GET /admin/jobs · GET /admin/log
```

> Fase 5: la administración quedó en `/admin/api/…`, aparte de `/api/v1` (ADR-007; contrato en `docs/API.md`).

El contrato se congela al final de la fase 1A con fixtures JSON reales. A partir de ahí servidor y TV se desarrollan **en paralelo**: la TV contra un servidor simulado (`yourflix --mock` o fixtures estáticos).

---

## 7. Estructura del repositorio

```
YourFlix/
├── PLAN.md  README.md  AGENTS.md
├── docs/  API.md  DECISIONES.md  TV-PROFILE.md  PRUEBAS-TV.md
├── profiles/samsung-ls01b-2022.json      # formatos admitidos (dato)
├── server/                               # módulo Go
│   ├── cmd/yourflix/main.go
│   ├── internal/{config,sources,scan,probe,parse,seed,meta,images,compat,subs,stream,optimize,progress,api,admin,service,store}
│   ├── web/                              # admin web estática
│   └── testdata/                         # JSON de ffprobe reales, nombres de archivo de ejemplo
├── tv/                                   # app Tizen
│   ├── config.xml index.html icon.png css/ js/
│   └── tests/                            # node --test
├── tools/
│   ├── spike-avplay/                     # app mínima de la fase 0
│   ├── avplay-sim/                       # AVPlay simulado para Chromium del Mac
│   └── tv-monitor/                       # monitor CDP del inspector remoto
└── scripts/  build-tv.sh  install-tv.sh  build-server.sh  release.sh
```

Fuera de Git: `.buildResult/`, `*.wgt`, `data/`, `secrets/`, `private/`, binarios.

---

## 8. Fases

Cada fase es un **paquete de trabajo** con entrada, entregables, criterio de hecho y modelo sugerido. Las fases marcadas ∥ pueden ir en paralelo.

### Fase 0 — Pruebas en la tele real (puerta G0) · *Opus*

Objetivo: no construir nada sobre suposiciones de AVPlay.

- `tools/spike-avplay`: app Tizen mínima con una lista de URL y trazas `[av]`.
- Servidor de prueba en el Mac: `python3 -m http.server` **no** vale (Range incompleto); un `main.go` de 30 líneas con `http.ServeContent`.
- Archivos de prueba reales (uno por categoría) y matriz a rellenar en `docs/PRUEBAS-TV.md`:

| # | Prueba | Qué se mide |
|---|---|---|
| S1 | MKV H.264 + AC3 por HTTP | Arranque, `seekTo` a 50 %, ±30 s, reanudar en mm:ss, peticiones Range que hace AVPlay |
| S2 | MKV HEVC Main10 + E-AC3 | Igual que S1 |
| S3 | AVI/MKV XviD | ¿Reproduce? |
| S4 | MKV con DTS | ¿Error, silencio o cuelgue? (define cómo se protege la app) |
| S5 | MKV con 2 audios | `getTotalTrackInfo` y `setSelectTrack('AUDIO')` en archivo (no stream) |
| S6 | MKV con SRT embebido | Qué reporta AVPlay; confirmar que **no** se toca TEXT |
| S7 | 4K HEVC HDR por Wi-Fi | Fluidez, ¿hace falta `SET_MODE_4K`?, caudal medido |
| S8 | MP4 de la colección | Control |
| S9 | Archivo de 15–25 GB | Arranque y saltos lejanos (offsets > 4 GB) |
| S10 | Servidor en el ROG Ally | Repetir S1 y S7 con el binario en el Ally (Defender, firewall, disco USB dormido, caudal por Wi-Fi y por dock con cable) |

Entregables: matriz rellenada, `profiles/samsung-ls01b-2022.json` con las memorias de Codex del usuario + resultados, ADR-001 en `docs/DECISIONES.md`.

**G0 (decisión con el usuario):** si S1/S2/S9 pasan → seguir con reproducción directa. Si no → valorar HLS remux (`-c copy` a segmentos) o Jellyfin.

### Fase 1A — Núcleo del servidor · *Sonnet* (revisa Opus)

`config`, `sources` (CLI completa de §3.5), `scan`, `probe`, `store`, `stream`, `api` mínima (`ping`, `titles` desde nombre de archivo, `stream`). Tests: Range (inicio, mitad, final, sufijo, > 4 GB, `If-Range`), huella de archivo, fuentes múltiples con un archivo repetido, fuente desconectada y reconectada con otra letra, parseo con los `testdata`. CLI: `yourflix source …`, `yourflix serve`.
**Hecho cuando** la app spike de la fase 0 reproduce desde este servidor y `docs/API.md` + fixtures quedan congelados.

### Fase 1B ∥ — Identificación y carátulas · *Sonnet*

`seed` (carga inicial T7), `parse` (con ALIASES de Cine T7), `meta` (emparejado por bytes, TMDB, confianza, pendientes, bloqueo manual), `images` (redimensionado y caché), series con temporadas/episodios y fotos.
**Hecho cuando** los archivos del T7 que están en la semilla quedan emparejados con su ficha **sin ninguna llamada a TMDB**; una carpeta nueva de prueba se identifica con TMDB en ≥ 90 % sin intervención y el resto aparece en pendientes; ninguna imagen se pide a Internet desde la TV.

### Fase 2 ∥ — Cliente TV: navegación y catálogo · *Sonnet* (UI) + *Opus* (revisión M85 y memoria)

Contra el servidor simulado: `nav.js`, `api.js`, Inicio, rejillas, Ficha, Serie, Buscar, Ajustes, descubrimiento. Tests en Node para la lógica (navegación, paginación, emparejado de foco). Prueba en Chromium del Mac **y** en la TV (M85).
**Hecho cuando** se recorre toda la colección con el mando en la TV real sin tirones apreciables y con memoria estable (inspector).

### Fase 3 — Reproducción · *Opus*

`player.js` (portar el controlador de AstroIPTV con tests), reanudar, OSD, `subs.js` con cues del servidor, `tracks.js` (audio) según G0, siguiente episodio, envío de progreso. Pruebas: simulador AVPlay en Chromium (lento, que «cuelga» si se viola la serialización) + TV con monitor CDP.
**Hecho cuando** el usuario ve una película completa con subtítulos elegidos, la deja a medias, la reanuda desde Inicio, y salta ±30 s y al 80 % sin cuelgues durante una sesión monitorizada.

### Fase 4 — Compatibilidad y optimización · *Sonnet* (revisa Opus)

`compat` con el perfil, distintivos en ficha, `optimize` con cola (DTS → E-AC3 conservando vídeo; XviD → H.264 si G0 lo pide), extracción ASS → texto, OCR de PGS como paso opcional documentado (herramienta externa, no obligatoria). La copia optimizada hereda ficha y progreso.
**Hecho cuando** los 9 DTS se reproducen con sonido tras optimizar y ningún archivo «optimizar» puede colgar la app (se avisa antes de abrirlo).
> **24-10-2026:** el usuario descarta la conversión DTS → E-AC3 (9 archivos, no le preocupan). La app 0.3.0 pasa sola a otra pista si la DTS es la de por defecto y avisa si solo hay DTS.

### Fase 5 ∥ — Admin web y despliegue en Windows · *Sonnet* (+ *Haiku* para textos/guías)

Admin web con diseño Cine T7 (fuentes con explorador de carpetas, escaneo en vivo, pendientes, compatibilidad, optimización), servicio de Windows + `launchd`, regla de firewall, energía del ROG Ally (§3.6), `release.sh` que compila los binarios, guía de instalación en el Ally paso a paso, arranque tras corte de luz, disco USB con letra cambiante (se reconoce por el volumen).
**Hecho cuando** el Ally arranca en frío con el dock, el servicio sube solo y la TV encuentra el servidor sin tocar nada; y una carpeta nueva se añade desde la admin sin usar la CLI.

### Fase 6 — Endurecimiento y v1.0 · *Opus* (revisión cruzada con *Codex*)

Emparejamiento por código, revisión de seguridad, prueba de una semana de uso real, README con estado real, etiqueta `v1.0.0`.

### Posterior a v1 (no prometido)

Transcodificación en vivo (HLS) · varios perfiles de usuario · capítulos MKV · intro/créditos · exportar al catálogo público `cine-t7-catalogo` · acceso desde el móvil como mando · tráilers.

---

## 9. Trabajo multimodelo

### Reparto recomendado

| Modelo | Papel |
|---|---|
| **Opus 5.5** | Arquitectura, fase 0, controlador AVPlay, puertas de decisión, revisiones de seguridad y de código crítico |
| **Sonnet 5.5** | Implementación del grueso: servidor, identificación, UI de la TV, admin web |
| **Haiku 5.5** | Tareas mecánicas: fixtures, textos de interfaz, guías, renombrados, tablas de pruebas |
| **Codex (GPT)** | Revisor cruzado de PR y segunda opinión; puede implementar paquetes completos siguiendo `AGENTS.md` |

### Reglas de relevo

1. **Contrato primero**: `docs/API.md` + fixtures y `profiles/*.json` se congelan antes de paralelizar. Cambiar el contrato = ADR + aviso en README.
2. **Un escritor por zona**: `server/` y `tv/` pueden ir a la vez con modelos distintos; dos modelos nunca editan los mismos archivos a la vez.
3. **Rama por paquete** (`fase-1a/stream`, `fase-2/ficha`…) y PR.
4. **Cada sesión termina** actualizando la sección «Estado» de `README.md`: qué se hizo, qué se probó de verdad (navegador ≠ TV), qué falta, siguiente paso. Es el relevo del siguiente modelo.
5. **Decisiones** en `docs/DECISIONES.md` (ADR numerados, breves). No se re-discute una decisión cerrada sin un dato nuevo.
6. **La TV es del usuario**: instalar o lanzar en la tele solo desde una sesión con acceso al Mac y con permiso, nunca mientras está viendo algo.
7. Comprobaciones mínimas antes de cualquier PR: `go test ./...`, `go vet ./...`, `node --test tv/tests/*.test.js`, `node --check` de cada JS de `tv/`, y la regla de M85 (test que prohíbe `inset`, `aspect-ratio`, `?.` si G0 confirma que no está, etc.).

---

## 10. Puertas de decisión

| Puerta | Cuándo | Qué se decide | Quién |
|---|---|---|---|
| G0 | Fin fase 0 | Reproducción directa por HTTP o alternativa (HLS/Jellyfin) | Usuario, con matriz de pruebas |
| G1 | Fin 1A | Contrato de API congelado | Opus |
| G2 | Fin fase 3 | La app reemplaza en el día a día a Cine T7 USB | Usuario |
| G3 | Fin fase 6 | v1.0 | Usuario |

---

## 11. Riesgos

| Riesgo | Prob. | Impacto | Mitigación |
|---|---|---|---|
| AVPlay no busca bien en MKV por HTTP o no pasa de 4 GB | Media | Alto | Fase 0 antes de todo; plan B HLS remux sin recodificar |
| Wi-Fi de la TV insuficiente para remux/4K (40–80 Mb/s) | Media | Medio | Medir en S7; servidor por cable; avisar en la ficha de archivos > 40 Mb/s |
| Cuelgue nativo de AVPlay (como en IPTV) | Media | Alto | Heredar serialización, no TEXT, trazas, monitor CDP |
| Memoria de la TV con cientos de pósteres | Media | Medio | Imágenes redimensionadas en servidor, carga diferida, liberar fuera de pantalla |
| ROG Ally suspendido, sin cargador o disco USB en reposo | Alta | Medio | Dock con carga, suspensión desactivada con cargador, suspensión USB desactivada, reintento en la TV con mensaje claro |
| Ally sin Ethernet: Wi-Fi → Wi-Fi (servidor y TV) duplica el uso del aire | Media | Medio | Dock con cable de red; medir en S10 |
| Identificación errónea (remakes, títulos en español) | Media | Bajo | Semilla por bytes, año obligatorio en TMDB, cola de pendientes, bloqueo manual |
| Certificado `CineT7` caduca 18-07-2027 | Segura | Alto | Recordatorio en README; renovar antes de junio 2027 |
| Nombres con tildes/ñ en NTFS y macOS (NFC/NFD) | Media | Bajo | Normalizar a NFC al comparar; tests con nombres reales |
| Clave TMDB filtrada | Baja | Medio | Solo en servidor, fuera de Git, nunca en la API |

---

## 12. Decisiones

### Cerradas (23-10-2026)

- Nombre del repo y la app: **YourFlix**; app TV nueva con ID propio `Z8YOURFLX1.YourFlix`.
- Servidor principal: **Asus ROG Ally** (Windows 11); el Mac como alternativa y para desarrollo.
- **Fuentes múltiples** elegidas por el usuario desde la admin o la CLI.
- **Carga inicial** = catálogo T7 de `cine-t7-catalogo`; **TMDB** (clave del usuario en Joplin, comprobada) para todo lo que se añada después.

### Abiertas

1. ~~Go o Python~~ → **Go** (cerrada, ADR-002).
2. ¿Un único perfil de usuario o varios (progreso por persona)?
3. Pasar las **memorias de Codex** con el formato que admite la tele para el perfil de la fase 0.
4. ¿Qué modelo exacto de Ally (original o Ally X) y si hay dock con Ethernet?

### Peticiones del usuario (24-10-2026), pendientes de plan

5. **Cliente web** para ver la colección desde cualquier navegador (móvil, Mac, PC) en la LAN, sin tocar la app de Tizen. Medido: de 496 archivos, 480 son MKV; solo 53 llevan H.264 de 8 bits con audio que lee un navegador; 214 solo necesitan convertir el audio (AC3/E-AC3/DTS → AAC) y 229 tienen vídeo HEVC/10 bits/XviD. Exige remux en vivo (vídeo copiado y audio a AAC, HLS o fMP4) y, para XviD, recodificar (AMF del Ally). Cambia la regla «sin transcodificación en vivo» (ADR-001): necesita ADR.
   > **24-10-2026: implementado en la rama `fase-web/cliente` (PR a `main`), [ADR-006](docs/DECISIONES.md).** El mismo servidor sirve `web/` en `/`. Directo cuando el navegador puede; si no, HLS fMP4 con vídeo copiado y audio a AAC; XviD, HEVC sin soporte (Firefox) y H.264 de 10 bits se recodifican a H.264 (`h264_amf` o `libx264`). Lista VOD completa, segmentos bajo demanda, ffmpeg relanzado en cada salto, 2 conversiones a la vez, caché en `DATOS/remux`. Probado en la nube con vídeos generados y Chromium; falta probar en iPhone, Android, Mac y Windows contra el Ally.
6. **Servidor permanente en el Ally** (servicio de Windows) que sirva la API, la app web y la administración.
7. **Interfaz de administración** del servidor (es la fase 5). → **Hecha en la rama `fase-5/admin`** (24-10-2026, ADR-007): web en `/admin/` con fuentes (explorador de carpetas del servidor, etiqueta, tipo, exclusiones, pausar, quitar, escaneo con progreso en vivo), catálogo (pendientes con candidatos de TMDB, emparejado con un clic que bloquea la ficha, búsqueda manual, identificar), estado (versión, encendido, ffmpeg/ffprobe, discos, escaneos, registro) y compatibilidad. Contraseña con hash y sesión; `localhost` sin contraseña. API en `docs/API.md` («API de administración»). Falta montarla en `main.go` (dos líneas, ver `server/cmd/yourflix/admin.go`) y probarla en el Ally.
8. **Audio y subtítulos en la TV:** hoy solo con los botones de color (verde y amarillo), que el mando Smart Remote de The Serif no tiene a la vista. Hacen falta botones en pantalla en el reproductor.

> **Actualización 24-10-2026:** el trabajo continúa en el propio Ally (Wi-Fi `192.168.1.13` por ahora; cable e IP nueva después). Estado real y relevo: `README.md` («Estado») y `docs/ENTORNO-ALLY.md`.
