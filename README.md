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
$Y --tmdb-token-file secrets/tmdb.token identify   # TMDB para lo que no está en la semilla
$Y titles --pending                              # dudosas, con sus candidatos de TMDB
$Y --tmdb-token-file secrets/tmdb.token match ID_FICHA --tmdb ID   # elegir a mano (queda bloqueada)
$Y titles                                        # catálogo
$Y --tmdb-token-file secrets/tmdb.token serve    # API en :8787; re-escanea e identifica cada 30 min
```

Compilar para el ROG Ally: `cd server && GOOS=windows GOARCH=amd64 go build -o yourflix.exe ./cmd/yourflix`; poner `ffprobe.exe` junto al ejecutable. API: [docs/API.md](docs/API.md).

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
