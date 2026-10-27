# Cambios

Versiones de YourFlix. Desde la 0.4.0, el número es el mismo para el servidor, la web y la app de la TV. Antes solo llevaba número la app de la TV. El detalle de cada entrega está en «Estado» de [README.md](README.md).

## 0.4.0 — 27-10-2026

Fichas navegables, «Filtrar por» y Kiosco, en la TV y en la web. Desplegado en el Ally y en la TV y probado por el usuario con el mando y en el navegador. Plan: [docs/PLAN-NAVEGACION-KIOSCO.md](docs/PLAN-NAVEGACION-KIOSCO.md).

### Nuevo

- **Fichas navegables.** Director, reparto, país, año, década, géneros, duración y calidad son enlaces a la lista de los títulos que comparten ese dato, con su número («Ridley Scott (6)»). En la TV son píldoras que se recorren con las flechas.
- **«Filtrar por»** en Películas y Series: género, director, actor, año y década, país, duración, nota, calidad, audio, subtítulos y estado (visto, a medias, sin ver). Los filtros se suman y se quitan con ✕. En la TV se eligen con un panel lateral con salto por letra; en la web, con desplegables con búsqueda, y los filtros quedan en la dirección de la página. El índice de actores muestra los que tienen 2 o más títulos, con un interruptor «Todos».
- **Kiosco**, pestaña nueva entre Series y Buscar: fichas aleatorias a pantalla completa, ◀▶ para pasar (deslizar en el móvil), OK para Reproducir o Ver ficha, filtros (películas o series, sin ver, género, década), pase automático opcional y «Ver en kiosco» desde cualquier lista filtrada.
- **Orden por duración** en las listas.
- **Servidor:** `GET /api/v1/facets` (categorías con recuentos, según los filtros puestos); filtros nuevos y `sort=runtime` / `sort=random&seed=N` (aleatorio estable) en `/titles`; enlaces con recuentos y país en castellano en `/titles/{id}`. La API solo añade campos: las apps anteriores siguen funcionando. Ver [docs/API.md](docs/API.md).

### Arreglado

- **Volver de una ficha conserva la posición de la lista**, en la TV y en la web, también después de reproducir algo. Antes la lista volvía arriba del todo.
- **«Recién añadidas» ordena por la llegada del archivo más reciente**, no por la fecha de la ficha. Una película cuya ficha ya existía en la semilla sin archivo («No hay salida») quedaba enterrada al añadirla.
- **Web:** sin el «minibucle de 2 s» al arrancar algunos vídeos en Chrome; la recuperación de errores sigue reproduciendo en el punto correcto.

### Cambiado

- **TV:** al subir o bajar, el foco elige la fila más próxima (`nav.pick`), para que las filas cortas de la ficha no se salten.
- **Proceso:** ya no se pide revisión por otro modelo antes de fusionar (AGENTS.md).

## 0.3.1 — 24-10-2026

App de la TV: la etiqueta de audio limpia el códec que da la TV (`audio/x-ac3` → «AC3») y ya no se cambia de pista al arrancar cuando AVPlay descarta la DTS. Instalada y probada en la TV. En esas fechas el servidor pasó a correr de forma permanente en el ROG Ally (`192.168.1.13:8787`), con la web y la administración.

## 0.3.0 — 24-10-2026

App de la TV: botones de Audio, Subtítulos y Siguiente en el reproductor (▼ para bajar a ellos). Instalada y probada en la TV.

## 0.2.0 — 23-10-2026

App de la TV: subtítulos elegibles (los dibuja la app) y siguiente capítulo en las series.

## 0.1.0 — 23-10-2026

Primera versión: servidor en Go con SQLite, carga inicial del T7 desde la semilla `cine-t7-catalogo`, TMDB para lo nuevo y app de la TV con reproducción directa por AVPlay.
