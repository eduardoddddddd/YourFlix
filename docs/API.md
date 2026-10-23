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

Emparejamiento y endpoints de administración (`/admin/…`).
