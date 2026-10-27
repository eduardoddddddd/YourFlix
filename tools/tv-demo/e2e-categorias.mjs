// Recorrido de la app de la TV en Chromium (Playwright) a 1920×1080 con el AVPlay simulado
// (tv/js/avplay-sim.js), solo con el teclado: flechas, Intro = OK, Escape = Atrás.
// Fichas navegables, «Filtrar por», posición conservada al volver (tema 1) y Kiosco.
//   python3 -m http.server 8794 --directory tv &
//   node tools/tv-demo/e2e-categorias.mjs [http://localhost:8794] [http://127.0.0.1:8792] [carpeta-de-capturas]
// Es una prueba en navegador: no sustituye a probar con el mando en la TV (Chromium 85).
import { createRequire } from 'node:module';
import fs from 'node:fs';
const require = createRequire(import.meta.url);
const { chromium } = require('playwright');

const APP = process.argv[2] || 'http://localhost:8794';
const SERVER = process.argv[3] || 'http://127.0.0.1:8792';
const SHOTS = process.argv[4] || 'docs/capturas/categorias-kiosco';
fs.mkdirSync(SHOTS, { recursive: true });
const errors = [];
let failed = 0;
const ok = (cond, msg) => { console.log((cond ? '✔ ' : '✘ ') + msg); if (!cond) failed++; };

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1920, height: 1080 } });
page.on('pageerror', (e) => errors.push(String(e)));
// «Failed to load resource» se comprueba aparte: en el navegador faltan, como es normal,
// $WEBAPIS/webapis/webapis.js (solo existe en la TV) y el /ping al propio origen del descubrimiento.
page.on('console', (m) => { if (m.type() === 'error' && !m.text().startsWith('Failed to load resource')) errors.push(m.text()); });
page.on('response', (r) => {
  if (r.status() >= 400 && !/\/\$WEBAPIS\/webapis\/webapis\.js$/.test(r.url()) && r.url() !== APP + '/api/v1/ping') errors.push(r.status() + ' ' + r.url());
});
const shot = (name) => page.screenshot({ path: `${SHOTS}/${name}.jpg`, type: 'jpeg', quality: 80 });
const wait = (ms) => page.waitForTimeout(ms);
async function press(key, n = 1, ms = 180) { for (let i = 0; i < n; i++) { await page.keyboard.press(key); await wait(ms); } }
const focused = () => page.evaluate(() => {
  const e = window.YF.nav.current;
  if (!e) return null;
  const sc = e.closest('.scroller');
  return { text: e.textContent, id: e.dataset.id || '', k: e.dataset.k || '', go: e.dataset.go || '', scroll: sc ? sc.scrollTop : -1 };
});
const screenName = () => page.evaluate(() => window.YF.app.current().name);
const idle = () => page.waitForFunction(() => !document.querySelector('#screens .screen:not(.hidden) > .loading'), null, { timeout: 8000 }).then(() => wait(400));

// Mueve el foco con una flecha hasta que se cumpla cond (como mucho n pulsaciones).
async function until(key, cond, n = 15) {
  for (let i = 0; i < n; i++) { const f = await focused(); if (f && cond(f)) return f; await press(key); }
  return focused();
}
// En la misma fila de píldoras: ◀ o ▶ hasta la de data-k = k.
async function rowTo(k) {
  const d = await page.evaluate((k) => {
    const cur = window.YF.nav.current, row = [...cur.parentNode.querySelectorAll('.f')];
    return row.findIndex((e) => e.dataset.k === k) - row.indexOf(cur);
  }, k);
  await press(d > 0 ? 'ArrowRight' : 'ArrowLeft', Math.abs(d));
  return focused();
}
async function menuTo(go) {
  await until('Escape', (f) => !!f.go, 4);
  const items = ['home', 'movies', 'series', 'kiosk', 'search', 'settings'];
  const f = await focused();
  const d = items.indexOf(go) - items.indexOf(f.go);
  await press(d > 0 ? 'ArrowRight' : 'ArrowLeft', Math.abs(d));
  await press('Enter');
  await idle();
}

await page.goto(`${APP}/?server=${encodeURIComponent(SERVER)}`);
await page.waitForSelector('.home-rows .card', { timeout: 15000 });
ok((await page.locator('#menu .menu-item').allTextContents()).join('|').includes('Series|Kiosco|⌕ Buscar'), 'menú: Inicio · Películas · Series · Kiosco · Buscar');

// ---------- 1. Lista: bajar varias filas, ficha y volver (tema 1) ----------
await menuTo('movies');
ok((await screenName()) === 'movies', 'Películas abierta');
ok((await page.locator('.grid-head h1').textContent()).includes('46 títulos'), 'título con el número: ' + await page.locator('.grid-head h1').textContent());
ok((await page.locator('.chip[data-k="sort:runtime"]').count()) === 1, 'orden por Duración');
await until('ArrowDown', (f) => !!f.id, 4);
await press('ArrowDown', 3, 250);
await press('ArrowRight', 2);
const before = await focused();
ok(before.id && before.scroll > 300, `foco en la 4.ª fila (${before.text.trim()}, desplazamiento ${before.scroll})`);
await press('Enter'); await idle();
ok((await screenName()) === 'detail', 'ficha abierta');
await press('Escape', 1, 400);
let after = await focused();
ok(after.id === before.id && after.scroll === before.scroll, `ficha → Atrás: misma tarjeta y desplazamiento (${after.text.trim()}, ${after.scroll})`);

// Pasando por el reproductor: Reproducir → Atrás → Atrás.
await press('Enter'); await idle();
await press('Enter', 1, 2500);
ok(await page.evaluate(() => window.YF.player.active), 'reproduciendo (AVPlay simulado)');
await press('Escape', 1, 1200);
ok((await screenName()) === 'detail' && !(await page.evaluate(() => window.YF.player.active)), 'Atrás: sale del vídeo a la ficha');
await press('Escape', 1, 600);
after = await focused();
ok(after.id === before.id && after.scroll === before.scroll && (await page.locator('.grid .card').count()) >= 42, `ficha → vídeo → Atrás → Atrás: misma tarjeta y desplazamiento, sin recargar (${after.text.trim()}, ${after.scroll})`);

// ---------- 2. Ficha navegable: director → sumar década → Atrás ----------
await press('Enter'); await idle();
let f = await until('ArrowDown', (x) => x.k.startsWith('director:'), 6);
ok(f && f.k.startsWith('director:'), 'ficha: ▼ desde Reproducir llega a la píldora de Dirección: ' + (f && f.text));
await press('ArrowDown');
const castPill = await focused();
ok(castPill.k.startsWith('actor:'), 'de Dirección ▼ al primer actor: ' + castPill.text);
await press('ArrowUp');
ok((await focused()).k.startsWith('director:'), 'y ▲ vuelve a Dirección');
await shot('tv-ficha-pildoras');
const director = f.k.slice(9);
await press('Enter'); await idle();
let h1 = await page.locator('.screen:not(.hidden) .grid-head h1').textContent();
ok(h1.startsWith('Películas · ' + director + ' · ') && / \d+ títulos?$/.test(h1), 'lista filtrada: ' + h1);
await until('ArrowDown', (x) => x.k === 'by:genre', 3);
await rowTo('by:when');
await press('Enter'); await idle();
ok((await screenName()) === 'picker' && await page.locator('.picker .pitem.sel').count() === 1, 'panel Año/Década abierto');
await press('Enter', 1, 800);
h1 = await page.locator('.screen:not(.hidden) .grid-head h1').textContent();
ok(h1.includes(director) && h1.includes('Años'), 'década sumada al momento con el panel abierto: ' + h1);
await press('Escape', 1, 400);
ok((await focused()).k === 'by:when', 'Atrás cierra el panel con el foco en «Año/Década»');
ok(await page.locator('.screen:not(.hidden) .chip.active').count() === 2, 'dos filtros activos como píldoras con ✕');
await shot('tv-lista-filtrada');
// Quitar la década con OK sobre su píldora.
await until('ArrowDown', (x) => x.k.startsWith('rm:'), 2);
await rowTo('rm:decade');
await press('Enter', 1, 800);
h1 = await page.locator('.screen:not(.hidden) .grid-head h1').textContent();
ok(!h1.includes('Años') && h1.includes(director), 'OK sobre la píldora quita el filtro: ' + h1);
// Ver en kiosco desde la lista filtrada.
await until('ArrowUp', (x) => x.k.startsWith('by:'), 3);
await rowTo('kiosk');
await press('Enter'); await idle(); await wait(1200);
ok((await page.locator('.kcount').textContent()).startsWith('Kiosco · ' + director), 'Ver en kiosco: ' + await page.locator('.kcount').textContent());
await press('Escape', 1, 500);
await press('Escape', 1, 500);
f = await focused();
ok((await screenName()) === 'detail' && f.k.startsWith('director:'), 'Atrás, Atrás: de vuelta en la ficha con el foco en el director');
await press('Escape', 1, 500);
ok((await focused()).id === before.id, 'y Atrás otra vez: en la lista, en la misma tarjeta');

// ---------- 3. «Filtrar por» → Director, salto por letra ----------
await until('ArrowUp', (x) => x.k.startsWith('by:'), 12);
await rowTo('by:director');
await press('Enter'); await idle();
const firstSel = await page.locator('.picker .pitem.sel .pname').textContent();
await press('ArrowDown');
await press('ArrowRight', 2, 250);
const letter = await page.locator('.pletter').textContent();
const sel = await page.locator('.picker .pitem.sel .pname').textContent();
ok(sel.toUpperCase().normalize('NFD').replace(/[̀-ͯ]/g, '')[0] === letter && letter > 'A', `◀▶ salta por letra: ${firstSel} → ${letter} ${sel}`);
await shot('tv-filtrar-directores');
await press('Enter', 1, 800);
h1 = await page.locator('.screen:not(.hidden) .grid-head h1').textContent();
ok(h1.includes(sel), 'OK elige y la lista cambia: ' + h1);
await press('Escape', 1, 400);
await until('ArrowDown', (x) => x.k === 'rm:director', 2);
await press('Enter', 1, 800);
ok((await page.locator('.screen:not(.hidden) .grid-head h1').textContent()).includes('46 títulos'), 'filtro quitado: otra vez 46 títulos');
// Actores: por defecto 2 o más; «Todos» con all=1.
await until('ArrowDown', (x) => x.k.startsWith('by:'), 2);
await rowTo('by:actor');
await press('Enter'); await idle();
const counts2 = await page.evaluate(() => [...document.querySelectorAll('.picker .pitem .pcount')].slice(1).map((e) => +e.textContent));
ok(counts2.length && counts2.every((n) => n >= 2), 'actores: solo con 2 o más títulos por defecto');
await press('ArrowUp', 2); await press('ArrowDown');
await press('Enter', 1, 900);
const counts1 = await page.evaluate(() => [...document.querySelectorAll('.picker .pitem .pcount')].map((e) => +e.textContent));
ok(counts1.some((n) => n === 1), 'interruptor «Todos»: aparecen actores con 1 título');
await press('Escape', 1, 400);

// ---------- 4. Kiosco ----------
await menuTo('kiosk');
await page.waitForSelector('.kslide.shown .kinfo h1', { timeout: 8000 }); await wait(800);
ok(await page.evaluate(() => document.getElementById('app').classList.contains('fullscreen')), 'kiosco a pantalla completa (sin menú)');
const kt = () => page.locator('.kslide.shown .kinfo h1').textContent();
const t0 = await kt();
ok((await page.locator('.kslide.shown .kbg').getAttribute('src')).includes('w=1920'), 'fondo a 1920 px');
await shot('tv-kiosco');
await press('ArrowRight', 1, 1000);
const t1 = await kt();
await press('ArrowLeft', 1, 1000);
ok(t1 !== t0 && (await kt()) === t0, `◀▶: ${t0} → ${t1} → ${t0} (la anterior es la misma)`);
await press('ArrowRight', 1, 1000);
await press('Enter', 1, 300);
ok(await page.locator('.kbtns:not(.hidden) .btn.focused').textContent() === '▶ Reproducir', 'OK: Reproducir seleccionado');
await press('ArrowDown');
ok(await page.locator('.kbtns .btn.focused').textContent() === 'Ver ficha', '▼: Ver ficha');
await shot('tv-kiosco-botones');
await press('Enter'); await idle();
ok((await screenName()) === 'detail' && (await page.locator('.screen:not(.hidden) .detail h1').textContent()) === t1, 'Ver ficha abre la ficha de ' + t1);
await press('Escape', 1, 900);
ok((await screenName()) === 'kiosk' && (await kt()) === t1, 'Atrás: kiosco en el mismo título');
await press('Enter'); await press('Enter', 1, 2500);
ok(await page.evaluate(() => window.YF.player.active), 'Reproducir desde el kiosco');
await press('Escape', 1, 1500);
ok((await screenName()) === 'kiosk' && (await kt()) === t1 && !(await page.evaluate(() => window.YF.player.active)), 'Atrás del vídeo: kiosco en el mismo título');
// Filtro: Series.
await press('ArrowDown', 1, 400);
ok(await page.locator('.kbar:not(.hidden)').count() === 1, '▼ abre los filtros del kiosco');
await rowTo('kind:series');
await press('Enter');
await press('Escape', 1, 1500);
ok((await page.locator('.kslide.shown .eyebrow').textContent()) === 'Serie', 'filtro Series aplicado');
await press('Escape', 1, 400);
ok((await focused()).go === 'kiosk' && !(await page.evaluate(() => document.getElementById('app').classList.contains('fullscreen'))), 'Atrás vuelve al menú');

ok(await page.evaluate(() => window.simHangs.length === 0), 'AVPlay simulado: sin cuelgues');
ok(errors.length === 0, 'errores de JavaScript: ' + errors.length + (errors.length ? ' → ' + errors.join(' | ') : ''));
await browser.close();
console.log(failed ? `\n${failed} comprobaciones fallidas` : '\nTodo bien');
process.exit(failed ? 1 : 0);
