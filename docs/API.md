# API de YourFlix · v1

Base: `http://<servidor>:8787/api/v1`. JSON UTF-8, CORS abierto. **Solo red local**: el servidor responde 403 a IP que no sean privadas o de bucle local. Los IDs son opacos y estables; **ninguna respuesta incluye rutas de disco**.

> Estado: implementado en el servidor 0.1.0 (fase 1A). Aún no congelado: la fase 2 (cliente TV) puede pedir cambios, que se anotan en `docs/DECISIONES.md`.

## `GET /ping`

```json
{"name":"YourFlix","version":"0.1.0","serverId":"fa3f7897c67c","api":1}
```

La TV lo usa para descubrir el servidor y comprobar que sigue siendo el mismo (`serverId`).

## `GET /titles`

Parámetros: `kind` (`movie` | `series`), `q` (busca en título, original y ficha: director, reparto…), `sort` (`az` por defecto | `year` | `added` | `rating`), `genre` (nombre exacto de `GET /genres`), `offset`, `limit` (60 por defecto, máx. 500). Solo devuelve títulos con algún archivo disponible.

```json
{"total":1,"offset":0,"items":[{
  "id":"t7-manual-e37844b42ec9","kind":"movie","title":"No Hay Salida","year":"1987",
  "genres":["Suspense","Drama"],"rating":6.857,
  "poster":"/api/v1/titles/t7-manual-e37844b42ec9/poster?w=300",
  "backdrop":"/api/v1/titles/t7-manual-e37844b42ec9/backdrop?w=1280",
  "files":2,"sources":["Pruebas","aMule"],"addedAt":"2026-10-23T18:56:26Z","matched":true}]}
```

`matched: false` = ficha provisional deducida del nombre del archivo: TMDB no la identificó con seguridad (pendiente de elegir a mano).

## `GET /titles/{id}`

```json
{"title":{"id":"…","kind":"movie","title":"No Hay Salida","original":"No Way Out","year":"1987","tmdb":10083,
          "meta":{"synopsis":"…","genres":[…],"director":[…],"cast":[…],"runtime":116,"rating":6.857,"tagline":"…","inventory":{…}},
          "matchSource":"seed","addedAt":"…"},
 "poster":"/api/v1/titles/…/poster?w=600","backdrop":"/api/v1/titles/…/backdrop?w=1920",
 "files":[{"id":"eaf3b7aec499","source":"Pruebas","size":2367101437,"duration":6863.0,"container":"mkv","bitrate":2759250,
   "video":{"codec":"hevc","profile":"Main","width":1920,"height":1040,"bitDepth":8},
   "audio":[{"index":1,"codec":"aac","channels":2,"language":"spa","default":true},{"index":2,"codec":"aac","channels":6,"language":"eng"}],
   "subtitles":[{"index":3,"codec":"hdmv_pgs_subtitle","language":"spa","title":"Forzados","text":false}, …],
   "season":null,"episode":null,"label":"",
   "compat":{"status":"warn","reasons":["3 subtítulos en imagen (PGS/DVD) no visibles"]},
   "progress":{"position":600,"duration":6863,"watched":false,"updatedAt":"…"},
   "stream":"/api/v1/files/eaf3b7aec499/stream"}]}
```

- Series: un elemento de `files` por capítulo, ordenados por `season`, `episode`. Si TMDB conoce el capítulo, `episodeInfo` trae `name`, `overview`, `airDate` y `still` (URL de la foto del capítulo):
  `"season":1,"episode":1,"episodeInfo":{"name":"El sistema","overview":"Carmy intenta…","still":"/api/v1/files/9e190eab8098/still?w=300","airDate":"2022-06-23"}`
- `compat.status`: `direct` (tal cual) · `warn` (se reproduce, con avisos) · `optimize` (p. ej. solo DTS: **sin sonido** en la TV). Lo calcula el perfil `profiles/samsung-ls01b-2022.json`.
- `subtitles[].text: false` = imagen (PGS/DVD): AVPlay no la entrega.

## `GET /titles/{id}/poster?w=` · `GET /titles/{id}/backdrop?w=`

JPEG redimensionado en el servidor y cacheado (`Cache-Control: max-age=604800`). Anchos: `185`, `300`, `600`, `1280`, `1920` (no se amplía si el original es menor). 404 si no hay imagen, 502 si no se pudo descargar.

## `GET /files/{id}/still?w=`

Foto del capítulo (series identificadas con TMDB), redimensionada y cacheada como los pósteres.

## `GET /files/{id}/subtitles`

Pistas de **texto** que la TV puede dibujar: internas (SRT, ASS, mov_text, WebVTT) y archivos junto al vídeo con el mismo nombre (`Película.es.srt`, `Película.en.forced.srt`, `.ass`, `.vtt`). `imageTracks` cuenta las de imagen (PGS, DVD), que no se pueden mostrar.

```json
{"tracks":[{"id":"e3","language":"spa","label":"Español (forzados)","codec":"subrip","forced":true,"source":"embedded"},
           {"id":"x0","language":"spa","label":"Español · archivo","codec":"srt","source":"external"}],"imageTracks":0}
```

## `GET /files/{id}/subtitles/{sid}`

Líneas `[[inicio, fin, "texto"], …]` en segundos, ordenadas, sin etiquetas de estilo. SRT en UTF-8 o Windows-1252. La **primera** petición de una pista interna lee el vídeo entero con ffmpeg (todas las pistas de texto de una vez) y lo guarda en `data/subs`: en un disco USB puede tardar decenas de segundos; las siguientes son instantáneas. 422 si no se pudo convertir.

## `GET|HEAD /files/{id}/stream`

El archivo tal cual, con `Range` (206), `Content-Range`, `Accept-Ranges` y `Content-Type` según contenedor (`video/x-matroska`, `video/mp4`, `video/x-msvideo`). Probado con AVPlay en la fase 0, también > 4 GB. Errores: 404 ID desconocido, **410** archivo de un disco desconectado, **503** el disco no responde en 15 s.

## `GET /continue`

«Seguir viendo»: lo último a medias de cada título (≥ 60 s y < 92 %). En una serie cuyo último capítulo se terminó, propone el siguiente sin ver con `"next":true` y `position` 0.

## `PUT /progress/{fileId}`

```json
{"position":600,"duration":6863}
```

204. Con más del 92 % se marca `watched`. La TV lo envía cada 15 s y al salir.

## `GET /sources`

Etiqueta, tipo y estado de cada fuente (`active` | `paused` | `offline`) con su número de archivos. Sin rutas.

```json
[{"id":"d0602025fda2","label":"Pruebas","kind":"auto","state":"active","files":11}]
```

## Pendiente (fases siguientes)

Emparejamiento de la TV por código (fase 6). La administración está en la sección siguiente.

---

# API de administración · `/admin/api`

Base: `http://<servidor>:8787/admin/api`. La sirve `internal/admin` (fase 5) junto a la web `admin/` en `/admin/`. Es **otra API**, aparte de `/api/v1`: sin CORS, con sesión, y **sí devuelve rutas de disco** (fuentes, explorador de carpetas, archivos), pero solo a una sesión de administración o a `localhost`, nunca a la TV ni a la web de la colección.

## Seguridad

- **Solo LAN**, como `/api/v1`: 403 a IP que no sean privadas, de bucle local o de enlace local.
- **Desde el propio servidor** (`127.0.0.1`/`::1` **y** cabecera `Host` `localhost`/`127.0.0.1`/`[::1]`) se entra **sin contraseña**. Comprobar el `Host` evita que una web abierta en el Ally entre con un dominio que resuelva a 127.0.0.1 (DNS rebinding).
- **Desde otro equipo**: contraseña de administración, guardada con hash PBKDF2-SHA256 (600 000 vueltas, sal aleatoria) en `settings.adminPassword`, y cookie de sesión `yf_admin` (`HttpOnly`, `SameSite=Strict`, `Path=/admin/`, 7 días). Las sesiones viven en memoria: reiniciar el servidor obliga a volver a entrar. Cambiar la contraseña (web o CLI) cierra las demás sesiones.
- **Primera contraseña**: solo desde `localhost` (web) o con `yourflix admin password` (lee de la entrada estándar); también `YOURFLIX_ADMIN_PASSWORD` en el primer arranque si aún no hay ninguna. Mínimo 8 caracteres. Desde la LAN no se puede definir la primera.
- **Intentos**: 5 contraseñas malas seguidas desde una IP la bloquean 1 minuto (429).
- **CSRF**: todo lo que no es `GET` exige `Content-Type: application/json`, `Origin` (si viene) igual al `Host`, y la cabecera `X-CSRF-Token` con el token de la sesión (lo dan `GET /session` y `POST /login`). Desde `localhost` también: `GET /session` crea una sesión local para tener token.
- Cabeceras: `Content-Security-Policy` (`default-src 'self'`; imágenes también de `image.tmdb.org`), `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cache-Control: no-store` en la API.
- **El token de TMDB nunca sale**: ni en respuestas ni en el registro (el anillo de registro de la página de estado lo tacha si apareciera).

Errores: JSON `{"error":"mensaje en español"}` con 400 (datos), 401 (sin sesión), 403 (LAN/CSRF), 404, 409 (conflicto), 429 (intentos o cola llena), 502 (TMDB), 503 (TMDB sin configurar).

## Sesión

| Método y ruta | Qué hace |
|---|---|
| `GET /session` | `{"authenticated":true,"local":true,"passwordSet":false,"version":"0.1.0","csrf":"…"}`. `csrf` solo con sesión |
| `POST /login` `{"password":"…"}` | 200 `{"authenticated":true,"csrf":"…"}` + cookie · 401 contraseña mala · 409 aún no hay contraseña · 429 |
| `POST /logout` | 204, borra la cookie |
| `POST /password` `{"current":"…","password":"…"}` | 204. La primera, solo desde `localhost`; desde la LAN hace falta `current` |

## Fuentes

| Método y ruta | Qué hace |
|---|---|
| `GET /sources` | `[{"id","label","path","kind","excludes":[…],"state","createdAt","lastScanAt","files"}]` |
| `POST /sources` `{"path","label","kind","excludes":[…],"scan":true}` | 201 `{"source":{…},"job":{…},"warning":"…"}`. La ruta debe ser una carpeta absoluta accesible; 409 si ya es una fuente; `warning` si se solapa con otra. `scan:true` encola el escaneo |
| `PATCH /sources/{id}` `{"label","kind","excludes"}` | Cambia lo que venga (la ruta no se cambia). Tipo y exclusiones se aplican en el próximo escaneo; el tipo solo reinterpreta archivos nuevos, cambiados o pendientes |
| `POST /sources/{id}/pause` · `/resume` | Estado `paused` / `active` (pausada = no se escanea) |
| `DELETE /sources/{id}` | Saca la fuente del catálogo con sus archivos y su progreso **en la base de datos**; no toca el disco. `{"removed","files","orphanTitles"}` |
| `POST /sources/{id}/scan` · `POST /scan` | 202 con el trabajo (una fuente o todas). Tras escanear, si hay TMDB, identifica lo nuevo (como `serve`) |

Solo IDs en la URL, nunca etiquetas (la CLI sí acepta etiquetas).

## Explorador de carpetas

`GET /browse?path=` — sin `path`, las raíces: unidades `A:`…`Z:` que respondan en 3 s (Windows) o `/`, la carpeta personal y `/Volumes`, `/media`, `/mnt` si existen (macOS/Linux). Con `path` (absoluta), sus **subcarpetas** (sin ocultas, `$RECYCLE.BIN` ni `System Volume Information`) y cuántos vídeos hay directamente dentro. Solo lista; no lee archivos.

```json
{"path":"C:\\YourFlix","parent":"C:\\","sep":"\\","videos":0,"truncated":false,
 "dirs":[{"name":"Patriot","path":"C:\\YourFlix\\Patriot"},{"name":"T7","path":"C:\\YourFlix\\T7"}],
 "roots":[{"name":"C:","path":"C:\\"},{"name":"D:","path":"D:\\"}]}
```

400 ruta relativa o ilegible · 404 no existe · 10 s de espera máxima por carpeta (disco dormido).

## Trabajos (escaneos e identificación)

Uno a la vez, en cola (máx. 32); si ya hay uno igual en cola o en marcha, se devuelve ese. Comparten candado con el re-escaneo periódico de `serve` (ver `server/cmd/yourflix/admin.go`).

```json
{"id":"a1b2c3d4e5f6","kind":"scan","title":"Escanear «Samsung T7»","state":"running","queuedAt":"…","startedAt":"…",
 "processed":120,"current":"pelis/Akira (1988).mkv → seed-bytes","lines":["…últimas 40…"],"summary":[],"error":""}
```

`state`: `queued` · `running` · `done` · `failed`. `summary`: el informe de cada fuente (vídeos, nuevos, cambiados, movidos, ausentes, fichas por semilla/título/nombre, errores de ffprobe) y de TMDB.

| Método y ruta | Qué hace |
|---|---|
| `GET /jobs` | `{"jobs":[…]}`, del más reciente al más antiguo (20 últimos) |
| `GET /jobs/{id}` | Un trabajo |
| `GET /jobs/events` | **SSE**: evento `jobs` con `{"jobs":[…]}` al conectar y en cada cambio (máx. 4/s); latido cada 20 s |

## Catálogo

| Método y ruta | Qué hace |
|---|---|
| `GET /pending` | `{"tmdb":true,"items":[{"id","kind","title","year","identifyTried","files":[{"id","source","path","size","season","episode"}],"candidates":[{"tmdb","title","english","original","year","score","votes"}]}]}` — las fichas que TMDB no identificó con seguridad (ADR-004) |
| `GET /titles?q=&kind=` | Buscar cualquier ficha (para corregir un emparejado): `{"total","items":[{"id","kind","title","year","tmdb","matchSource","locked","poster","files":[…]}]}` (50 como mucho) |
| `GET /tmdb/search?q=&year=&kind=movie\|series` | Búsqueda manual en TMDB (es-ES; si con año no hay nada, sin año): `{"kind","items":[{"tmdb","title","original","year","votes","popularity","poster"}]}`. `poster` es una miniatura `https://image.tmdb.org/…` |
| `POST /titles/{id}/match` `{"tmdb":78,"kind":"movie"}` | Igual que `yourflix match ID --tmdb 78`: la ficha queda **bloqueada** (`manual`); si esa película ya existía, se fusiona con ella. `kind` es el tipo del resultado elegido (`movie` \| `series`; los IDs de TMDB de películas y series son espacios distintos): si no coincide con el de la ficha, la ficha cambia de tipo; sin `kind`, el de la ficha. `{"title","year","tmdb","kind","merged","locked"}` |
| `POST /identify` `{"retry":false}` | 202 con el trabajo: `yourflix identify [--retry]` |

## Estado

`GET /status`:

```json
{"version":"0.1.0","startedAt":"…","uptimeSeconds":3600,"dataDir":"C:\\…\\private\\data","os":"windows/amd64","go":"go1.26.2",
 "tools":[{"name":"ffprobe","path":"C:\\…\\ffprobe.exe","found":true,"version":"ffprobe version 9.0.2-full_build-www.gyan.dev"},{"name":"ffmpeg",…}],
 "tmdb":true,"passwordSet":true,
 "disks":[{"path":"C:\\","usedBy":["datos de YourFlix","Samsung T7","USB Patriot"],"free":912000000000,"total":2000000000000}],
 "sources":[{"id","label","state","files","lastScanAt"}],
 "scans":[{"id","sourceId","label","startedAt","seconds","report":{…},"error"}],
 "titles":251,"pending":0,"log":["…40 últimas líneas…"]}
```

`tmdb` solo dice si hay credenciales. `disks` agrupa las fuentes que están en el mismo disco (3 s máximo por disco). `scans`: los 15 últimos escaneos lanzados **desde la admin** (los periódicos de `serve` se ven en `sources[].lastScanAt` y en el registro).

| Método y ruta | Qué hace |
|---|---|
| `GET /log?n=200` | `{"lines":[…]}`: últimas líneas del registro del servidor (máx. 1000; se guardan 500 en memoria) |
| `GET /compat` | Resumen de compatibilidad con el perfil de la TV: `{"profile","total","counts":{"direct","warn","optimize"},"reasons":[{"reason","count"}],"items":[{"fileId","titleId","title","year","source","path","season","episode","status","reasons":[…]}]}` (solo `warn` y `optimize`, primero los `optimize`) |
