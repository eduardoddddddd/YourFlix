# Pendientes de corregir

Lista de cosas que el usuario va pidiendo corregir. Cada tema: qué pasa, análisis, propuesta y estado. Al arreglar uno, márcalo como hecho con la rama o el commit.

## 1. Al volver de una ficha, la lista vuelve arriba (TV y web)

**Apuntado el 27-10-2026 · estado: pendiente**

**Qué pasa:** en Películas (o Series), bajas hasta, por ejemplo, la W, entras en una ficha y al volver atrás la lista aparece arriba del todo, en «1917». Debería quedarse en la película de la que saliste.

**Se puede arreglar en los dos sitios.**

**Web (`web/js/app.js`, `web/js/screens.js`). Causa clara en el código:**
- Cada cambio de ruta borra la pantalla y la vuelve a dibujar. Al volver de `#/titulo/…` a `#/peliculas`, `S.grid` se crea de nuevo y solo carga la primera página (60 títulos, `offset: 0`).
- `show()` guarda `window.scrollY` e intenta restaurarlo después, pero la página nueva solo tiene esos 60 títulos. El navegador no puede bajar hasta la W, así que el desplazamiento se queda corto o vuelve a 0.
- Propuesta: guardar por ruta cuántos títulos había cargados y cuál se abrió. Al volver, pedir de una vez hasta ese número (`limit` hasta 500, o varias páginas seguidas) y luego situar la vista y el foco en esa tarjeta, no en una coordenada en píxeles. Alternativa: mantener vivo el DOM de la lista (oculto) mientras se ve la ficha, como hace la TV.

**TV (`tv/js/app.js`, `tv/js/screens.js`, `tv/js/nav.js`). Causa probable, falta confirmarla en la tele:**
- La TV ya guarda la pantalla de la lista en una pila y, al volver, la muestra y enfoca la tarjeta de antes (`prev.last`). En teoría, ficha → Atrás debería conservar la posición.
- Pero `app.markDirty()` (llamado al salir del reproductor, `player.js`) marca **todas** las pantallas como sucias. Al volver a la lista, `S.grid` ejecuta `reload()`: `offset 0`, `scrollTop = 0` y el foco en el primer filtro. **Después de reproducir algo, la lista siempre vuelve arriba.**
- Posible segunda causa: la lista se oculta con `display:none` (`.hidden`) mientras se ve la ficha, y Chromium pierde el `scrollTop` del contenedor. Se compensa con `scrollIntoView` al enfocar `prev.last`, pero hay que comprobarlo en la tele.
- Propuesta: que la lista no se recargue entera al volver. Solo hay que refrescar «Seguir viendo» y el progreso de la ficha, no el orden alfabético. Si hace falta recargar, cargar hasta el mismo número de títulos y volver a enfocar la misma tarjeta, buscándola por ID de título.

**Para probar:** en la TV, (a) ficha → Atrás sin reproducir y (b) ficha → reproducir → salir → Atrás. En la web, lo mismo con el botón Atrás del navegador.

## 2. Sustituir una película por una versión mejor

**Apuntado el 27-10-2026 · estado: pendiente (casi todo funciona ya; faltan detalles)**

**Qué quiere el usuario:** copiar una versión de más calidad (ejemplo real: «El odio», que hoy es `compartidos/Pelis/El Odio (Dvdrip De La Haine - Spanish) Cvcd.mpg`), borrar la vieja y que la app se entere sola.

**Cómo funciona hoy** (`server/internal/scan/scan.go`, `match.go`, `identify/identify.go`):
- **Mismo nombre y misma carpeta** (sobrescribir): el escáner ve que cambiaron el tamaño y la fecha y reutiliza el mismo registro de archivo. Vuelve a leer las pistas con ffprobe y conserva el ID y el progreso. **Funciona.**
- **Nombre distinto** (lo normal): el archivo nuevo entra como archivo nuevo y se une a la misma ficha si el título y el año coinciden con la ficha existente. Si no coinciden, queda «pendiente» hasta que el identificador lo lleva a TMDB y lo fusiona por el ID de TMDB (`TitleByTMDB`). La vieja, al borrarla, queda marcada como «ausente» en el siguiente escaneo y deja de verse en las listas y en la ficha. **Funciona: se entera sola.**

**Lo que falla o falta:**
1. **Se pierde el progreso y el «visto»:** se guardan por archivo (`progress.file_id`), no por título. La versión nueva empieza como no vista y «Seguir viendo» pierde la peli si estaba a medias. Propuesta: cuando un archivo pasa a ausente y su ficha tiene otro archivo disponible (en películas, o el mismo capítulo en series), copiar el progreso a ese archivo.
2. **Mientras conviven las dos copias**, no se elige la mejor. `FilesOfTitle` ordena por ruta y la TV elige la primera compatible (`chooseFile`), no la de más calidad. Propuesta: ordenar las copias por calidad (resolución, HDR, bitrate) y marcar en la ficha cuál es cuál («1080p HEVC» o «DVD MPEG-2» en vez de «Copia 1»).
3. **Opción «Sustituir» en la administración:** en la ficha de admin, botón para quedarse con una copia y olvidar las demás (solo en la base de datos, nunca borrar del disco: la biblioteca es de solo lectura). Útil si la vieja está en otro disco que no quieres borrar.
4. **Los registros ausentes se quedan para siempre** en la base de datos. Hoy no hay ninguno (0 ausentes), pero con el tiempo se acumulan. Propuesta: botón «Olvidar ausentes» en la admin, o limpiarlos solos tras X días si la fuente está conectada. No hacerlo si la fuente está desconectada: ahí los ausentes son temporales.

**Visto de paso:** «Los odiosos ocho» aparece con 4 «copias», pero son los 4 capítulos de la versión extendida (`Chapter.1` a `Chapter.4`). Hoy se muestran como copias alternativas, no como partes en orden. Otros títulos con 2 copias de verdad: El imperio del sol, Platoon y Delitos y faltas.
