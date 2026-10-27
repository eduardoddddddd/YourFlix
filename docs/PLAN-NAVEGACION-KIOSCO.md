# Plan: fichas navegables y Kiosco

**27-10-2026 · plan, nada implementado.** Saca de [PLAN-INTERFAZ.md](PLAN-INTERFAZ.md) las partes §1, §2 y §6 (fichas que enlazan, explorar con filtros, TV) y añade una pestaña nueva, **Kiosco**. Es para la TV y para la web a la vez. Incluye el tema 1 de [PENDIENTES.md](PENDIENTES.md) (volver a la lista sin perder la posición), porque con la navegación nueva se va atrás mucho más.

## Qué hay en las fichas hoy (catálogo real, 290 títulos: 269 películas y 21 series)

| Dato | Cobertura | Riqueza |
|---|---|---|
| Director (lista) | 288 / 290 | 229 directores; 49 con 2 o más títulos (Oliver Stone 8, Ridley Scott 6, Peter Jackson 6, Scorsese 5…) |
| Reparto (10 primeros) | 289 / 290 | 2.278 actores; 368 con 2 o más títulos (Harrison Ford 7, Willem Dafoe 7, Josh Brolin 7…) |
| País (texto en inglés, separado por comas) | 290 / 290 | 33 países (EE. UU. 229, Reino Unido 58, Francia 32, España 25, Alemania 14…) |
| Año | 290 / 290 | Décadas de los 30 a los 2020 (2020s 83, 1980s 61, 2010s 37, 1990s 36, 1970s 34…) |
| Géneros | 290 / 290 | Ya se filtra por género |
| Duración (`runtime`) | 273 / 290 | Tramos: <90, 90–120, >120 min |
| Nota TMDB (`rating`, `votes`) | 290 / 290 | Nota mínima: 6, 7, 8 |
| Lema, sinopsis, título original, tráiler | 220 / 290 / 290 / 139 | Para mostrar, no para filtrar |
| **De los archivos**: resolución, HDR, idiomas de audio, subtítulos | todos | Calidad (4K, HDR, 1080p, SD), audio (castellano, VO), subtítulos |
| **Del progreso**: visto, a medias, sin ver | todos | Estado |

No hay fotos de actores, sagas, guionista ni idioma original: no están en la semilla del T7. Pedirlos a TMDB contradice AGENTS.md (§7 de PLAN-INTERFAZ, pendiente de decidir). **Fuera de este plan.**

## Lo que verá el usuario

### A. Fichas navegables

**En la ficha (TV y web):** debajo de la sinopsis, filas de enlaces:
- **Dirección:** Ridley Scott *(6)*
- **Reparto:** los 10 actores; el número de títulos aparece solo si es 2 o más
- **País · Año · Década · Géneros · Duración**

En la TV son píldoras que se recorren con las flechas. En la web son enlaces. El número entre paréntesis dice cuántos títulos de la colección comparten ese dato.

**Al elegir uno**, se abre la lista de Películas (o Series) filtrada: «Películas · Ridley Scott · 6 títulos». Arriba aparecen los filtros activos como píldoras con ✕. Se pueden **sumar**: desde Ridley Scott, añadir «Década 80» o «Sin ver».

**En Películas y Series**, la fila de géneros pasa a ser una fila de **«Filtrar por»**: Género · Director · Actor · Año/Década · País · Más (duración, nota, calidad, audio, subtítulos, estado). Cada uno abre un selector:
- **TV:** panel lateral con la lista y su número. ▲▼ recorre, OK elige y la rejilla se actualiza al momento. En las listas largas (directores, actores), salto por letra con ◀▶.
- **Web:** desplegable con caja de búsqueda («scor» → Martin Scorsese 5).
- **Índice de actores:** por defecto solo los que tienen 2 o más títulos (368). Interruptor «Todos» para ver los 2.278. Desde una ficha se llega a cualquiera.

**Ordenar:** se mantienen A→Z, Añadidas, Año y Valoración, y se añade **Duración**.

**Web:** los filtros van en la dirección de la página (`#/peliculas?director=Ridley%20Scott&decada=1980`), así se pueden guardar y el botón Atrás funciona. **TV:** pila de pantallas como hoy, sin perder la posición al volver (tema 1).

### B. Kiosco

Nueva pestaña en el menú, entre Series y Buscar: **Inicio · Películas · Series · Kiosco · Buscar**.

- **Pantalla completa, una ficha cada vez**, en orden aleatorio: imagen de fondo a 1920 px de borde a borde, con degradado; carátula; título; año · duración · nota · país; géneros; director y 5 actores; lema; sinopsis; insignias de calidad (4K, HDR, VOSE) y el progreso si está a medias.
- **◀ ▶** (flechas en la web, deslizar en el móvil) pasan a la anterior o la siguiente, con un fundido corto. La siguiente imagen ya está cargada, así que el cambio es inmediato. La anterior es la misma de antes: el orden aleatorio es fijo durante la sesión.
- **OK / Intro:** botones **Reproducir** (seleccionado) · **Ver ficha**. ▲▼ cambia entre ellos.
- **Filtro del kiosco** (▼ en la TV, píldoras en la web): Películas / Series / Todo · Sin ver · un género · una década. También desde cualquier lista filtrada: **«Ver en kiosco»** («Kiosco de Ridley Scott»).
- **Pase automático** (opcional, apagado por defecto): cambia cada 20 s. Se para al tocar cualquier tecla. Sirve de salvapantallas.
- **Atrás** vuelve al menú. En la web, la dirección guarda la semilla y la posición (`#/kiosco?semilla=8213&i=14`), así que al volver de una ficha se sigue en el mismo título.

## Cómo se hace

### Servidor (`server/`)

1. **Índice de categorías en memoria** (`internal/facets`, nuevo). Lee las fichas y los archivos disponibles y saca, por título: directores, actores, países (separados y **traducidos al castellano** con una tabla fija de unos 40 nombres de TMDB: «United States of America» → «Estados Unidos»), año, década, tramo de duración, nota, calidad, idiomas de audio y de subtítulos. Con 290 títulos se calcula en milisegundos. Se rehace cuando cambia la base de datos (tras escanear, identificar o editar fichas) y no hace falta ninguna tabla nueva.
2. **`GET /api/v1/facets?kind=movie&…filtros`**: cada categoría con sus valores y cuántos títulos tiene cada uno, **teniendo en cuenta los filtros ya puestos** (si está «Ridley Scott», las décadas que salen son las suyas). Los actores con 1 título solo salen con `all=1`.
3. **`GET /api/v1/titles` con filtros nuevos**, combinables entre sí con Y: `director`, `actor`, `person` (director o actor), `country`, `year`, `decade`, `runtime` (`short` | `mid` | `long`), `rating` (mínima), `quality` (`4k` | `hdr` | `1080` | `sd`), `audio`, `subs`, `state` (`unwatched` | `partial` | `watched`). Orden nuevo: `runtime` y `random&seed=N` (orden aleatorio estable por semilla, para poder paginar y volver atrás).
4. **Ficha (`/titles/{id}`)**: cada director, actor y país viene con su número de títulos en la colección (`{"name":"Ridley Scott","count":6}`) y el país ya en castellano.
5. **Tests en Go:** filtros sueltos y combinados, recuentos de categorías con filtros, traducción y separación de países, orden aleatorio estable (misma semilla, mismo orden, sin repetir entre páginas), ficha con recuentos. `docs/API.md` actualizado.

### Web (`web/`)

1. **Ficha:** filas de enlaces (`web/js/screens.js`, `S.detail`).
2. **Lista:** fila «Filtrar por», selectores con búsqueda, píldoras de filtros activos con ✕, todo leído de la dirección (`S.grid` y `YF.parseRoute`).
3. **Kiosco:** pantalla nueva `S.kiosk` y ruta `#/kiosco`. Tres diapositivas en el DOM (anterior, actual, siguiente), teclado, deslizar y botones; precarga de la imagen de fondo siguiente.
4. **Tema 1:** al volver, cargar tantos títulos como había y situar la vista en la tarjeta de la que se salió.
5. **Pruebas:** tests de node para las funciones puras (dirección ↔ filtros, índice circular del kiosco) y un recorrido en Chromium con Playwright (`tools/web-demo/e2e.mjs`): ficha → director → añadir década → Atrás → posición conservada; kiosco ◀▶, Reproducir y Ficha. Capturas a 1920 px y en móvil.

### TV (`tv/`)

1. **Ficha:** filas de píldoras navegables con las flechas (`S.detail`). Se comprueba que `nav.pick` va bien entre filas de anchos distintos.
2. **Lista:** «Filtrar por» con panel lateral (pantalla nueva `S.picker` encima de la pila), con salto por letra.
3. **Kiosco:** `S.kiosk` y menú nuevo en `tv/index.html`. Teclas ◀▶ propias de la pantalla (no del navegador espacial). Precarga de la siguiente imagen. Fundido con `opacity` y `transition`, que Chromium 85 soporta. Nada de `aspect-ratio` (llega en Chromium 88).
4. **Tema 1:** al volver de la ficha o del reproductor, no recargar la lista entera. Solo refrescar el progreso; si hace falta recargar, volver a enfocar la misma tarjeta por ID.
5. **Pruebas:** primero en el navegador a 1920×1080 con `avplay-sim.js`. Después, **con permiso del usuario y sin nada reproduciéndose**, empaquetar el `.wgt` (ID `Z8YOURFLX1.YourFlix`, perfil `CineT7`, nombre sin espacios), instalar en la TV y probar con el mando. Los resultados se anotan en `docs/PRUEBAS-TV.md`, separando lo probado en el navegador de lo probado en la TV.

## Orden de trabajo (según AGENTS.md: un escritor por zona, una rama por paquete)

Base de todo: la rama `feat/categorias-kiosco` (sale de `fix/orden-anadidas`, que es lo que corre en el Ally).

| Paquete | Rama | Depende de | Puede ir en paralelo con |
|---|---|---|---|
| **P0. Servidor:** categorías, filtros, aleatorio, países, tests, `API.md` | `feat/categorias-servidor` | nada | — |
| **P1. Web:** ficha navegable, Filtrar por, Kiosco, tema 1 | `feat/categorias-web` (sale de P0) | P0 | P2 |
| **P2. TV:** ficha navegable, Filtrar por, Kiosco, tema 1 | `feat/categorias-tv` (sale de P0) | P0 | P1 |
| **P3. Integración:** fusionar P0+P1+P2 en `feat/categorias-kiosco`, tests completos, recorrido en navegador | `feat/categorias-kiosco` | P1, P2 | — |
| **P4. Despliegue** (con el usuario) | — | P3 | — |

P0–P3 se hacen en una sesión de Claude en la nube. P4, con el usuario: el servidor y `web/` en el Ally (`C:\YourFlix\servidor`, reinicio con la TV sin uso) y la app de la TV desde el Mac (`192.168.1.3`, Tizen Studio y perfil `CineT7`), con permiso y sin nada reproduciéndose. La API solo **añade** campos y parámetros, así que la app de la TV que hay instalada sigue funcionando con el servidor nuevo mientras tanto.

## Estimación de tokens

Referencias reales: el arreglo de «Recién añadidas» de hoy (consulta + test + README, un solo archivo de Go) gastó unos 140.000 tokens. El del minibucle de 2 s en la web, unos 266.000. El coste lo dominan leer código, probar en el navegador o la TV y las revisiones; escribir el código es la parte pequeña.

| Paquete | Tokens estimados |
|---|---|
| P0. Servidor | 250.000 – 400.000 |
| P1. Web (incluye kiosco y tema 1) | 350.000 – 600.000 |
| P2. TV (incluye kiosco, tema 1, empaquetado e instalación) | 400.000 – 700.000 |
| P3. Integración y recorrido completo | 80.000 – 150.000 |
| P4. Despliegue y comprobación | 50.000 – 100.000 |
| **Total** | **~1,1 – 1,9 millones** |

Por separado: **solo el Kiosco** (servidor aleatorio + web + TV) son unos 350.000–600.000 tokens. **Solo las fichas navegables** (sin kiosco), unos 0,9–1,5 millones. Hacer las dos a la vez ahorra ~10 %, porque comparten el orden aleatorio, los filtros y las pruebas.

## Decisiones (aprobadas por el usuario el 27-10-2026: las recomendadas)

1. **Índice de actores:** solo los que tienen 2 o más títulos, con un interruptor «Todos». *Recomendado.*
2. **Kiosco:** películas y series, con películas por defecto y sin filtro de visto. *Recomendado.* Alternativa: solo «sin ver».
3. **OK en el kiosco:** muestra Reproducir (seleccionado) y Ver ficha. Así no arranca un vídeo por accidente. *Recomendado.*
4. **Pase automático:** opcional y apagado por defecto. *Recomendado.*
5. **Fotos de actores, sagas e idioma original:** fuera de este plan (necesitan TMDB para el T7 y un ADR).
