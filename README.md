# YourFlix

Tu propio «Netflix» en casa, sin nube ni suscripciones: un servidor en un PC (hoy un ROG Ally con Windows 11) reúne tus carpetas de películas y series, las identifica con fichas, carátulas y reparto, y las sirve por la red local a:

| Dónde | Cómo | Qué hace |
|---|---|---|
| **Samsung TV** (Tizen) | app nativa `tv/`, con el mando | Reproduce el archivo tal cual con AVPlay (4K, HEVC 10 bits, varias pistas de audio); subtítulos elegibles, botones de audio y subtítulos en pantalla, siguiente capítulo |
| **Cualquier navegador** (móvil, Mac, PC) | `http://IP-del-servidor:8787/` | La misma colección. Lo que el navegador no lee lo adapta el servidor al vuelo (audio a AAC, o vídeo recodificado con la GPU) |
| **Administración** | `http://IP-del-servidor:8787/admin/` | Añadir carpetas con un explorador, escanear, elegir fichas dudosas de TMDB, estado del servidor, reiniciarlo. Sin contraseña desde el propio servidor; desde otro equipo, con la que se define allí la primera vez |

«Seguir viendo» es común: empiezas en la tele y sigues en el móvil en el mismo minuto. Todo es solo red local; internet solo hace falta para las fichas de lo nuevo (TMDB).

**Cómo funciona:** un único ejecutable en Go con SQLite (`server/`) + ffmpeg/ffprobe. La colección inicial sale del catálogo `cine-t7-catalogo` (sin llamar a TMDB); lo nuevo se identifica con TMDB. En el Ally corre como tarea de Windows que arranca al encender y se relanza sola ([docs/ENTORNO-ALLY.md](docs/ENTORNO-ALLY.md)).

- [PLAN.md](PLAN.md): arquitectura, fases y decisiones · [docs/DECISIONES.md](docs/DECISIONES.md): ADR · [docs/API.md](docs/API.md): API.
- [AGENTS.md](AGENTS.md): reglas para cualquier modelo que trabaje en el repo.

**Instalar en otro Windows:** copiar `C:\YourFlix\servidor` (ejecutable, ffmpeg, `web/`, `admin/`, `data/`, `secrets/`) y ejecutar una vez `instalar.cmd` como administrador (tarea de arranque, firewall TCP 8787, energía). La TV encuentra el servidor sola en la red. La app de la TV va firmada con un certificado de Samsung que caduca el **18-07-2027**: renovarlo antes y reinstalar con el mismo ID.

## Uso del servidor

```bash
cd server && go build -o ../private/yourflix ./cmd/yourflix && cd ..
Y="private/yourflix --data private/data"
$Y seed import RUTA/cine-t7-catalogo            # carga inicial: fichas y portadas del T7
$Y source add "E:\pelis" --label "Samsung T7"   # tantas fuentes como quieras
$Y source add "F:\" --label "USB Patriot" --exclude "*sample*"
$Y source scan --all
$Y --tmdb-token-file secrets/tmdb.token identify   # TMDB para lo que no está en la semilla
$Y titles --pending                              # dudosas, con sus candidatos de TMDB
$Y --tmdb-token-file secrets/tmdb.token match ID_FICHA --tmdb ID   # elegir a mano (queda bloqueada)
$Y titles                                        # catálogo
$Y --tmdb-token-file secrets/tmdb.token serve    # API en :8787; re-escanea e identifica cada 30 min
```

Compilar para el ROG Ally: `cd server && GOOS=windows GOARCH=amd64 go build -o yourflix.exe ./cmd/yourflix`; poner `ffprobe.exe` y `ffmpeg.exe` junto al ejecutable (o `--ffprobe` / `--ffmpeg`). API: [docs/API.md](docs/API.md).

**Web** ([ADR-006](docs/DECISIONES.md#adr-006--remux-en-vivo-solo-para-el-cliente-web-la-tv-sigue-con-reproducción-directa--24-10-2026)): `serve` sirve también la carpeta `web/` en `http://IP:8787/` para cualquier navegador de la red local (la busca junto al ejecutable o en la carpeta actual; si no, `--web RUTA`). Lo que el navegador no lee tal cual lo convierte ffmpeg al vuelo en HLS:

```bash
$Y serve --remux-jobs 2 --remux-cache-gb 20 --video-encoder auto   # valores por defecto
# --video-encoder auto|h264_amf|libx264|off  (off: XviD y HEVC sin soporte no se ven en el navegador)
# --remux-jobs 0                             (solo reproducción directa)
tools/web-demo/make_demo.sh /tmp/demo       # biblioteca de prueba generada con ffmpeg
node tools/web-demo/e2e.mjs http://localhost:8787 /tmp/capturas   # recorrido con Chromium (Playwright)
node --test web/tests/*.test.js
```

Administración web: `http://localhost:8787/admin/` en el propio servidor entra sin contraseña; desde otro equipo de casa, `http://192.168.1.13:8787/admin/` con la contraseña que se define allí o con `yourflix admin password` (la lee de la entrada estándar). La web vive en `admin/` (o `YOURFLIX_ADMIN_WEB`).

TMDB: token de lectura en `secrets/tmdb.token` (fuera de Git; origen: nota «TMDB API» de Joplin), o variables `TMDB_ACCESS_TOKEN` / `TMDB_API_KEY`. Sin token, todo funciona salvo la identificación de lo nuevo.

**macOS:** si una fuente está en Descargas, Documentos o un disco externo, el sistema pide permiso de privacidad al proceso que lanza el servidor. Hasta concederlo, la fuente aparece como desconectada (el servidor no se bloquea).

## App de la tele

`tv/`: app Tizen `Z8YOURFLX1.YourFlix` («YourFlix»), JavaScript puro para Chromium 85, firmada con el perfil `CineT7`.

```bash
node --test tv/tests/*.test.js                       # lógica y reglas de AstroIPTV / Chromium 85
python3 -m http.server 8790 --directory tv           # probar en el Mac: http://localhost:8790/?server=http://localhost:8787
TIZEN="$HOME/tizen-studio/tools/ide/bin/tizen"
cd tv && "$TIZEN" build-web -e "tests/*" -- . && "$TIZEN" package -t wgt -s CineT7 -- .buildResult
"$TIZEN" install -n YourFlix.wgt -t QE43LS01BBUXXC -- .buildResult   # pedir permiso antes (AGENTS.md)
```

En el navegador, `js/avplay-sim.js` simula AVPlay (y rechaza saltos solapados como la TV); en la tele no hace nada. Teclas en el Mac: flechas, Intro, Esc (Atrás), F/R (⏩/⏪), G (verde), Y (amarillo).

## Estado

**27-10-2026 — «Recién añadidas» por llegada del archivo (rama `fix/orden-anadidas`, sin desplegar).** «No hay salida (1987)» se escaneó bien pero se enlazó a la ficha que la semilla había creado el 24-10 sin archivo, y `sort=added` ordenaba por la creación de la ficha (`titles.added_at`): quedaba enterrada y parecía que no se había añadido. Ahora `sort=added` y el `addedAt` de `GET /titles` usan la llegada del archivo disponible más reciente del título (`MAX(files.added_at)` sin ausentes); una serie con un capítulo nuevo también sube. La ficha (`GET /titles/{id}`) conserva la fecha de creación. Revisado: ningún otro sitio usa `titles.added_at` como novedad (admin, «Seguir viendo» y géneros no), y los clientes no muestran `addedAt`, así que web y TV no cambian. Probado: test nuevo `store/TestListTitlesAddedUsesLatestFile` (falla sin el arreglo), `go test ./...`, `go vet ./...`, tests de TV (13) y web (8), `node --check` de `tv/js` y `web/js`; contra una copia de la base real en `127.0.0.1:8788`: «No Hay Salida» sale la primera (285 títulos, 85 ms con `limit=500`). Sin probar en la TV ni en el servidor en uso. Pendiente: desplegar cuando el usuario quiera.

**26-10-2026 — web: arreglo del «minibucle de 2 s» al arrancar (rama `fix/bucle-arranque`, sin desplegar).** Visto en Chrome headless en el Ally (2 de ~45 aperturas, intermitente): Chrome pinta el vídeo pero deja el `<video>` en readyState 2 con `currentTime` parado; hls.js lo toma por atasco y cada 2 s empuja `currentTime` (0,1 → 0,3 → 0,6 s), lo que devuelve la imagen al principio; al tercero, error fatal, y `recoverMediaError()` dejaba el vídeo en pausa en el punto de inicio (en una reanudación, en el punto de reanudar aunque se fuera más adelante) sin que la interfaz lo supiera. Ahora: `nudgeMaxRetry: 0` (sin empujones), y la recuperación rehace el MediaSource en la posición de la imagen (`requestVideoFrameCallback`) o de `currentTime` y vuelve a reproducir (hasta 3 veces; el contador se reinicia tras 30 s). Probado en Chrome y Edge headless contra una copia de los datos en `127.0.0.1:8788`: aperturas normales sin cambios; atasco forzado con el código real de hls.js → sigue en 5,1 s / 2266 s reproduciendo (antes: pausa en 0 / 2260). Sin probar: la causa interna del atasco en Chrome, móviles Android, y que el usuario no vuelva a verlo.

**24-10-2026 — arreglos de la revisión de la web en marcha** (`cba9523`): `h264_amf` con `-forced_idr 1` (IDR cada 6 s comprobado con la GPU del Ally), renombrados con reintento, máximo 2 ffprobe. Servidor reiniciado a las 20:55; web, admin y API de la TV responden. Pendiente: contraseña de la admin (desde el Ally), Safari/Firefox sin probar, segmentos largos en 12 MKV con Cues escasos, 4K HDR recodificado justo de velocidad.

**24-10-2026 — web en marcha en el servidor del Ally: `http://192.168.1.13:8787/`.** PR #3 fusionado (`38aa86e`); binario nuevo y `web/` en `C:YourFlixservidor`, reinicio con `reiniciar.cmd` (TV sin conexiones). Probado en el Ally antes de ponerlo (copia de datos, Chromium): directo (Alien: Covenant, arranque 1,2 s, salto 1,3 s), HLS copiando vídeo con audio a AAC (Akira 1,5/1,4 s; Barry Lyndon HEVC 10 bits 1,5/1,9 s; 2001 4K 1,5/2,4 s) y recodificado con `h264_amf` (XviD, 1,9/1,9 s); sonido en todos, ffmpeg cerrado al salir. Sin probar: Safari (iPhone y Mac) y Firefox. La revisión de código de Sonnet del PR #3 sigue pendiente de aplicar. En Windows, `web/vendor/**` va sin conversión de fin de línea (`.gitattributes`) para que cuadre el hash de hls.js.

**24-10-2026 — cliente web (rama `fase-web/cliente`, PR a `main` sin fusionar).** [ADR-006](docs/DECISIONES.md): remux en vivo solo para la web; la TV sigue con el archivo tal cual (ADR-001) y no se ha tocado `tv/`.

- **Hecho:** `web/` (JS sin compilar, sin CDN; hls.js 1.5.20 copiado con su licencia) servida en `/` con la misma protección solo LAN. Inicio (Seguir viendo, Añadidas, Mejor valoradas, Series, géneros), Películas y Series con orden y género, series con temporadas y capítulos, Búsqueda, Ficha (distintivos, cómo se va a reproducir en *este* navegador, reanudar, copias) y Reproductor (barra con saltos, ±10 s, pista de audio, subtítulos, pantalla completa, siguiente capítulo, teclado y toques). Sin soporte de origen a la vista. Progreso con el mismo `PUT /progress`, así que «Seguir viendo» es común con la TV. Servidor: paquete `remux` (índice de fotogramas clave del MKV, ffmpeg por tubería, segmentos fMP4 bajo demanda, salto = ffmpeg relanzado en ese segmento, 2 conversiones a la vez, freno cuando va por delante, parada a los 60 s sin peticiones, caché en `DATOS/remux`), rutas nuevas `/remux`, `/files/{id}/hls/…` y `/files/{id}/vtt/{sid}` ([docs/API.md](docs/API.md)), opciones `--web`, `--remux-jobs`, `--remux-cache-gb`, `--video-encoder`. Decisión XviD/HEVC sin soporte: **se recodifican a H.264** (`h264_amf` si funciona, si no `libx264`; `off` para avisar en vez de convertir).
- **Probado en la nube (Linux, ffmpeg 6.1, sin LAN, sin TV ni archivos reales):** `go test ./...` y `go vet ./...` (también `GOOS=windows`), con `-race` en `remux` y `api`; los 13 tests de `tv/` y los de `web/`; `node --check` de todos los JS. Remux real con ffmpeg sobre vídeos generados: MKV H.264 + AC3 5.1, MKV HEVC Main10 + E-AC3 y AVI XviD + MP3 (y HEVC recodificado para navegadores sin HEVC); ffprobe confirma códecs (h264/hevc + AAC estéreo), que cada segmento decodifica limpio y que el segmento de un salto a la mitad empieza exactamente en su corte (vídeo ±2 ms; audio sin hueco); el demuxer HLS de ffmpeg lee la lista entera por HTTP y la duración cuadra. Índice Cues del MKV = fotogramas clave de ffprobe. Película generada de 2 h (MKV H.264 + AC3 5.1, 760 MB): lista de 692 segmentos en 0,05 s; salto en frío a 1:45:00 → segmento en 0,8 s y el siguiente en 0,4 s; salto atrás a 0:30:00 en 0,9 s (máquina de 4 núcleos en la nube). Recorrido en Chromium (Playwright, escritorio y móvil) contra el servidor con la biblioteca de `tools/web-demo/make_demo.sh`: directo (MP4), HLS copiado (MKV AC3) con cambio de pista y subtítulos internos, HLS recodificado (HEVC 10 bits y XviD), salto a la mitad en todos, subtítulos externos, progreso guardado, ffmpeg parado al salir, sin errores de JavaScript.
- **Sin probar (para la sesión del Ally):** iPhone (Safari con HLS nativo, HEVC copiado, pantalla completa nativa con subtítulos), Android (Chrome), Mac (Safari y Chrome) y Windows (Edge/Chrome con HEVC por hardware; Firefox recodificando) contra `192.168.1.13:8787`; `h264_amf` en el Z1 (detección, fotogramas clave forzados, calidad) y CPU/tiempo real con 4K HEVC 10 bits recodificado; saltos y caché con los archivos reales de decenas de GB; HDR copiado en Safari; que la regla del firewall deje entrar a los móviles igual que a la TV.
- **Siguiente paso:** revisar y fusionar el PR (choca solo en `main.go` si la administración también lo toca), compilar en el Ally con `ffmpeg.exe` junto al ejecutable, arrancar `serve` desde la raíz del repo (para que encuentre `web/`) y probar en los cuatro tipos de dispositivo.

**24-10-2026 — administración en marcha en el servidor del Ally (`http://localhost:8787/admin/` desde el Ally; desde otros equipos, con contraseña).**

- Hecho: PR #2 fusionado (`9febe12`) y binario nuevo con `admin/` en `C:\YourFlix\servidor`. El proceso de la tarea (S4U) no se puede matar desde la sesión del usuario (`taskkill`: «Acceso denegado»): ahora `iniciar.cmd` cierra al arrancar cualquier `yourflix.exe` huérfano (corre en la misma sesión que la tarea) y el servidor se reinicia por la admin (`POST /admin/api/restart`, botón «Reiniciar servidor», `reiniciar.cmd`). Ya no hace falta ir al Ally ni ser administrador para actualizar.
- Probado en el Ally: `schtasks /run` sustituyó el proceso viejo por el nuevo sin intervención; ping, 251 títulos, Range 206, `/admin/` 200 en localhost y 401 desde la LAN sin contraseña, `admin.js` con `text/javascript`; `reiniciar.cmd` → servidor de vuelta en 3 s con otro PID.
- Pendiente: el usuario define la contraseña de la admin desde el Ally (`http://localhost:8787/admin/`, Estado) para entrar desde el Mac o el móvil. `yourflix.old.exe` se queda como vuelta atrás. La TV no se ha probado contra el binario nuevo (la API pública no cambia).

**24-10-2026 — fase 5: interfaz web de administración (rama `fase-5/admin`, PR a `main`, sin fusionar). Hecha en una sesión en la nube, sin acceso a la LAN ni a la biblioteca real.**

- Hecho: paquete `server/internal/admin` (un `http.Handler` para `/admin/`) y web `admin/` (HTML, CSS y JS propios, sin CDN, colores y tipografía de la app de la TV, responsive). **Fuentes:** lista con ruta, tipo, estado, archivos y último escaneo; alta con **explorador de carpetas del servidor** (unidades de Windows → carpetas; `/` y atajos en macOS/Linux); editar etiqueta, tipo y exclusiones; pausar, reanudar, quitar (no borra nada del disco); escanear una o todas con **progreso en vivo por SSE** (y sondeo de respaldo) y TMDB después, como `serve`. **Catálogo:** pendientes con sus candidatos y «Elegir» con un clic (igual que `yourflix match`: bloquea la ficha), búsqueda manual en TMDB por título y año, «Identificar ahora» (con o sin reintento), y corregir cualquier ficha. **Estado:** versión, encendido, carpeta de datos, ffprobe/ffmpeg (ruta y versión), espacio libre por disco, últimos escaneos, registro. **Compatibilidad:** archivos `warn`/`optimize` del perfil de la TV con sus motivos y filtros. **Seguridad:** solo LAN; `localhost` sin contraseña (comprobando también el `Host`); contraseña PBKDF2 y cookie de sesión; CSRF (JSON + `Origin` + token); 5 contraseñas malas seguidas bloquean esa IP un minuto; el token de TMDB no sale ni en respuestas ni en el registro. CLI `yourflix admin password`. API en [docs/API.md](docs/API.md#api-de-administración--adminapi); decisión en ADR-007.
- **Probado en la nube** (Linux, Go 1.26.2): `go test ./...` con 9 tests nuevos (`httptest`, base de datos y carpetas temporales: LAN, sesión local y remota, DNS rebinding, CSRF, cambio de contraseña por web y por CLI, explorador, alta/edición/pausa/quitar, escaneo real, SSE, pendientes y emparejado con un TMDB simulado, ningún rastro del token, compatibilidad), también con `-race`; `go vet` para linux, windows y darwin; binario `windows/amd64` compilado; `node --check admin/admin.js`; 13 tests de la app. **En Chromium** (Playwright) contra el binario con las dos líneas de montaje puestas a mano: alta de una carpeta con el explorador, escaneo de vídeos reales generados con ffmpeg (ffprobe real), edición, catálogo, compatibilidad (detecta el DTS), estado, entrada remota con contraseña (`Host` no local) y salida, vista de móvil (390 px) sin desbordes y 0 errores de CSP.
- **Sin probar** (pendiente para la sesión del Ally): montarla en `main.go` (línea en el PR y en `server/cmd/yourflix/admin.go`); el explorador con **unidades de Windows** (`C:\`, `D:\`) y el espacio libre con `GetDiskFreeSpaceExW` (solo compilado, no ejecutado); TMDB real (búsqueda y emparejado solo contra un simulador); escanear `C:\YourFlix\T7` (462 vídeos) desde la web; entrar desde el Mac o el móvil por la LAN real; que `scanOne` use el candado `scanMu` para no coincidir con el re-escaneo periódico.
- **Revisión e integración en el Ally (Opus, revisión de código por Sonnet):** sin bloqueos; arreglado lo que pedía cambio: el límite de intentos de contraseña se cuenta antes de comprobarla (20 intentos a la vez ya no pasan todos) y el PBKDF2 va de uno en uno; quitar una fuente durante un escaneo responde 409 en vez de esperar; las carpetas de red (`\\equipo\carpeta`) se rechazan; un proxy local (cabeceras `X-Forwarded-For`, `Forwarded`, `Via`) no entra sin contraseña; tipo fijo para `.js`/`.css` (el registro de Windows puede decir `text/plain`). Test nuevo con todo ello. Montada en `main.go` (`withAdmin`, `yourflix admin password`, `scanMu` en `scanOne`).
- **Probado en el Ally** (binario de la rama en `127.0.0.1:8788` con una copia de los datos; el servidor en uso no se tocó): explorador con las unidades `C:` y `D:` y `C:\YourFlix`; espacio libre de `C:` (788 GB); búsqueda real en TMDB («Akira» 1988); compatibilidad (38 avisos); escanear todo desde la web; CSRF (403 sin token) y 401 desde la LAN sin contraseña; el registro no muestra el token.
- Pendiente: fusionar en `main`, copiar `admin/` junto a `C:\YourFlix\servidor\yourflix.exe` y cambiar el binario (corte breve, con permiso); definir la contraseña desde el Ally y entrar desde el Mac o el móvil. Quedan como mejoras menores de la revisión: re-comprobar la sesión en el SSE abierto, agrupar discos montados en carpetas y el «quitar» que borra el progreso (PLAN §3.5 prefiere pausar).

**24-10-2026 — servidor permanente en marcha en `C:\YourFlix\servidor`.** Detalle y procedimiento en [docs/ENTORNO-ALLY.md](docs/ENTORNO-ALLY.md#servidor-permanente-en-cyourflixservidor-24-10-2026).

- Hecho: binario de `main`, ffmpeg/ffprobe, token y cachés copiados fuera del worktree; `tools/ally/` con `iniciar.cmd` (relanza si se cae, registro con rotación), `parar.cmd`, `instalar-servicio.ps1` (tarea al encender con el usuario de Windows, que la arranca al terminar, firewall TCP 8787 para ese ejecutable, energía sin suspensión con cargador) y `migrar-datos.mjs` (copia la base de datos y corrige las rutas absolutas de las carátulas).
- Probado en el Ally, sin cortar el servidor en uso: copia de prueba en `127.0.0.1:8788` idéntica al servidor actual (251 títulos, carátulas, Range 206, subtítulos, progreso).
- **Cambio hecho (20:04):** el usuario ejecutó `instalar.cmd` en el Ally (la tarea «YourFlix» corre con su usuario en modo S4U; el registro de la instalación termina en «LISTO»). Base de datos copiada con `migrar-datos.mjs` justo antes; se paró el `serve` del worktree y el nuevo ocupó el puerto 8787 en 6 s. Comprobado en `192.168.1.13:8787`: ping, 251 títulos, carátulas, vídeo con Range 206, progreso intacto (Alien 30,796 s) y «Seguir viendo» con 6 elementos. El `serverId` no cambia, así que la TV sigue usando la dirección que tenía guardada. Desde el Mac (otro equipo de la LAN, a través de la regla de firewall nueva): ping correcto y 10 MB con Range (206) en 0,70 s, unos 120 Mb/s.
- Sin comprobar todavía: el arranque tras reiniciar el Ally (la tarea se dispara al encender), y que la TV cargue una película contra el servidor nuevo. `private\` del worktree queda como copia de seguridad.

**24-10-2026 — app 0.3.1 instalada y probada en la TV** (desde el Mac, mismo ID, firmada con CineT7, monitor CDP activo: 0 latidos perdidos, 0 excepciones, 0 cuelgues).

- **Probado en la TV:** en «Alien contra predator», `#osdAudio` dice «EN 5.1 · AC3» y el registro no tiene ningún `setSelectTrack` al arrancar. En Akira el menú de audio dice «ES 5.1 · AC3», «ES estéreo · AC3» y «JA estéreo · AC3», y el cambio a japonés funcionó 3 de 3 veces (`setSelectTrack AUDIO 3`, `getCurrentStreamInfo` → índice 3, el vídeo sigue avanzando). Apertura en 0,7–1,8 s.
- **Cierre espontáneo de la 0.3.0:** no se repitió. Akira estuvo 3 minutos reproduciéndose sin tocar nada (de 23 a 195 s), con registros de diagnóstico inyectados por CDP (visibilitychange, keydown, eventos de AVPlay, errores de ventana): ningún evento ni tecla. Lo más probable es que aquella vez fuera una tecla del mando real.
- Progreso restaurado (Alien 30,796 s, Akira 0) y `subPref` en «EN». No se sabe con certeza qué valor tenía antes de la primera prueba; «EN» encaja con que Alien eligiera solo los subtítulos en inglés.

**24-10-2026 — app 0.3.1: arreglos de la prueba en la TV (sin instalar todavía).**

- Hecho: la etiqueta de audio limpia el `fourCC` real de la TV (`audio/x-ac3` → «AC3»). Ya no se llama a `setSelectTrack` al arrancar cuando AVPlay ha descartado la DTS (es lo que hace la TV real: suena sola la pista compatible); si algún día la listara, el cambio se hace después de `play()`, no en READY. El simulador da el `fourCC` como la TV. El fallo (3) no lo era: Akira solo tiene una pista de subtítulos en español y el servidor la llama «Español».
- Probado: 13 tests; en Chromium con AVPlay simulado contra el Ally, Alien muestra «EN 5.1 · AC3» y Akira «ES 5.1 · AC3 (1/3)», sin cambios de pista al arrancar. Progreso intacto.
- Pendiente: instalar la 0.3.1 en la TV. Observado una vez en la prueba de la 0.3.0: el reproductor de Akira se cerró solo sin errores (posiblemente una tecla del mando real); vigilarlo.

**24-10-2026 — app 0.3.0 instalada y probada en la TV** (desde el Mac, mismo ID, firmada con CineT7, monitor CDP activo: 0 latidos perdidos, 0 excepciones, 0 cuelgues). Detalle en [docs/PRUEBAS-TV.md](docs/PRUEBAS-TV.md#app-030-en-la-tv-24-10-2026).

- **Probado en la TV:** en «Alien contra predator» (DTS española por defecto + AC3 inglesa) suena la AC3. ▼ marca el botón Audio, y OK abre el menú con las 3 pistas de Akira; al elegir la japonesa, `setSelectTrack AUDIO 3` funciona y `getCurrentStreamInfo` devuelve `jpn`. El menú de subtítulos con «Español» muestra líneas en `#subtitle` (1266 líneas, cargadas en 5,8 s). ▲ vuelve al vídeo y la barra se oculta sola. En El Eternauta 1×1 sale «Siguiente ▶» y se puede marcar.
- **Fallos para `tv/`:** (1) `fourCC` llega como `audio/x-ac3`, así que la etiqueta dice «EN 5.1 · AUDIO/X-AC3» y no «EN 5.1 · AC3». (2) AVPlay **no lista la pista DTS** en `getTotalTrackInfo` (en Alien solo aparece la AC3, con index 1), y el `setSelectTrack AUDIO 1` del arranque, lanzado antes de `play()`, devuelve `PLAYER_ERROR_INVALID_STATE`. No hace daño, porque AVPlay ya suena con la única pista que expone, pero el cambio en estado READY no vale. (3) En Akira el menú de subtítulos pone «Español», no «Español · Completos».
- Sin comprobar: capturas, porque `Page.captureScreenshot` no responde en la TV.

**24-10-2026 — app de la TV 0.3.0: botones de audio y subtítulos en pantalla (sin instalar todavía).**

- Hecho: en el reproductor, ▼ baja a una fila de botones (Audio, Subtítulos, Siguiente); ◀ ▶ para moverse, OK abre el menú, ▲ o Atrás vuelve al vídeo. El mando de The Serif no tiene botones de color, que siguen funcionando (verde y amarillo). Menú de audio con idioma, canales y códec («ES 5.1 · AC3»). Si la pista por defecto es DTS (AVPlay la descarta y la tele se queda muda), la app pasa sola a la primera pista que sí suena; si solo hay DTS, lo avisa. La ficha ya no muestra el soporte de origen; las dos fuentes se llaman «Ally» (cambiado en la base de datos, sin reiniciar el servidor). El simulador de AVPlay descarta las pistas DTS como la TV.
- Probado: 13 tests de la app; en Chromium con AVPlay simulado contra el servidor real del Ally: «Alien contra predator» (DTS por defecto) arranca con la AC3 inglesa; Akira muestra 3 pistas y cambia a la japonesa; cambiar el audio durante la carga avisa en vez de ignorarse; 0 cuelgues del simulador. El progreso de esos dos archivos se dejó como estaba.
- **No probado en la TV**: hay que compilar, firmar e instalar la 0.3.0 desde el Mac (pedir permiso al usuario y no hacerlo mientras ve algo).
- Decidido por el usuario: **no se convierte el DTS** (hay 9 archivos y no le preocupan); se quita de la fase 4.
- En marcha: cliente web (remux en vivo) y administración, encargados a sesiones en la nube (se lanzan desde la sesión del Mac).

**24-10-2026 — S10: servidor en el ROG Ally, probado en la TV real.** Detalle en [docs/PRUEBAS-TV.md](docs/PRUEBAS-TV.md#s10--servidor-en-el-rog-ally-probado-en-la-tv-24-10-2026).

- **Probado en la TV** (app 0.2.0 sin reinstalar, lanzada desde el Mac con inspector y monitor CDP: 0 cuelgues, 0 excepciones): la app descubre el Ally (`192.168.1.13`) **sola en 1,14 s**, porque la IP del Mac falla al momento y el barrido de la /24 lo encuentra en la primera tanda. No hizo falta guardar la IP a mano. Caudal Wi-Fi Ally → TV: **90–103 Mb/s**. Akira (H.264), Barry Lyndon (HEVC 10 bits) y The Punisher: One Last Kill (**4K HEVC 10 bits, 14,8 Mb/s**): apertura en 0,8–1,5 s, salto al 50 % en 0,3–0,4 s, ±30 s, pausa y reanudación (0,31 s) y salida limpia. Un solo parón de más de 1 s, en el 4K, justo después de −30 s.
- El ping desde otro equipo de la LAN queda comprobado: la TV llega a `:8787` con el firewall que aceptó el usuario.
- Pendiente: HDR a ojo, DTS → E-AC3 (fase 4), subtítulos extraídos desde el T7 en el Ally, servicio de Windows (fase 5). La dirección de fábrica de la app sigue siendo la del Mac: no hace falta cambiarla mientras el barrido tarde ~1 s, pero hay que actualizarla cuando el Ally pase a cable.
- Siguiente paso: fase 4 o la refactorización que plantee el usuario.

**24-10-2026 — el trabajo pasa al ROG Ally: T7 y Patriot copiados a C:, semilla cargada y fuentes escaneadas.** Detalle operativo y relevo para el siguiente modelo en [docs/ENTORNO-ALLY.md](docs/ENTORNO-ALLY.md).

- Hecho: en el Ally (Windows 11, `192.168.1.13` por **Wi-Fi**, reservada en el DHCP del usuario) `go vet`, `go test ./...` y los 12 tests de la app pasan; `private/yourflix.exe` compilado (windows/amd64) junto a `ffprobe.exe` (ffmpeg 9.0.2 gyan en el PATH). El contenido del **T7** (`pelis` y `pelis2`, 882 GB, 463 archivos) y del **USB Patriot** (104 GB, 34 vídeos) se copió con `robocopy` a `C:\YourFlix\T7` y `C:\YourFlix\Patriot` (sin `._*`, papeleras ni carpetas de sistema); mismo número de archivos y bytes que el origen, 0 errores (sin comparar hashes de contenido). Semilla importada desde un clon de `cine-t7-catalogo` en `private/cine-t7-catalogo`. Dos fuentes registradas: «Samsung T7» y «USB Patriot». Token de TMDB en `secrets/tmdb.token` y token del clipper de Joplin en `secrets/joplin.token` (ambos fuera de Git).
- Probado de verdad (en el Ally, sin TV): T7 → 462 vídeos, **460 emparejados con la semilla por bytes**, 2 por título, 0 solo por nombre, 0 errores de ffprobe, escaneo en 13 s, sin llamadas a TMDB. Patriot → 34 vídeos, **34 por bytes**, escaneo en 0,5 s. 251 títulos disponibles, 0 pendientes de TMDB. Token de TMDB comprobado (HTTP 200). Velocidad de copia ≈200 MB/s (T7) y ≈110 MB/s (Patriot): el cuello está en la lectura por USB (causa exacta sin investigar).
- **Red:** la IP actual del Ally, `192.168.1.13`, es la de la **Wi-Fi** y ya está reservada en el DHCP del usuario; sirve para todas las pruebas. Cuando el Ally pase a **cable (dock con Ethernet)** tendrá otra IP fija que el usuario comunicará; entonces hay que actualizar la dirección de fábrica de la app y esta nota.
- **`serve` arrancado en el Ally (24-10-2026, 14:28):** escucha en `:8787` (IPv4 e IPv6); re-escaneo inicial de las dos fuentes sin cambios; `http://192.168.1.13:8787/api/v1/ping` → 200 y `/titles` → 200, **pedidos desde el propio Ally** (no prueba que llegue otro equipo de la LAN). Proceso en primer plano de una sesión, no servicio: se para si se cierra la sesión o se reinicia el Ally.
- **Firewall:** el usuario aceptó el aviso de Windows al arrancar `serve`, que creó dos reglas de entrada «yourflix.exe» (TCP y UDP, perfil Público, para el binario del worktree). La Wi-Fi del Ally está como red **Pública**, así que esas reglas son las que aplican. Se quedan así; la API rechaza IP que no sean privadas. Si algún día la Wi-Fi pasa a Privada o el binario cambia de ruta (servicio, fase 5), habrá que rehacer la regla.
- **No hecho / no probado:** no se ha tocado la TV (la app instalada sigue siendo la 0.2.0, que busca el servidor en el Mac y luego barre la /24), no hay servicio de Windows, no se midió el caudal Wi-Fi a la TV (S7/S10), y la copia se verificó por número de archivos y bytes, no por hash. Un `.txt` de torrent suelto en `T7\pelis2\Batman.1966…` lo ignora el escáner.
- Siguiente paso previsto (S10): (1) ~~arrancar `serve`~~ hecho; (2) ~~firewall~~ hecho (aviso de Windows aceptado); falta comprobar `ping` desde otro equipo; (3) con permiso y la TV libre, abrir la app, comprobar que descubre `192.168.1.13`, reproducir H.264, HEVC 10 bits y el 4K y medir el caudal Wi-Fi; (4) fase 4 (DTS → E-AC3) y fase 5 (servicio de Windows, admin web, energía del Ally). El usuario puede plantear una refactorización antes de seguir.

**23-10-2026 — fase 3: subtítulos elegibles y siguiente capítulo (app 0.2.0).**

- Hecho: el servidor lista y convierte subtítulos de texto internos y externos (`.srt/.ass/.vtt` junto al vídeo, UTF-8 o Windows-1252), extrae todas las pistas internas de una pasada con ffmpeg y las cachea; la app los dibuja por tiempo (AVPlay nunca toca pistas de texto). Selector con el amarillo; recuerda el idioma; los **forzados** del idioma del audio se activan solos. Al terminar un capítulo, cuenta atrás de 10 s al siguiente (OK ya, Atrás salir); azul = siguiente. «Seguir viendo» propone el siguiente capítulo tras terminar uno. Icono nuevo (`tools/icon/make_icon.py`) y logo del menú.
- Probado: `go test ./...` (SRT con BOM/CRLF/cp1252, archivos externos, extracción real con ffmpeg); 12 tests de la app; en Chromium con AVPlay simulado: forzados automáticos, menú, cuenta atrás → 1×04, 1×03 visto, «Siguiente: The Bear 1×04». Vídeos de prueba reales en `private/testseries` (1×03 con forzados, 1×04).
- **Probado en la TV real (0.2.0):** forzados en español activados solos en 1×03 y visibles; al acabar, «A continuación: 1×04 · Los perros» y 1×04 reproduciéndose; en «s6» el menú ofrece Español/Inglés y el inglés se ve. Monitor CDP: 0 cuelgues, 0 excepciones.
- Pendiente: la primera extracción de subtítulos en discos USB lentos (T7 en el ROG Ally).

**23-10-2026 — fase 2 (app de la tele) instalada en la TV.**

- Hecho: Inicio con héroe que sigue al foco y filas (Seguir viendo, Añadidas, Mejor valoradas, Series, géneros); rejillas de Películas y Series con orden y géneros; ficha (sinopsis, reparto, técnica, avisos de compatibilidad, reanudar, copias); series con temporadas y capítulos (nombre, foto, visto); búsqueda con teclado en pantalla (título, director, reparto); ajustes con IP manual; descubrimiento del servidor (guardado → Mac → barrido de la /24); reproductor con cola de saltos (sin solapes: reanudar, arranque, pausa), cambio de audio, subtítulos de texto, progreso cada 15 s. Servidor: `/continue`, `/genres`, `genre=` y `sort=rating`.
- Probado: 10 tests de Node; recorrido completo en Chromium del Mac con AVPlay simulado (reanudar + ⏩ durante la carga, ◀ ×5, ⏩ en pausa: 0 saltos solapados). En la TV real: instalada, encuentra el servidor del Mac, Inicio con 8 filas y carátulas, 10 MB de memoria JS.
- **El usuario la probó con el mando en la TV: va bien.** Reproducción de «No Hay Salida» desde la app con el inspector: apertura en ~1 s, pausa y reanudación, salida limpia; 0 cuelgues ni excepciones.
- Pendiente (fase 3/4): subtítulos extraídos por el servidor (elegir idioma), siguiente capítulo automático, optimizar DTS.

**23-10-2026 — fase 1B terminada: TMDB para lo que no está en la semilla.**

- Hecho: cliente TMDB (token v4 o clave v3, caché de 30 días en SQLite, ~20 peticiones/s, reintento en 429, credenciales fuera de caché y registros); búsqueda en español **e inglés**, puntuación por título (es/en/original), año y popularidad; asignación solo con coincidencia clara, si no queda **pendiente con candidatos**; fusión con fichas existentes (un archivo nuevo de algo que ya está en la semilla); series con nombre, sinopsis y foto de cada capítulo (también las de la semilla); `yourflix identify`, `titles --pending`, `match` (manual, bloquea la ficha); identificación automática en `serve` tras cada escaneo.
- Probado con TMDB real (15 nombres típicos de descargas): Parasite → «Parásitos», Alien sin año → la de 1979, El laberinto del fauno, Los otros, Tesis, Torrente 2, La sociedad de la nieve, Amélie, La casa de papel; nombres basura («s4», `VID_2024…`, `pelicula_final_v2`) quedan pendientes. `go test ./...` con un TMDB simulado.
- Límite conocido: un archivo **sin año** se empareja con la semilla si hay una sola ficha con ese título («Dracula.mkv» → Drácula 2025); con varias, queda para TMDB.
- Sigue sin probar: el T7 conectado y el servidor en el ROG Ally.
- Siguiente paso: **fase 2** (app de la tele contra esta API).

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
