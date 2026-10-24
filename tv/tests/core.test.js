// node --test tv/tests/*.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const util = require('../js/util.js');
const { nav } = require('../js/nav.js');
const player = require('../js/player.js');
const { api } = require('../js/api.js');

test('formatos', () => {
  assert.equal(util.clock(6863), '1:54:23');
  assert.equal(util.clock(300), '5:00');
  assert.equal(util.clock(-1), '--:--');
  assert.equal(util.runtime(116), '1 h 56 min');
  assert.equal(util.runtime(45), '45 min');
  assert.equal(util.lang('spa'), 'ES');
});

test('distintivos técnicos', () => {
  const b = util.techBadges({
    video: { codec: 'hevc', width: 3840, height: 2160, bitDepth: 10, hdr: 'HDR10' },
    audio: [{ codec: 'eac3', channels: 6, language: 'spa' }, { codec: 'aac', channels: 2, language: 'eng' }],
    subtitles: [{ codec: 'subrip', text: true, language: 'spa' }, { codec: 'hdmv_pgs_subtitle', text: false, language: 'eng' }],
  });
  assert.deepEqual(b, ['4K', 'HDR10', 'HEVC 10 bits', '5.1', 'Audio ES·EN', 'Sub ES']);
});

const R = (left, top, w = 200, h = 300) => ({ left, top, width: w, height: h, right: left + w, bottom: top + h });

test('navegación espacial: filas y cambio de fila', () => {
  // Fila 1 en y=100, fila 2 en y=500 (desplazada). Un botón del héroe arriba.
  const cands = [R(90, 100), R(312, 100), R(534, 100), R(90, 500), R(312, 500), R(534, 500), R(90, -100, 300, 60)];
  assert.equal(nav.pick(cands[0], cands, 'right'), 1, 'derecha: siguiente de la misma fila');
  assert.equal(nav.pick(cands[2], cands, 'right'), -1, 'fin de fila');
  assert.equal(nav.pick(cands[1], cands, 'down'), 4, 'abajo: la de debajo, no la más cercana en diagonal');
  assert.equal(nav.pick(cands[4], cands, 'up'), 1);
  assert.equal(nav.pick(cands[0], cands, 'up'), 6, 'arriba de la primera fila: el héroe/menú');
  assert.equal(nav.pick(cands[0], cands, 'left'), -1);
});

test('navegación espacial: de un botón ancho a la rejilla', () => {
  const btn = R(800, 100, 400, 60);
  const grid = [R(90, 300), R(600, 300), R(1000, 300), R(1500, 300)];
  assert.equal(nav.pick(btn, grid.concat([btn]), 'down'), 2, 'la tarjeta más centrada bajo el botón');
});

test('cola de saltos: nunca dos a la vez y las pulsaciones seguidas se suman', () => {
  const calls = [];
  let pending = null;
  const q = new player.SeekQueue((ms, done) => { calls.push(ms); pending = done; });
  assert.equal(q.request(10000), true);
  assert.equal(q.busy, true);
  // Tres pulsaciones de +10 s mientras el primer salto rellena el búfer.
  let base = q.base(0);
  q.request(player.seekTarget(base, 10000, 60000));
  base = q.base(0);
  q.request(player.seekTarget(base, 10000, 60000));
  base = q.base(0);
  q.request(player.seekTarget(base, 10000, 60000));
  assert.deepEqual(calls, [10000], 'el segundo salto espera');
  pending();
  assert.deepEqual(calls, [10000, 40000], 'un único salto acumulado');
  let idle = 0;
  q.onIdle = () => idle++;
  pending();
  assert.equal(q.busy, false);
  assert.equal(idle, 1, 'onIdle al quedar libre (reanudar: play tras el salto)');
  assert.equal(q.base(1234), 1234, 'sin pendientes, la base es la posición actual');
});

test('cola de saltos: reanudar con pulsaciones durante la carga', () => {
  // prepare → salto de reanudar (antes de play) → el usuario pulsa ⏩ dos veces → play.
  const calls = [];
  let pending = null;
  const q = new player.SeekQueue((ms, done) => { calls.push(ms); pending = done; });
  let started = 0, release = null;
  q.onIdle = () => { started++; release = q.block(); };
  q.request(281000);
  q.request(player.seekTarget(q.base(0), 30000, 6863000));
  q.request(player.seekTarget(q.base(0), 30000, 6863000));
  pending(false); // salto de reanudar hecho, aún sin reproducir
  assert.deepEqual(calls, [281000], 'no salta otra vez antes de arrancar');
  assert.equal(started, 1, 'arranca la reproducción');
  release(); // la reproducción ya avanza
  assert.deepEqual(calls, [281000, 341000], 'los dos ⏩ acumulados, en un solo salto');
});

test('cola de saltos: bloqueo durante el arranque', () => {
  const calls = [];
  const q = new player.SeekQueue((ms, done) => { calls.push(ms); done(); });
  const release = q.block();
  q.request(30000);
  q.request(60000);
  assert.deepEqual(calls, [], 'nada mientras arranca');
  release();
  assert.deepEqual(calls, [60000], 'al arrancar, solo el último destino');
  release();
  assert.deepEqual(calls, [60000], 'liberar dos veces no repite');
});

test('subtítulos: línea visible por tiempo', () => {
  const cues = [[1, 2.5, 'uno'], [3, 4, 'dos'], [3.5, 6, 'solapada'], [10, 12, 'tres']];
  assert.equal(player.cueAt(cues, 0.5), null);
  assert.equal(player.cueAt(cues, 1.2)[2], 'uno');
  assert.equal(player.cueAt(cues, 2.7), null, 'hueco entre líneas');
  assert.equal(player.cueAt(cues, 3.7)[2], 'solapada');
  assert.equal(player.cueAt(cues, 4.5)[2], 'solapada', 'la anterior ya terminó, la solapada sigue');
  assert.equal(player.cueAt(cues, 11)[2], 'tres');
  assert.equal(player.cueAt([], 5), null);
});

test('subtítulos: preferencia y forzados', () => {
  player.lang = util.lang; // en la TV, YF es un único objeto
  const tracks = [
    { id: 'e3', language: 'spa', forced: true },
    { id: 'e4', language: 'spa', default: true },
    { id: 'e5', language: 'eng' },
  ];
  assert.equal(player.pickSubtitle(tracks, 'ES', 'EN'), 'e4', 'preferencia ES: la completa, no la forzada');
  assert.equal(player.pickSubtitle(tracks, 'EN', 'ES'), 'e5');
  assert.equal(player.pickSubtitle(tracks, null, 'ES'), 'e3', 'audio en español: forzados en español solos');
  assert.equal(player.pickSubtitle(tracks, 'off', 'ES'), 'e3', 'los forzados se ven aunque estén desactivados');
  assert.equal(player.pickSubtitle(tracks, 'off', 'EN'), null);
  assert.equal(player.pickSubtitle(tracks, 'FR', 'EN'), null, 'idioma preferido inexistente');
});

test('audio: si la pista por defecto es DTS, pasar a una que suene', () => {
  // «Alien contra Predator»: DTS español por defecto + AC3 inglés. AVPlay solo lista la AC3.
  const file = [{ codec: 'dts', default: true }, { codec: 'ac3' }];
  assert.equal(player.audioStart(file, [{ index: 2, codec: 'AC3' }]), 0);
  assert.equal(player.audioStart(file, [{ index: 1, codec: 'DTS' }, { index: 2, codec: 'AC3' }]), 1, 'si AVPlay la listara, saltarla');
  assert.equal(player.audioStart([{ codec: 'dts' }], []), -1, 'solo DTS: no hay nada que elegir');
  assert.equal(player.audioStart([{ codec: 'ac3', default: true }, { codec: 'dts' }], [{ index: 1, codec: 'AC3' }]), -1, 'por defecto compatible: no tocar');
  assert.equal(player.audioStart([], []), -1);
});

test('destino de salto: sin pasar del final ni del principio', () => {
  assert.equal(player.seekTarget(55000, 30000, 60000), 55000, 'cinco segundos antes del final (más allá, AVPlay falla)');
  assert.equal(player.seekTarget(5000, -30000, 60000), 0);
  assert.equal(player.seekTarget(1000, 10000, 0), 11000, 'duración desconocida');
});

test('descubrimiento: candidatos de la red de la TV', () => {
  const c = api.subnetCandidates('192.168.1.4');
  assert.equal(c.length, 253);
  assert.ok(c.includes('http://192.168.1.3:8787'));
  assert.ok(!c.includes('http://192.168.1.4:8787'), 'la propia TV no');
  assert.deepEqual(api.subnetCandidates(''), []);
});

test('reglas de AstroIPTV y Chromium 85', () => {
  const js = fs.readdirSync(path.join(root, 'js')).filter((f) => f.endsWith('.js') && f !== 'avplay-sim.js')
    .map((f) => fs.readFileSync(path.join(root, 'js', f), 'utf8')).join('\n')
    .replace(/\/\*[\s\S]*?\*\/|\/\/.*$/gm, ''); // los comentarios pueden citar las reglas
  assert.ok(!/setSelectTrack\(\s*['"]TEXT/.test(js), 'nunca setSelectTrack TEXT: colgó la TV');
  assert.ok(!/call\('setSelectTrack', 'TEXT'/.test(js));
  assert.ok(!/\?\.|\?\?/.test(js), 'sin ?. ni ?? (riesgo en Chromium 85)');
  assert.ok(!/\.innerHTML\s*=/.test(js), 'datos siempre por textContent');
  const css = fs.readFileSync(path.join(root, 'css', 'app.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
  assert.ok(!/\binset\s*:/.test(css), 'inset no existe en Chromium 85 (ficha invisible en AstroIPTV)');
  assert.ok(!/aspect-ratio/.test(css));
  assert.ok(!/:focus-visible/.test(css), ':focus-visible es de Chromium 86');
  const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
  assert.ok(/<body>\s*(<!--[^>]*-->\s*)?<object id="av-player"/.test(html), 'el objeto AVPlay es el primer hijo de body');
  const cfg = fs.readFileSync(path.join(root, 'config.xml'), 'utf8');
  assert.ok(cfg.includes('id="Z8YOURFLX1.YourFlix"'), 'ID fijo: cambiarlo pierde los datos de la TV');
});
