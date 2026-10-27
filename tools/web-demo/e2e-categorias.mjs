// Recorrido de fichas navegables, «Filtrar por» y Kiosco con Chromium (Playwright) contra un servidor
// YourFlix con la biblioteca de make_categorias.sh (46 películas y 2 series de la semilla).
//   node tools/web-demo/e2e-categorias.mjs [http://127.0.0.1:8791] [carpeta-de-capturas]
// Es una prueba en navegador de escritorio y móvil simulado: no sustituye a probar en la TV.
import { createRequire } from 'node:module';
import fs from 'node:fs';
const require = createRequire(import.meta.url);
const { chromium } = require('playwright');

const BASE = process.argv[2] || 'http://127.0.0.1:8791';
const SHOTS = process.argv[3] || '';
if (SHOTS) fs.mkdirSync(SHOTS, { recursive: true });
const errors = [];
let failed = 0;
const ok = (cond, msg) => { console.log((cond ? '✔ ' : '✘ ') + msg); if (!cond) failed++; };
const api = (p) => fetch(BASE + '/api/v1' + p).then((r) => r.json());
const wait = (ms) => new Promise((r) => setTimeout(r, ms));

const browser = await chromium.launch();
async function newPage(opts) {
  const page = await browser.newPage(opts);
  page.on('pageerror', (e) => errors.push(String(e)));
  page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
  return page;
}
// En JPEG de calidad 80 (en PNG pesan de 0,5 a 1 MB cada una).
const shot = (page, name) => SHOTS && page.screenshot({ path: `${SHOTS}/web-${name}.jpg`, type: 'jpeg', quality: 80 });
const hash = (page) => page.evaluate(() => decodeURIComponent(location.hash));
const curName = (page) => page.locator('.k-slide.cur .k-name').textContent();

const page = await newPage({ viewport: { width: 1920, height: 1080 } });

// ---- Ficha navegable ----
const ridley = (await api('/titles?kind=movie&director=Ridley%20Scott&sort=year')).items;
ok(ridley.length === 5, `API: Ridley Scott tiene ${ridley.length} películas`);
await page.goto(`${BASE}/#/titulo/${ridley[0].id}`);
await page.waitForSelector('.detail .links');
const dirLink = page.locator('.link-row:has(.lbl:text-is("Dirección")) .tag').first();
ok((await dirLink.textContent()) === 'Ridley Scott (5)', 'ficha: «Dirección: Ridley Scott (5)»');
const counts = await page.locator('.links .tag .n').allTextContents();
ok(counts.every((c) => +c.replace(/\D/g, '') >= 2), `ficha: el número solo aparece si es 2 o más (${counts.join(' ')})`);
const rows = await page.locator('.link-row .lbl').allTextContents();
ok(['Dirección', 'Reparto', 'País', 'Año', 'Géneros', 'Duración'].every((r) => rows.includes(r)), 'ficha: filas ' + rows.join(' · '));
ok((await page.locator('.link-row:has(.lbl:text-is("Reparto")) .tag').count()) >= 5, 'ficha: reparto enlazado');
await shot(page, 'ficha-navegable');

// ---- Ficha → director → sumar década → Atrás ----
await dirLink.click();
await page.waitForFunction(() => location.hash.includes('director=Ridley'));
await page.waitForSelector('.grid .card');
ok((await page.locator('.page-head h1').evaluate((h) => h.firstChild.textContent + ' · ' + h.querySelector('.count').textContent)) === 'Películas · Ridley Scott · 5 títulos', 'lista: «Películas · Ridley Scott · 5 títulos»');
ok((await page.locator('.grid .card').count()) === 5, 'lista: 5 tarjetas');
await page.click('.filter-by .chip:has-text("Año/Década")');
await page.waitForSelector('.fdrop .fopt');
const decades = await page.locator('.fdrop .fopt .nm').allTextContents();
ok(decades.includes('Años 80'), 'desplegable Año/Década con las décadas de Ridley Scott: ' + decades.slice(0, 4).join(', '));
await page.click('.fdrop .fopt:has-text("Años 80")');
await page.waitForFunction(() => location.hash.includes('decada=1980'));
await page.waitForSelector('.grid .card');
const n80 = (await api('/titles?kind=movie&director=Ridley%20Scott&decade=1980')).total;
await page.waitForFunction((n) => document.querySelectorAll('.grid .card').length === n, n80);
ok((await page.locator('.active-filters .pill:not(.clear)').count()) === 2, `filtros sumados: Ridley Scott + Años 80 (${n80} títulos)`);
ok((await hash(page)).includes('director=Ridley Scott') && (await hash(page)).includes('decada=1980'), 'filtros en la dirección: ' + await hash(page));
await shot(page, 'lista-filtrada');
await page.goBack();
await page.waitForFunction(() => !location.hash.includes('decada'));
await page.waitForFunction(() => document.querySelectorAll('.grid .card').length === 5);
ok(true, 'Atrás quita la década y vuelve a las 5 de Ridley Scott');
await page.click('.active-filters .pill:has-text("Ridley Scott")');
await page.waitForFunction(() => !location.hash.includes('director'));
await page.waitForSelector('.grid .card');
ok((await page.locator('.grid .card').count()) >= 40, 'quitar la píldora ✕ deja todas las películas');

// ---- Desplegable con búsqueda ----
await page.click('.filter-by .chip:has-text("Director")');
await page.fill('.fdrop-search', 'scor');
const opts = await page.locator('.fdrop .fopt').allTextContents();
ok(opts.length === 1 && /Martin Scorsese/.test(opts[0]) && /4$/.test(opts[0]), '«scor» → ' + opts.join(' | '));
await shot(page, 'desplegable-busqueda');
await page.keyboard.press('Escape');
await page.click('.filter-by .chip:has-text("Actor")');
const few = await page.locator('.fdrop .fopt').count();
await page.click('.fdrop .chip:has-text("Todos")');
await page.waitForFunction((n) => document.querySelectorAll('.fdrop .fopt').length > n, few);
ok((await page.locator('.fdrop .fopt').count()) > few, `actores: ${few} con 2 o más; «Todos» muestra ${await page.locator('.fdrop .fopt').count()}`);
await page.keyboard.press('Escape');
await page.click('.chips.sorts .chip:has-text("Duración")');
await page.waitForFunction(() => location.hash.includes('orden=runtime'));
ok(true, 'orden nuevo: Duración');

// ---- Tema 1: volver de una ficha a la tarjeta de la que se salió ----
await page.goto(`${BASE}/#/peliculas`);
await page.waitForSelector('.grid .card');
const cards = page.locator('.grid .card');
const low = cards.nth((await cards.count()) - 3);
await low.scrollIntoViewIfNeeded();
const lowId = await low.getAttribute('data-id');
const y0 = await page.evaluate(() => scrollY);
await low.click();
await page.waitForSelector('.detail h1');
await page.goBack();
await page.waitForSelector('.grid .card');
await page.waitForFunction((id) => document.activeElement && document.activeElement.dataset.id === id, lowId, { timeout: 5000 }).catch(() => {});
const back = await page.evaluate((id) => {
  const c = document.querySelector(`.grid .card[data-id="${id}"]`), r = c.getBoundingClientRect();
  return { focused: document.activeElement === c, visible: r.top >= 0 && r.bottom <= innerHeight, y: scrollY };
}, lowId);
ok(back.focused && back.visible && back.y > 500, `al volver, la vista sigue en la tarjeta (scrollY ${y0} → ${Math.round(back.y)}, enfocada)`);

// ---- Ver en kiosco desde una lista filtrada ----
await page.goto(`${BASE}/#/peliculas?director=Ridley%20Scott`);
await page.waitForSelector('.grid .card');
await page.click('.active-filters .kiosk-btn');
await page.waitForSelector('.k-slide.cur .k-name');
ok((await page.locator('.k-title').textContent()) === 'Kiosco de Ridley Scott' && (await page.locator('.k-count').textContent()) === '1 / 5', '«Ver en kiosco» → Kiosco de Ridley Scott, 1 / 5');

// ---- Kiosco ----
await page.click('#nav a[data-nav=kiosk]');
await page.waitForFunction(() => /semilla=\d+/.test(location.hash) && !location.hash.includes('director'));
await page.waitForSelector('.k-slide.cur .k-name');
const total = (await api('/titles?kind=movie&limit=1')).total;
ok((await page.locator('.k-count').textContent()) === `1 / ${total}`, `kiosco: ${total} películas en orden aleatorio`);
ok((await page.locator('#nav a').allTextContents()).join(' · ') === 'Inicio · Películas · Series · Kiosco · Buscar', 'menú: Inicio · Películas · Series · Kiosco · Buscar');
ok((await page.locator('.k-slide').count()) === 3, 'tres diapositivas en el DOM');
ok(await page.evaluate(() => document.activeElement && document.activeElement.classList.contains('k-play')), 'Reproducir seleccionado');
const seed = +(await hash(page)).match(/semilla=(\d+)/)[1];
const order = (await api(`/titles?kind=movie&sort=random&seed=${seed}&limit=500`)).items.map((t) => t.title);
const a0 = await curName(page);
ok(a0 === order[0], `primer título = el primero de sort=random&seed=${seed}`);
// Precarga: la imagen de fondo de la siguiente ya está cargada.
await page.waitForFunction(() => [...document.querySelectorAll('.k-slide:not(.cur) .k-bg')].every((i) => !i.src || i.complete), null, { timeout: 15000 }).catch(() => {});
ok(await page.evaluate(() => [...document.querySelectorAll('.k-slide:not(.cur) .k-bg')].every((i) => !i.src || (i.complete && i.naturalWidth > 0))), 'fondos de anterior y siguiente precargados');
ok(await page.evaluate(() => /w=1920/.test(document.querySelector('.k-slide.cur .k-bg').src)), 'fondo a 1920 px');
await page.keyboard.press('ArrowRight');
ok((await curName(page)) === order[1] && (await hash(page)).includes('i=1'), '▶ (teclado) pasa al siguiente: ' + order[1]);
await page.keyboard.press('ArrowLeft');
await page.keyboard.press('ArrowLeft');
ok((await curName(page)) === order[order.length - 1], '◀ desde el primero da la vuelta al último');
await page.click('.k-arrow.next');
await page.click('.k-arrow.next');
await page.click('.k-arrow.next');
ok((await curName(page)) === order[2], 'botones ◀▶: ' + order[2]);
await wait(1500);
ok((await page.locator('.k-slide.cur .synopsis').count()) === 1 && (await page.locator('.k-slide.cur .people').count()) === 1, 'ficha del kiosco con sinopsis, director y reparto');
await shot(page, 'kiosco-escritorio');
// ▲▼ entre Reproducir y Ver ficha
await page.focus('.k-slide.cur .k-play');
await page.keyboard.press('ArrowDown');
ok(await page.evaluate(() => document.activeElement.classList.contains('k-ficha')), '▼ selecciona Ver ficha');
// Ver ficha y volver al mismo título
await page.keyboard.press('Enter');
await page.waitForSelector('.detail h1');
ok((await page.locator('.detail h1').textContent()) === order[2], 'Ver ficha abre ' + order[2]);
await page.goBack();
await page.waitForSelector('.k-slide.cur .k-name');
ok((await curName(page)) === order[2] && (await hash(page)).includes(`semilla=${seed}`) && (await hash(page)).includes('i=2'), 'Atrás vuelve al mismo título del kiosco (' + await hash(page) + ')');
// Reproducir y volver
await page.waitForFunction(() => /^#\/ver\//.test(new URL(document.querySelector('.k-slide.cur .k-play').href).hash));
await page.focus('.k-slide.cur .k-play');
await page.keyboard.press('Enter');
await page.waitForSelector('#player:not(.hidden)');
ok((await hash(page)).startsWith('#/ver/'), 'Reproducir abre el reproductor');
await wait(1500);
await page.keyboard.press('Escape');
await page.waitForSelector('#player.hidden', { state: 'attached' });
await page.waitForSelector('.k-slide.cur .k-name');
ok((await curName(page)) === order[2], 'al salir del reproductor, el mismo título del kiosco');
// Pase automático: se enciende y se para con cualquier tecla
await page.click('.k-pills .auto');
ok((await page.getAttribute('.k-pills .auto', 'aria-pressed')) === 'true', 'pase automático encendido');
await page.keyboard.press('Shift');
ok((await page.getAttribute('.k-pills .auto', 'aria-pressed')) === 'false', 'una tecla lo para');
// Filtros del kiosco
await page.click('.k-pills .chip:has-text("Series")');
await page.waitForFunction(() => location.hash.includes('tipo=series'));
await page.waitForSelector('.k-slide.cur .k-name');
ok((await page.locator('.k-count').textContent()) === '1 / 2', 'kiosco de series: 2');
await page.click('.k-pills .chip:has-text("Películas")');
await page.waitForFunction(() => !location.hash.includes('tipo='));
await page.selectOption('.k-pills select[aria-label="Todas las décadas"]', '1980');
await page.waitForFunction(() => location.hash.includes('decada=1980'));
await page.waitForSelector('.k-slide.cur .k-name');
const k80 = (await api('/titles?kind=movie&decade=1980')).total;
ok((await page.locator('.k-count').textContent()) === `1 / ${k80}`, `kiosco con década 80: ${k80}`);
ok((await hash(page)).includes(`semilla=${seed}`), 'la semilla se conserva en la sesión');

// ---- Móvil ----
const m = await newPage({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, deviceScaleFactor: 2 });
await m.goto(`${BASE}/#/kiosco`);
await m.waitForSelector('.k-slide.cur .k-name');
const m0 = await curName(m);
const swipe = (dx) => m.evaluate((dx) => {
  const t = document.querySelector('.k-slides');
  const mk = (x) => new Touch({ identifier: 1, target: t, clientX: x, clientY: 400 });
  t.dispatchEvent(new TouchEvent('touchstart', { touches: [mk(200)], changedTouches: [mk(200)], bubbles: true }));
  t.dispatchEvent(new TouchEvent('touchend', { touches: [], changedTouches: [mk(200 + dx)], bubbles: true }));
}, dx);
await swipe(-150);
const m1 = await curName(m);
await swipe(150);
ok(m1 !== m0 && (await curName(m)) === m0, `móvil: deslizar ◀▶ (${m0} → ${m1} → ${m0})`);
ok(await m.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'móvil: kiosco sin desplazamiento horizontal');
await wait(1500);
const btnBox = await m.locator('.k-slide.cur .k-play').boundingBox();
const navTop = await m.locator('#nav').evaluate((n) => n.getBoundingClientRect().top);
ok(btnBox && btnBox.y + btnBox.height <= navTop, 'móvil: los botones quedan por encima de la barra inferior');
await shot(m, 'kiosco-movil');
await m.goto(`${BASE}/#/peliculas?director=Oliver%20Stone`);
await m.waitForSelector('.grid .card');
ok(await m.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'móvil: lista filtrada sin desplazamiento horizontal');
await shot(m, 'lista-movil');
await m.click('.filter-by .chip:has-text("Actor")');
await m.waitForSelector('.fdrop .fopt');
ok(await m.locator('.fdrop').evaluate((d) => getComputedStyle(d).position === 'fixed'), 'móvil: el desplegable sale como hoja inferior');
await shot(m, 'desplegable-movil');

ok(errors.length === 0, 'sin errores de JavaScript' + (errors.length ? ': ' + errors.join(' | ') : ''));
await browser.close();
process.exit(failed ? 1 : 0);
