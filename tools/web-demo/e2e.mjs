// Recorrido de la web con Chromium (Playwright) contra un servidor YourFlix con la biblioteca de
// make_demo.sh: Inicio, Películas, Series, Búsqueda, Ficha y reproducción de cada archivo
// (directo o HLS), salto a la mitad, cambio de audio, subtítulos y progreso guardado.
//   node tools/web-demo/e2e.mjs [http://localhost:8787] [carpeta-de-capturas]
// Es una prueba en navegador de escritorio (Chromium sin HEVC por hardware): no sustituye a
// probar en iPhone, Android, Mac y Windows.
import { createRequire } from 'node:module';
import fs from 'node:fs';
const require = createRequire(import.meta.url);
const { chromium } = require('playwright');

const BASE = process.argv[2] || 'http://localhost:8787';
const SHOTS = process.argv[3] || '';
if (SHOTS) fs.mkdirSync(SHOTS, { recursive: true });
const errors = [];
let failed = 0;
const ok = (cond, msg) => { console.log((cond ? '✔ ' : '✘ ') + msg); if (!cond) failed++; };
const api = (p) => fetch(BASE + '/api/v1' + p).then((r) => r.json());

const browser = await chromium.launch();
async function newPage(opts) {
  const page = await browser.newPage(opts);
  page.on('pageerror', (e) => errors.push(String(e)));
  page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
  return page;
}
const shot = (page, name) => SHOTS && page.screenshot({ path: `${SHOTS}/${name}.png` });

const page = await newPage({ viewport: { width: 1440, height: 900 } });
await page.goto(BASE + '/');
await page.waitForSelector('.row .card');
ok((await page.locator('.row h2').allTextContents()).includes('Añadidas recientemente'), 'Inicio con la fila «Añadidas recientemente»');
ok(!(await page.content()).includes('Samsung T7') && !(await page.content()).includes('Patriot'), 'no se muestra el soporte de origen');
await shot(page, '01-inicio');

await page.click('#nav a[data-nav=movies]');
await page.waitForSelector('.grid .card');
ok((await page.locator('.grid .card').count()) >= 4, 'Películas: rejilla con las películas');
await page.click('.chip:has-text("Por año")');
await page.waitForFunction(() => location.hash.includes('orden=year'));
await page.waitForSelector('.grid .card');
ok((await page.locator('.grid .card .cap').first().textContent()).includes('2019'), 'orden por año (la más reciente primero)');
await shot(page, '02-peliculas');

await page.click('#nav a[data-nav=series]');
await page.waitForSelector('.grid .card');
await page.click('.grid .card');
await page.waitForSelector('.episode');
ok((await page.locator('.seasons .chip').count()) === 2, 'Serie con dos temporadas');
await page.click('.seasons .chip:has-text("Temporada 2")');
ok((await page.locator('.episode').count()) === 1, 'Temporada 2 con un capítulo');
await shot(page, '03-serie');

await page.click('#nav a[data-nav=search]');
await page.fill('.search-box input', 'tres');
await page.waitForSelector('.grid .card');
ok((await page.locator('.grid .card').count()) === 1, 'Búsqueda «tres» → 1 resultado');
await shot(page, '04-buscar');

const titles = (await api('/titles?limit=50')).items;
const byName = (n) => titles.find((t) => t.title === n);

async function play(name, expect) {
  const t = byName(name);
  const d = await api('/titles/' + t.id);
  const f = d.files[0];
  await page.goto(`${BASE}/#/titulo/${t.id}`);
  await page.waitForSelector('.detail .btn.primary');
  const how = await page.locator('.how').first().textContent();
  await page.click('.detail .btn.primary');
  await page.waitForSelector('#player:not(.hidden)');
  const started = await page.waitForFunction(() => { const v = document.querySelector('#player video'); return v && v.currentTime > 1 && !v.paused; }, null, { timeout: 90000 }).then(() => true, () => false);
  const info = await page.evaluate(() => YF.player.info());
  ok(started, `${name}: reproduce (${info && info.method} ${info && info.mode || ''}) — ficha: «${how}»`);
  ok(info && info.method === expect.method && (info.mode || '') === (expect.mode || ''), `${name}: método ${expect.method} ${expect.mode || ''}`);
  // Salto a la mitad: el vídeo sigue avanzando desde allí.
  const mid = Math.floor(f.duration / 2);
  await page.evaluate((m) => { document.querySelector('#player video').currentTime = m; }, mid);
  const seeked = await page.waitForFunction((m) => { const v = document.querySelector('#player video'); return v.currentTime > m + 1 && v.currentTime < m + 20; }, mid, { timeout: 90000 }).then(() => true, () => false);
  ok(seeked, `${name}: salto a ${mid} s y sigue`);
  await shot(page, '05-player-' + name.replace(/\W+/g, '-'));
  return { t, f, info };
}

// Prueba cuatro: MP4 H.264 + AAC → directo; SRT externo.
let r = await play('Prueba cuatro', { method: 'direct' });
await page.evaluate(() => document.querySelector('.ibtn[aria-label^="Subtítulos"]').click());
await page.click('.menu button:has-text("Español")');
const cues = await page.waitForFunction(() => { const tt = document.querySelector('#player video').textTracks[0]; return tt && tt.cues && tt.cues.length >= 3; }, null, { timeout: 30000 }).then(() => true, () => false);
ok(cues, 'Prueba cuatro: subtítulos externos en WebVTT cargados en <track>');
await page.click('.pl-top .ibtn');
await page.waitForSelector('#player.hidden', { state: 'attached' });
const prog = (await api('/titles/' + r.t.id)).files[0].progress;
ok(prog && prog.position > 60, `progreso guardado al salir (${prog && prog.position} s), el mismo que usa la TV`);

// Prueba uno: MKV H.264 + AC3 5.1 → HLS con vídeo copiado; segunda pista AAC; SRT interno.
r = await play('Prueba uno', { method: 'hls', mode: 'copy' });
await page.evaluate(() => document.querySelector('.ibtn[aria-label="Audio"]').click());
await page.click('.menu button:has-text("EN")');
const switched = await page.waitForFunction(() => { const i = YF.player.info(); const v = document.querySelector('#player video'); return i && i.audio === 2 && v.currentTime > 1 && !v.paused; }, null, { timeout: 60000 }).then(() => true, () => false);
const after = await page.evaluate(() => [YF.player.info(), document.querySelector('#player video').currentTime]);
ok(switched && after[1] > r.f.duration / 2 - 5, `Prueba uno: cambio a la pista inglesa sin perder la posición (${after[0] && after[0].variant}, ${Math.round(after[1])} s)`);
await page.evaluate(() => document.querySelector('.ibtn[aria-label^="Subtítulos"]').click());
await page.click('.menu button:has-text("Español")');
const cues2 = await page.waitForFunction(() => { const tt = document.querySelector('#player video').textTracks[0]; return tt && tt.cues && tt.cues.length >= 3; }, null, { timeout: 60000 }).then(() => true, () => false);
ok(cues2, 'Prueba uno: subtítulos internos (extraídos con ffmpeg) en WebVTT');

// Prueba dos: HEVC Main10 + E-AC3. Este Chromium no decodifica HEVC → recodifica a H.264.
await play('Prueba dos', { method: 'hls', mode: 'h264' });
// Prueba tres: AVI XviD + MP3 → recodifica a H.264.
await play('Prueba tres', { method: 'hls', mode: 'h264' });
await page.keyboard.press('Escape');
await page.waitForSelector('#player.hidden', { state: 'attached' });

// Al salir, la web avisa al servidor (sendBeacon) y ffmpeg se para.
let st;
for (let i = 0; i < 20; i++) { st = await api('/remux'); if (st.jobs === 0) break; await new Promise((r) => setTimeout(r, 250)); }
ok(st.jobs === 0, `conversiones activas tras salir del reproductor: ${st.jobs} (máx. ${st.maxJobs})`);

// Móvil (táctil, estrecho): barra inferior y reproductor.
const m = await newPage({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
await m.goto(BASE + '/');
await m.waitForSelector('.row .card');
ok(await m.locator('#nav').evaluate((n) => getComputedStyle(n).position === 'fixed' && n.getBoundingClientRect().bottom >= innerHeight - 1), 'móvil: navegación abajo');
ok(await m.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'móvil: sin desplazamiento horizontal');
await shot(m, '06-movil-inicio');
await m.goto(`${BASE}/#/titulo/${byName('Prueba uno').id}`);
await m.waitForSelector('.detail .btn.primary');
await shot(m, '07-movil-ficha');

ok(errors.length === 0, 'sin errores de JavaScript' + (errors.length ? ': ' + errors.join(' | ') : ''));
await browser.close();
process.exit(failed ? 1 : 0);
