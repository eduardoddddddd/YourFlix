# Fase 0 — pruebas de AVPlay en la TV real

Matriz de la puerta **G0** (PLAN.md §8): ¿reproduce la Samsung The Serif los vídeos de la colección servidos por HTTP, buscando dentro del archivo, sin colgarse?

## Montaje

| Pieza | Detalle |
|---|---|
| TV | QE43LS01BBUXXC, Tizen 6.5, Chromium 85.0.4183.93, AVPlay 6.0, Wi-Fi |
| Servidor | `tools/spike-server` (Go, `http.ServeContent`) en el Mac `192.168.1.3:8787`, Wi-Fi. Registra cada petición de AVPlay (Range, código, bytes, ms) y recibe las trazas de la app en `private/spike-log.jsonl` |
| App | `tools/spike-avplay`, ID `Z8YFSPIKE1.YourFlixSpike` («YourFlix Pruebas»), firmada con `CineT7`. Prueba automática por archivo y modo manual con el mando |
| Monitor | `tools/tv-monitor/cdp.mjs`: inspector remoto, latido cada 2 s y diagnóstico de bloqueo nativo |

### Prueba automática (por archivo)

`prepare` (≤ 30 s) → pistas (`getTotalTrackInfo`) → `play` hasta superar 2 s → `seekTo` 50 % → `jumpForward` 30 s → `jumpBackward` 30 s → `seekTo` 90 % → `seekTo` 10 % → si hay 2+ audios, `setSelectTrack('AUDIO', 2.ª)` → si hay subtítulos, `setSilentSubtitle(false)` 12 s contando `onsubtitlechange`. Un salto cuenta como bueno si la posición cae en el margen esperado **y** el tiempo sigue avanzando 2,5 s después. Nunca se llama a `setSelectTrack('TEXT')`.

### Archivos

Sintéticos de `tools/spike-media/make_media.sh` (imagen `testsrc` con contador de tiempo para verificar cada salto a ojo), más un MKV real:

| Archivo | Contenido | Prueba |
|---|---|---|
| `r1_no_hay_salida_hevc_2aac_pgs.mkv` | **Real**: «No hay salida» (1987), 2,37 GB, 1 h 54 min, HEVC Main 1920×1040, AAC 2.0 spa + AAC 5.1 eng, 3 PGS | S1/S5/S6 con archivo real, PGS |
| `s1_h264_ac3.mkv` | H.264 High 1080p + AC3 5.1 | S1 |
| `s2_hevc10_eac3.mkv` | HEVC Main10 10 bits + E-AC3 5.1 | S2 |
| `s3_xvid.avi`, `s3_xvid.mkv` | MPEG-4 ASP (XVID) + MP3 / AC3 | S3 |
| `s4_dts.mkv` | H.264 + DTS 5.1 | S4 |
| `s5_2audio.mkv` | H.264 + AC3 spa (440 Hz) + AAC eng (880 Hz) | S5 |
| `s6_srt.mkv` | H.264 + AAC + SRT spa (por defecto) + SRT eng | S6 |
| `s7_4k_hevc_hdr.mkv` | HEVC Main10 3840×2160, etiquetas HDR10, ~41 Mb/s, 60 s | S7 |
| `s8_h264_aac.mp4` | H.264 + AAC, MP4 `faststart` | S8 |
| `s9_big_7gb.mkv` | H.264 1080p ~42 Mb/s, 24 min, **7,64 GB** | S9 (offsets > 4 GB) |

## Cómo repetirlo

```bash
tools/spike-media/make_media.sh                       # genera private/testmedia (≈8 GB)
(cd tools/spike-server && go build -o ../../private/yourflix-spike .)
private/yourflix-spike -dir private/testmedia          # o la carpeta de vídeos reales
# compilar, firmar e instalar tools/spike-avplay como en AstroIPTV/README.md (ID Z8YFSPIKE1.YourFlixSpike)
sdb -s 192.168.1.4:26101 shell 0 debug Z8YFSPIKE1.YourFlixSpike   # imprime «port: NNNNN»
sdb -s 192.168.1.4:26101 forward tcp:9333 tcp:NNNNN
node tools/tv-monitor/cdp.mjs monitor &
node tools/tv-monitor/cdp.mjs eval 'spike.autoAll(); "iniciada"'
python3 tools/spike-media/summarize.py private/spike-log.jsonl --from <línea inicial>
```

Con el mando: ▲▼ elegir, OK reproducir, Rojo prueba automática, Azul todos, Amarillo modo 4K (`SET_MODE_4K`).

## Resultados (23-10-2026)

Tres rondas en la TV real (unas 20 pruebas automáticas por archivo y ronda), servidor y TV por **Wi-Fi**. Monitor CDP activo: **0 latidos perdidos, 0 excepciones, ningún cuelgue** en toda la sesión. La tabla es la ronda final; el tiempo de cada salto es lo que tarda el vídeo en **volver a avanzar**.

| Archivo | prepare | arranque | continuo 20 s | salto 50 % | +30 s | −30 s | salto 90 % | 10 % | audio 2.ª pista | subtítulos |
|---|---|---|---|---|---|---|---|---|---|---|
| r1 «No hay salida» HEVC 2,37 GB | ✓ 2,3 s | ✓ 4,8 s | ✓ 0 cortes | ✓ 1,3 s | ✓ 1,3 s | ✓ 1,3 s | ✓ 1,3 s | ✓ 1,5 s | ✓ (150 ms) | PGS: 0 recibidos |
| s1 H.264 + AC3 | ✓ 0,6 s | ✓ 4,8 s | ✓ 0 | ✓ 0,8 s | ✓ 0,8 s | ✓ 1,0 s | ✓ 0,8 s | ✓ 1,0 s | — | — |
| s2 HEVC Main10 + E-AC3 | ✓ 0,4 s | ✓ 4,8 s | ✓ 0 | ✓ 0,8 s | ✓ 1,3 s | ✓ 1,0 s | ✓ 0,8 s | ✓ 1,0 s | — | — |
| s3 XviD AVI / MKV | ✓ | ✓ | ✓ 0 | ✓ 0,8 s | ✓ 0,5 s | ✓ 0,5–0,8 s | ✓ 0,5 s | ✓ 0,8 s | — | — |
| s4 H.264 + **DTS** | ✓ 0,4 s | ✓ 5,3 s | — | ✓ | ✓ | ✓ | ✓ | ✓ | — | — |
| s5 dos audios | ✓ 0,5 s | ✓ 5,0 s | ✓ 0 | ✓ 0,8 s | ✓ 0,8 s | ✓ 0,8 s | ✓ 0,8 s | ✓ 1,0 s | ✓ | — |
| s6 SRT embebido | ✓ 0,3 s | ✓ 4,8 s | ✓ 0 | ✓ 0,5 s | ✓ 0,5 s | ✓ 1,0 s | ✓ 0,8 s | ✓ 0,8 s | — | ✓ texto recibido |
| s7 4K HEVC Main10 41 Mb/s | ✓ 1,0 s | ✓ 13,8 s | ✓ 0 | ✓ 4,0 s | ✗ (fuera del final)¹ | ✓ 3,8 s | ✓ 3,8 s | ✓ 3,8 s | — | — |
| s8 MP4 H.264 + AAC | ✓ 0,3 s | ✓ 4,8 s | ✓ 0 | ✓ 0,8 s | ✓ 0,8 s | ✓ 0,8 s | ✓ 0,8 s | ✓ 0,8 s | — | — |
| s9 **7,64 GB** 42 Mb/s | ✓ 0,7 s | ✓ 7,5 s | ⚠ 2 cortes² | ✓ 4,3 s | ✓ 4,3 s | ✓ 4,8 s | ✓ 5,3 s | ✓ 5,0 s | — | — |

¹ El archivo dura 60 s y el salto apuntaba más allá del final: AVPlay devuelve `PLAYER_ERROR_SEEK_FAILED`. También lo devuelve si se salta mientras aún está llenando el búfer del salto anterior (ronda 2). La app real debe **encolar saltos** y no lanzar otro hasta que el vídeo reanude.
² En la ronda 2 dio 0 cortes. A ~42 Mb/s el Wi-Fi Mac → TV va al límite.

El «arranque» incluye 2 s de reproducción a propósito: el tiempo real hasta la primera imagen es ~2,5 s (archivos normales), ~5,5 s (7,6 GB) y ~11 s (4K).

### Conclusiones para G0

1. **Reproducción directa por HTTP Range: SÍ.** MKV, MP4 y AVI se reproducen y buscan sin problemas, también un MKV real de 2,4 GB y uno de 7,6 GB con posiciones pedidas de hasta **7,64 GB** (sin límite de 4 GB). AVPlay usa siempre `Range` (206), con un `User-Agent: samsung-agent/1.1`, y lee el final del archivo al abrir (índice MKV).
2. **Códecs de vídeo:** H.264, HEVC Main y Main10 (10 bits), MPEG-4 ASP (XviD) y HEVC 4K: **todos directos**.
3. **Audio:** AC3, E-AC3, AAC (2.0 y 5.1) y MP3 directos. **DTS: no cuelga, pero AVPlay descarta la pista en silencio** (`getTotalTrackInfo` solo lista el vídeo, sin error): el vídeo se ve **sin sonido**. El servidor tiene que marcarlo y optimizarlo (DTS → E-AC3); la TV no puede detectarlo salvo comparando con las pistas del servidor.
4. **Cambio de audio** con `setSelectTrack('AUDIO')` en archivo: funciona, ~150 ms, sin cuelgue.
5. **Subtítulos:** SRT embebido llega como texto por `onsubtitlechange` con `setSilentSubtitle(false)`; **PGS no llega** (0 eventos, sin cuelgue). Se mantiene el plan: el servidor extrae los subtítulos de texto a JSON y la app los dibuja (permite elegir idioma sin `setSelectTrack('TEXT')`); PGS → OCR o sin subtítulos.
6. **Caudal Wi-Fi Mac → TV:** 51–66 Mb/s medidos. Bien hasta ~30 Mb/s (toda la colección salvo los 3 4K y remuxes); a 40+ Mb/s hay riesgo de cortes y cada salto tarda ~4–5 s. Para 4K: servidor con cable (dock del Ally) y aviso en la ficha.
7. **`SET_MODE_4K`:** con y sin él el 4K reproduce igual (20 s sin cortes). No se ha podido comprobar a ojo si cambia la resolución de salida; queda como opción en la app.

### No probado (pendiente)

- **HDR real**: el archivo de prueba no llevó las etiquetas de color (ffprobe: `color_transfer=unknown`). Probar con uno de los 3 4K de la colección.
- ~~**Servidor en el ROG Ally** (S10)~~: hecho el 24-10-2026, ver «S10» más abajo.
- Pistas con DTS **y** otra pista compatible en el mismo archivo (¿elige AVPlay la compatible?).
- ASS embebido y SRT externo (los convertirá el servidor; no dependen de AVPlay).

## S10 — servidor en el ROG Ally, probado en la TV (24-10-2026)

Servidor `serve` en el Ally (Windows 11, `192.168.1.13`, **Wi-Fi**, datos en `C:\YourFlix`), app **0.2.0 ya instalada** (sin reinstalar), lanzada desde el Mac con `sdb shell 0 debug` y monitor CDP (`tools/tv-monitor/cdp.mjs monitor`) activo toda la sesión: **0 latidos perdidos, 0 excepciones, ningún cuelgue**. Prueba automática por CDP con el reproductor de la app (`YF.player.play` / `P.key`); el salto al 50 % es un `seekTo` directo, uno cada vez.

**Descubrimiento:** la app probó la dirección de fábrica (el Mac, `192.168.1.3`, sin servidor), que falló a los 0,66 s; empezó el barrido de la /24 a los 1,09 s y encontró el Ally en la primera tanda a los **1,14 s** desde que se cargó la página. Lo guardó ella sola, así que **no hizo falta** poner la IP a mano por CDP. A los 1,35 s ya pedía `/titles`.

**Caudal Wi-Fi Ally → TV** (XHR desde la app, 3 × 40 MB con `Range`, respuesta 206): **90, 92 y 103 Mb/s**. El Mac daba 51–66 Mb/s.

| Título (fuente T7) | Vídeo | Mb/s | apertura¹ | 6 s iniciales | salto 50 %² | +30 s | −30 s | pausa / reanudar | salir |
|---|---|---|---|---|---|---|---|---|---|
| Akira | H.264 8 bits 1040p, 3 × AC3 | 3,4 | ✓ 1,54 s | ✓ 0 cortes | ✓ 0,30 s | ✓ 0 cortes | ✓ 0 cortes | ✓ se queda quieta / 0,31 s | ✓ `NONE`, vuelve a la interfaz |
| Barry Lyndon | HEVC Main10 1080p, 2 × AC3 | 2,7 | ✓ 0,82 s | ✓ 0 | ✓ 0,36 s | ✓ 0 | ✓ 0 | ✓ / 0,31 s | ✓ |
| The Punisher: One Last Kill | **HEVC Main10 2160p**, 2 × E-AC3, `SET_MODE_4K` | 14,8 | ✓ 0,82 s | ✓ 0 | ✓ 0,34 s + 1,1 s hasta volver a avanzar | ✓ 0 | ⚠ 1 parón de > 1 s justo después | ✓ / 0,31 s | ✓ |

¹ Desde `play()` hasta que el tiempo de reproducción pasa de 0,3 s (`prepareAsync` tarda menos de 1 s en los tres).
² Es lo que tarda el `seekTo` en confirmar. En ±30 s, AVPlay devuelve el tiempo de destino casi al instante, así que lo útil es contar los parones de los 5 s siguientes (vídeo en `PLAYING` sin avanzar durante más de 1 s).

Notas:
- Después se dejó el progreso de los tres archivos a 0 (`PUT /progress`), así que no aparecen en «Seguir viendo». No tenían progreso antes de la prueba.
- **Wake-on-LAN** a `A0:D0:5B:4A:4F:20` no sacó a la TV del reposo en 30 s. Se encendió poco después (no está claro si fue por el WoL, por la conexión al canal `samsung.remote.control` en `:8001` o porque la encendió el usuario).
- `sdb shell 0 debug` se quedó colgado dos veces, sin imprimir nada, justo después de despertar la TV. Funcionó tras `sdb shell 0 was_kill Z8YOURFLX1.YourFlix` (`... successfully launched ... port: NNNNN`).
- Sin comprobar: HDR (no se puede ver a ojo por CDP), 4K a más de 15 Mb/s desde el Ally (el otro 4K, Ciudadano Kane, va a 5,2 Mb/s), un DTS y subtítulos desde el T7.

## App 0.3.0 en la TV (24-10-2026)

Instalada desde el Mac (`tizen install`, mismo ID `Z8YOURFLX1.YourFlix`, perfil CineT7), lanzada con `sdb shell 0 debug` y manejada por CDP con `YF.player.key`. El servidor era el del Ally. Monitor activo toda la sesión: **0 latidos perdidos, 0 excepciones, 0 cuelgues**.

**`getTotalTrackInfo` real** («Alien contra predator», DTS spa por defecto + AC3 eng): la DTS **no aparece**. Solo se listan:

```
AUDIO index 1  {"language":"eng","channels":"6","sample_rate":"48000","bit_rate":"384000","fourCC":"audio/x-ac3"}
VIDEO index 0  {"fourCC":"video/x-h264","Width":"1280","Height":"544","Bit_rate":"0"}
TEXT  index 2–4 {"track_num":"0","track_lang":"","subtitle_type":"-1","fourCC":"text/plain"}
```

`extra_info` trae `channels` (como texto) y `fourCC` en formato MIME (`audio/x-ac3`). En Akira los índices de AVPlay son 1, 2 y 3, igual que en el archivo.

| Prueba | Resultado |
|---|---|
| a) Alien: DTS por defecto → AC3 | ✓ suena la AC3 (es la única pista que expone AVPlay). Registro: `[av] setSelectTrack AUDIO 1` seguido de `PLAYER_ERROR_INVALID_STATE` (se lanza en READY, antes de `play`). `#osdAudio`: «EN 5.1 · AUDIO/X-AC3» ✗ (se esperaba «EN 5.1 · AC3») |
| b) Akira: ▼ → Audio → OK → japonés | ✓ menú con 3 pistas, `setSelectTrack AUDIO 3` sin error, `getCurrentStreamInfo` → `jpn`, el vídeo sigue avanzando |
| b) Subtítulos | ✓ ▶ marca Subtítulos y OK abre el menú. Opciones: «Desactivados» y «Español» (sin la etiqueta «· Completos»). Cargados en 5,8 s (1266 líneas, extracción desde `C:\YourFlix\T7`); «31 AÑOS DESPUÉS DE LA TERCERA GUERRA…» visible en `#subtitle` |
| b) ▲ | ✓ quita el foco de los botones y la barra se oculta sola antes de 5 s |
| c) El Eternauta 1×1 | ✓ el botón «Siguiente ▶» es visible y se puede marcar con ▶ ▶ |
| d) Capturas | ✗ `Page.captureScreenshot` no responde en la TV (se agota el tiempo) |

Notas: el progreso de los archivos se dejó como estaba (Alien 30,796 s; Akira y El Eternauta 1×1 a 0). La preferencia de subtítulos de la app (`subPref`) quedó en «ES» porque se eligió Español en la prueba; antes probablemente era «EN». Una vez el reproductor de Akira se cerró solo mientras no se le enviaba ninguna tecla, sin errores en el registro. Repetido, el mismo recorrido fue bien. Quizá fue una tecla del mando real.

