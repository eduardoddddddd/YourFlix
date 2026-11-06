// Favoritos y Pendientes de ver en la web, con Playwright, contra un servidor YourFlix de prueba
// (nunca el que está en uso: en el Ally, 8787–8790 son servidores del usuario).
//   PLAYWRIGHT=<ruta a playwright o playwright-core> PW_CHANNEL=msedge \
//   node tools/web-demo/e2e-listas.mjs [http://127.0.0.1:8795] [carpeta-de-capturas]
// Cambia las listas del servidor al que apunta: usar una copia de la base de datos.
// Es una prueba en navegador de escritorio y móvil simulado: no sustituye a probar en la TV.
import { createRequire } from 'node:module';
import fs from 'node:fs';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT || 'playwright');

const BASE = process.argv[2] || 'http://127.0.0.1:8795';
const SHOTS = process.argv[3] || '';
if (SHOTS) fs.mkdirSync(SHOTS, { recursive: true });
const errors = [];
let failed = 0;
const ok = (cond, msg) => { console.log((cond ? '✔ ' : '✘ ') + msg); if (!cond) failed++; };
const api = (p, m) => fetch(BASE + '/api/v1' + p, { method: m || 'GET' }).then((r) => (r.status === 204 ? null : r.json()));
const wait = (ms) => new Promise((r) => setTimeout(r, ms));

// Punto de partida: listas vacías; una película y una serie con algún capítulo numerado.
const lists = await api('/lists');
for (const k of ['favorites', 'watchlist']) {
  for (const t of lists[k].titles) await api(`/lists/${k}/${t.id}`, 'DELETE');
  for (const e of lists[k].episodes) await api(`/lists/${k}/${e.titleId}?season=${e.season}&episode=${e.episode}`, 'DELETE');
}
const movie = (await api('/titles?kind=movie&limit=1')).items[0];
let series = null, ep = null;
for (const s of (await api('/titles?kind=series&limit=50')).items) {
  const d = await api('/titles/' + s.id);
  const f = d.files.find((x) => x.season != null && x.episode != null);
  if (f) { series = s; ep = f; break; }
}
ok(movie && series, `datos: ${movie && movie.title} y ${series && series.title} ${ep && ep.season + '×' + ep.episode}`);

const browser = await chromium.launch(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {});
async function newPage(opts) {
  const page = await browser.newPage(opts);
  page.on('pageerror', (e) => errors.push(String(e)));
  page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
  return page;
}
const shot = (page, name) => SHOTS && page.screenshot({ path: `${SHOTS}/${name}.jpg`, type: 'jpeg', quality: 80 });

// ---- Escritorio ----
let page = await newPage({ viewport: { width: 1920, height: 1080 } });
await page.goto(`${BASE}/#/titulo/${movie.id}`);
await page.waitForSelector('.detail .list-btn');
const favBtn = page.locator('.detail .btns .list-btn').nth(0), wlBtn = page.locator('.detail .btns .list-btn').nth(1);
ok((await favBtn.textContent()).includes('Favorito') && (await favBtn.getAttribute('aria-pressed')) === 'false', 'ficha: «Favorito» sin marcar');
await favBtn.click();
await wlBtn.click();
await wait(400);
let l = await api('/lists');
ok(l.favorites.titles.some((t) => t.id === movie.id), 'clic en ♥: la película está en Favoritos (servidor)');
ok(l.watchlist.titles.some((t) => t.id === movie.id), 'clic en «Pendiente de ver»: está en Pendientes');
ok((await wlBtn.textContent()).includes('En pendientes'), 'el botón cambia a «En pendientes»');
await shot(page, 'web-ficha-botones');

await page.goto(`${BASE}/#/titulo/${series.id}`);
await page.waitForSelector('.ep-lists .list-ibtn');
const hashBefore = await page.evaluate(() => location.hash);
const epItem = page.locator('.episode-item', { has: page.locator(`a[href*="${ep.id}"]`) });
await epItem.locator('.list-ibtn').nth(0).click();
await epItem.locator('.list-ibtn').nth(1).click();
await wait(400);
ok((await page.evaluate(() => location.hash)) === hashBefore, 'el ♥ del capítulo no lo reproduce');
l = await api('/lists');
ok(l.favorites.episodes.some((e) => e.titleId === series.id && e.season === ep.season && e.episode === ep.episode), 'capítulo en Favoritos');
ok(l.watchlist.episodes.some((e) => e.titleId === series.id), 'capítulo en Pendientes');
await shot(page, 'web-serie-capitulos');

await page.click('#nav a[data-nav="mylist"]');
await page.waitForSelector('.list-section');
const secs = await page.$$eval('.list-section', (ss) => ss.map((s) => ({ h: s.querySelector('h2').textContent, cards: s.querySelectorAll('.card').length })));
ok(secs.length === 2 && secs[0].h.startsWith('Pendientes de ver') && secs[1].h.startsWith('Favoritos'), 'Mi lista: Pendientes de ver y Favoritos');
ok(secs[0].cards === 2 && secs[1].cards === 2, `Mi lista: 2 + 2 tarjetas (${secs.map((s) => s.cards).join(' + ')})`);
await shot(page, 'web-mi-lista');
await page.click('.list-section .grid.eps .card');
await page.waitForFunction(() => location.hash.indexOf('#/ver/') === 0);
ok(true, 'la tarjeta del capítulo abre el reproductor');
await page.goto(`${BASE}/#/`);
await page.waitForSelector('.rows .row');
const rows = await page.$$eval('.rows .row h2', (hs) => hs.map((h) => h.textContent));
ok(rows.indexOf('Pendientes de ver') >= 0 && rows.indexOf('Favoritos') >= 0, 'Inicio: filas ' + rows.slice(0, 4).join(' · '));
await page.goto(`${BASE}/#/peliculas`);
await page.waitForSelector(`.grid .card[data-id="${movie.id}"]`);
ok(await page.locator(`.grid .card[data-id="${movie.id}"] .list-badges .lb`).count() === 2, 'Películas: la carátula lleva ♥ y la marca');

// Quitar desde la ficha.
await page.goto(`${BASE}/#/titulo/${movie.id}`);
await page.waitForSelector('.detail .list-btn');
await page.locator('.detail .btns .list-btn').nth(0).click();
await wait(400);
l = await api('/lists');
ok(!l.favorites.titles.some((t) => t.id === movie.id) && l.watchlist.titles.some((t) => t.id === movie.id), 'quitar ♥ solo la saca de Favoritos');

// Kiosco: ♥ y + junto a Reproducir · Ver ficha.
await page.goto(`${BASE}/#/kiosco`);
await page.waitForSelector('.k-slide[aria-hidden="false"] .list-ibtn');
const kb = page.locator('.k-slide[aria-hidden="false"] .list-ibtn');
ok(await kb.count() === 2, 'Kiosco: dos botones de lista en la diapositiva visible');
await kb.nth(1).click();
await wait(400);
ok((await api('/lists')).watchlist.titles.length === 2, 'Kiosco: + añade el título a Pendientes');
await shot(page, 'web-kiosco-botones');
await page.close();

// ---- Móvil ----
page = await newPage({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, deviceScaleFactor: 2 });
await page.goto(`${BASE}/#/mi-lista`);
await page.waitForSelector('.list-section');
ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'móvil: Mi lista sin desbordar a lo ancho');
const navFits = await page.evaluate(() => { const n = document.getElementById('nav'); return [...n.children].every((a) => a.getBoundingClientRect().right <= innerWidth + 0.5); });
ok(navFits, 'móvil: los 6 botones de la barra inferior caben');
await shot(page, 'web-movil-mi-lista');
await page.goto(`${BASE}/#/titulo/${series.id}`);
await page.waitForSelector('.ep-lists .list-ibtn');
ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'móvil: ficha de la serie sin desbordar');
await shot(page, 'web-movil-serie');
await page.close();

await browser.close();
ok(errors.length === 0, 'errores de JavaScript: ' + (errors.length ? errors.join(' | ') : 0));
process.exit(failed ? 1 : 0);
