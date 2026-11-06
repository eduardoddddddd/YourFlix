// Favoritos y Pendientes de ver en la app de la TV, en Chromium/Edge (Playwright) a 1920×1080
// con el AVPlay simulado, solo con teclas: flechas, Intro = OK, Escape = Atrás.
//   node tools/tv-demo/e2e-listas.mjs [http://127.0.0.1:8796] [http://127.0.0.1:8795] [capturas]
// APP sirve tv/ como estáticos (el script lo arranca él mismo si APP es 127.0.0.1:8796);
// SERVER es un servidor YourFlix de prueba con una copia de los datos (cambia sus listas).
// Variables: PLAYWRIGHT (ruta a playwright o playwright-core), PW_CHANNEL (msedge, chrome).
// Es una prueba en navegador: no sustituye a probar con el mando en la TV (Chromium 85).
import { createRequire } from 'node:module';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT || 'playwright');

const APP = process.argv[2] || 'http://127.0.0.1:8796';
const SERVER = process.argv[3] || 'http://127.0.0.1:8795';
const SHOTS = process.argv[4] || '';
if (SHOTS) fs.mkdirSync(SHOTS, { recursive: true });
const errors = [];
let failed = 0;
const ok = (cond, msg) => { console.log((cond ? '✔ ' : '✘ ') + msg); if (!cond) failed++; };
const api = (p, m) => fetch(SERVER + '/api/v1' + p, { method: m || 'GET' }).then((r) => (r.status === 204 ? null : r.json()));

// Estáticos de tv/ (sin Python).
let staticSrv = null;
if (APP === 'http://127.0.0.1:8796') {
  const root = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', '..', 'tv');
  const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.css': 'text/css', '.png': 'image/png' };
  staticSrv = http.createServer((req, res) => {
    const p = path.join(root, decodeURIComponent(new URL(req.url, APP).pathname).replace(/^\/+/, '') || 'index.html');
    if (!p.startsWith(root) || !fs.existsSync(p) || fs.statSync(p).isDirectory()) { res.writeHead(404); res.end(); return; }
    res.writeHead(200, { 'Content-Type': types[path.extname(p)] || 'application/octet-stream' });
    fs.createReadStream(p).pipe(res);
  }).listen(8796, '127.0.0.1');
}

// Punto de partida: listas vacías, una película y una serie con capítulos numerados.
const lists = await api('/lists');
for (const k of ['favorites', 'watchlist']) {
  for (const t of lists[k].titles) await api(`/lists/${k}/${t.id}`, 'DELETE');
  for (const e of lists[k].episodes) await api(`/lists/${k}/${e.titleId}?season=${e.season}&episode=${e.episode}`, 'DELETE');
}
const movie = (await api('/titles?kind=movie&limit=1')).items[0];
let series = null;
for (const s of (await api('/titles?kind=series&limit=50')).items) {
  const d = await api('/titles/' + s.id);
  if (d.files.filter((x) => x.season != null && x.episode != null).length >= 2) { series = s; break; }
}
await api(`/lists/watchlist/${movie.id}`, 'PUT');
ok(movie && series, `datos: ${movie.title} (en Pendientes) y ${series && series.title}`);

const browser = await chromium.launch(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {});
const page = await browser.newPage({ viewport: { width: 1920, height: 1080 } });
page.on('pageerror', (e) => errors.push(String(e)));
page.on('console', (m) => { if (m.type() === 'error' && !m.text().startsWith('Failed to load resource')) errors.push(m.text()); });
page.on('response', (r) => {
  if (r.status() >= 400 && !/\/\$WEBAPIS\/webapis\/webapis\.js$/.test(r.url()) && r.url() !== APP + '/api/v1/ping') errors.push(r.status() + ' ' + r.url());
});
const shot = (name) => SHOTS && page.screenshot({ path: `${SHOTS}/${name}.jpg`, type: 'jpeg', quality: 80 });
const wait = (ms) => page.waitForTimeout(ms);
async function press(key, n = 1, ms = 200) { for (let i = 0; i < n; i++) { await page.keyboard.press(key); await wait(ms); } }
const focused = () => page.evaluate(() => {
  const e = window.YF.nav.current;
  return e ? { text: e.textContent, cls: e.className, id: e.dataset.id || '', k: e.dataset.k || '', go: e.dataset.go || '' } : null;
});
const screenName = () => page.evaluate(() => window.YF.app.current().name);
const idle = () => page.waitForFunction(() => !document.querySelector('#screens .screen:not(.hidden) > .loading'), null, { timeout: 8000 }).then(() => wait(500));
async function until(key, cond, n = 15) {
  for (let i = 0; i < n; i++) { const f = await focused(); if (f && cond(f)) return f; await press(key); }
  return focused();
}
const MENU = ['home', 'movies', 'series', 'mylist', 'kiosk', 'search', 'settings'];
async function menuTo(go) {
  await until('Escape', (f) => !!f.go, 5);
  const f = await focused();
  const d = MENU.indexOf(go) - MENU.indexOf(f.go);
  await press(d > 0 ? 'ArrowRight' : 'ArrowLeft', Math.abs(d));
  await press('Enter');
  await idle();
}

await page.goto(`${APP}/?server=${encodeURIComponent(SERVER)}`);
await page.waitForSelector('.home-rows .card', { timeout: 15000 });
await idle();
ok((await page.locator('#menu .menu-item').allTextContents()).join('|') === 'Inicio|Películas|Series|♥ Mi lista|Kiosco|⌕ Buscar|Ajustes', 'menú con «♥ Mi lista» tras Series');
const homeRows = await page.$$eval('.home-rows .row h2', (hs) => hs.map((h) => h.textContent));
ok(homeRows.indexOf('Pendientes de ver') >= 0 && homeRows.indexOf('Favoritos') < 0, 'Inicio: fila Pendientes de ver (Favoritos vacía, no sale): ' + homeRows.slice(0, 3).join(' · '));

// ---- Ficha de la película: ♥ Favorito con el mando ----
await menuTo('mylist');
ok((await screenName()) === 'mylist', 'Mi lista abierta');
let f = await focused();
ok(f && f.id === movie.id, 'foco en la tarjeta de ' + movie.title);
await shot('tv-mi-lista-antes');
await press('Enter');
await idle();
f = await until('ArrowRight', (x) => x.text.indexOf('Favorito') >= 0, 5);
ok(f.text === '♥ Favorito', 'ficha: botón «♥ Favorito»');
await press('Enter');
await wait(500);
f = await focused();
ok(f.text === '♥ En favoritos' && f.cls.indexOf('on') >= 0, 'OK: «♥ En favoritos», el foco no se mueve');
await press('ArrowRight');
f = await focused();
ok(f.text === '✓ En pendientes', 'a la derecha, «✓ En pendientes» (ya estaba)');
await shot('tv-ficha-botones');
let l = await api('/lists');
ok(l.favorites.titles.some((t) => t.id === movie.id), 'servidor: la película está en Favoritos');
await press('Escape');
await idle();
f = await focused();
ok((await screenName()) === 'mylist' && f && f.id === movie.id, 'Atrás: Mi lista recargada, foco en la misma tarjeta');
ok((await page.$$eval('.mylist .list-h', (hs) => hs.map((h) => h.textContent))).join(' | ') === 'Pendientes de ver  ·  1 | Favoritos  ·  1', 'Mi lista: 1 pendiente y 1 favorito');

// ---- Serie: ♥ y + de un capítulo ----
await page.evaluate((id) => window.YF.app.open(window.YF.screens.detail(id)), series.id);
await idle();
f = await until('ArrowDown', (x) => x.cls.indexOf('episode') >= 0, 8);
ok(f.cls.indexOf('episode') >= 0, 'capítulos: foco en ' + f.text.slice(0, 30));
await press('ArrowRight');
const ep = await focused();
await page.evaluate(() => { window.__ep = window.YF.nav.current; });
await press('ArrowUp');
f = await focused();
ok(f.cls.indexOf('mbtn') >= 0 && f.text === '♥', '▲ desde el capítulo: su ♥');
const mine = await page.evaluate(() => { const c = window.YF.nav.current; return c.closest('.episode') === c._nav.down; });
ok(mine, 'el ♥ es el de ese capítulo, no el del vecino');
await press('ArrowRight');
f = await focused();
ok(f.text === '+', '▶: el + del mismo capítulo');
await press('Enter');
await wait(500);
ok((await focused()).text === '✓', 'OK: + pasa a ✓');
await shot('tv-capitulo-botones');
await press('ArrowDown');
f = await focused();
ok(await page.evaluate(() => window.YF.nav.current === window.__ep), '▼: vuelve al mismo capítulo');
l = await api('/lists');
ok(l.watchlist.episodes.length === 1 && l.watchlist.episodes[0].titleId === series.id, 'servidor: el capítulo está en Pendientes');
await press('Escape');
await idle();
ok((await page.$$eval('.mylist .grid .card.wide', (c) => c.length)) === 1, 'Mi lista: aparece la tarjeta del capítulo');

// ---- Kiosco: ♥ desde los botones ----
await menuTo('kiosk');
await wait(1500);
await press('Enter');
await press('ArrowRight', 2);
const kb = await page.$$eval('.kbtns .btn', (b) => b.map((x) => x.textContent + (x.classList.contains('focused') ? '*' : '')));
ok(kb.length === 4 && kb[2].endsWith('*'), 'Kiosco: 4 botones, foco en el 3.º: ' + kb.join(' · '));
const kt = await page.evaluate(() => window.YF.app.current().cur.t.id);
const wasFav = (await api('/lists')).favorites.titles.some((t) => t.id === kt);
await press('Enter');
await wait(500);
ok((await api('/lists')).favorites.titles.some((t) => t.id === kt) === !wasFav, 'Kiosco: OK cambia Favoritos en el servidor');
await shot('tv-kiosco-botones');

const sim = await page.evaluate(() => (window.webapis && webapis.avplay && webapis.avplay._hangs) || 0);
ok(sim === 0, 'AVPlay simulado sin cuelgues');
ok(errors.length === 0, 'errores: ' + (errors.length ? errors.join(' | ') : 0));
await browser.close();
if (staticSrv) staticSrv.close();
process.exit(failed ? 1 : 0);
