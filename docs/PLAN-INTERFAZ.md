# Plan de mejoras de la interfaz: búsqueda y exploración

**26-10-2026 · solo plan, nada implementado.** Afecta sobre todo a la web (`web/`) y al servidor (`server/`). La app de la TV (`tv/`) va en una fase aparte.

## Punto de partida

- **Datos que ya tiene cada ficha** (`titles.meta`): director (lista), reparto (10), país (texto en inglés con comas, p. ej. «United States of America, United Kingdom»), géneros, año, duración (`runtime`), nota, sinopsis, título original, IDs de TMDB e IMDb.
- **Datos de cada archivo** (`files`): resolución, HDR (`transfer`), idiomas y códecs de audio, subtítulos.
- **Búsqueda actual:** título, título original, director y reparto (`LIKE` en `store/titles.go`). Filtros: solo género. Orden: nota o añadidas.

## 1. Fichas que llevan a otras búsquedas

- En la ficha, director, cada actor, país, géneros y año son enlaces a la lista de la colección que comparte ese dato («Ridley Scott · 6 películas»).
- **Página de persona:** foto (si se enriquece con TMDB, ver §7), sus títulos en la colección por año y cuáles se han visto.
- En la ficha, filas «Del mismo director» y «Más con este actor» (el primero del reparto).

## 2. Explorar con filtros combinables

- Pantalla **«Explorar»** con filtros que se suman:
  - Género, década o rango de años, país.
  - Duración (menos de 90 min, 90–120, más de 2 h) y nota mínima.
  - Calidad (4K, HDR, 1080p), audio (castellano, VO) y subtítulos (castellano, inglés).
  - Estado: visto, sin ver, a medias.
- **Orden:** nota, año, título, duración, añadidas y aleatorio.
- **«Sorpréndeme»:** un título al azar, sin ver, entre los que cumplen los filtros.
- **Los filtros van en la dirección de la página**, así se pueden guardar o compartir (p. ej. «cine japonés de los 80 en VOSE»).

## 3. Búsqueda más lista

- **Una caja con resultados agrupados:** Títulos, Personas, Países y Géneros («kubri» → Stanley Kubrick → su filmografía).
- **Tolerante a acentos y erratas:** «amelie», «el padrino 2», «terminator 2» o «t2».
- **Búsqueda también en la sinopsis**, con menos peso que el título.
- Búsquedas recientes y sugerencias al escribir.

## 4. Filas automáticas en Inicio

- Directores de la colección (carrusel con número de títulos).
- Por décadas.
- «Cine de…»: los 5 o 6 países con más títulos.
- «Joyas sin ver»: nota alta y nunca empezadas.
- «Para esta noche»: menos de 100 min y bien valoradas.
- «En 4K / HDR».
- Sagas (necesita TMDB, §7).

## 5. Comodidades

- **Marcar visto / no visto** desde la ficha y desde las listas.
- **Lista «Para ver»**, compartida entre TV y web (tabla nueva en el servidor). → **Hecho en 0.5.0** como «Pendientes de ver» y «Favoritos» (ADR-008).
- **Insignias en las carátulas:** 4K, HDR, VOSE, «a medias» con barra de progreso, «nuevo».
- **Vista de lista compacta en la web:** tabla con título, año, director, duración y nota, ordenable por columnas.
- **Series:** «siguiente capítulo» en la ficha y avance de temporada (7/10).

## 6. TV (con el mando, Chromium 85)

- La ficha enlaza a director y reparto, que se recorren con las flechas.
- «Explorar» con una columna lateral de filtros, uno por fila: ◀▶ cambia el valor y la rejilla se actualiza al momento.
- Búsqueda: sugerencias de personas en el teclado en pantalla que ya existe.
- Firmar e instalar siguiendo AGENTS.md: con permiso del usuario y nunca mientras está viendo algo.

## 7. Datos que faltan y decisiones pendientes

- **País en castellano y normalizado:** se traduce y se separa en el servidor (tabla o índice propio). No hace falta TMDB.
- **Sagas, idioma original, guionista, compositor, palabras clave y fotos de personas:** no están en la semilla del T7. Para tenerlos hay que llamar a TMDB también para el T7, y eso contradice la regla actual de AGENTS.md (la colección T7 sin TMDB). **Decisión del usuario**: serían unas 250 llamadas una sola vez, que quedan en caché. Si se aprueba, hay que registrar un ADR en `docs/DECISIONES.md`.
- **Rendimiento:** con ~250 títulos todo cabe en memoria; no hacen falta índices especiales para filtrar.

## Fases y estimación de tokens

Estimaciones a ojo. Como referencia, el diagnóstico y arreglo del minibucle de 2 s (`d620c88`, ~65 líneas con test) gastó unos 266.000 tokens. El coste lo domina releer código y verificar en el navegador o la TV; escribir el código serían unos 30.000–60.000 tokens por fase. Cada fase incluye un 20–30 % por la revisión de otro modelo que pide AGENTS.md.

| Fase | Qué incluye | Tokens estimados |
|---|---|---|
| **1. Fichas que enlazan, persona y Explorar (web)** | Filtros nuevos en `/titles`, endpoint de persona, país normalizado y en castellano, pantallas web, tests | 400.000–800.000 |
| **2. Búsqueda agrupada y tolerante; visto y «Para ver»** | Búsqueda sin acentos ni erratas, resultados agrupados, tabla nueva, botones en la web | 300.000–600.000 |
| **3. Filas automáticas e insignias** | Consultas para Inicio e insignias en las carátulas | 150.000–300.000 |
| **4. Llevarlo a la TV** | Interfaz con el mando en Chromium 85, firmar e instalar (con permiso) | 400.000–800.000 |
| **5. TMDB para el T7 (opcional)** | Sagas, fotos de personas, idioma original (necesita la decisión de §7) | 200.000–400.000 |
| **Total** | | **~1,5–3 millones** (solo web, fases 1–3: ~0,9–1,7 millones) |

**Recomendación:** empezar por la fase 1 sola, medir lo que gasta de verdad y decidir el resto con ese dato.
