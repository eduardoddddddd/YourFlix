// node --test tv/tests/*.test.js — filtros, enlaces de la ficha, kiosco y reglas de Chromium 85.
const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const { filters: F } = require('../js/filters.js');
const { api } = require('../js/api.js');
const { nav } = require('../js/nav.js');

test('filtros ↔ parámetros: se suman categorías distintas y se sustituye la misma', () => {
  let l = F.add([], { key: 'director', value: 'Ridley Scott', name: 'Ridley Scott' });
  l = F.add(l, { key: 'decade', value: '1980', name: 'Años 80' });
  l = F.add(l, { key: 'state', value: 'unwatched', name: 'Sin ver' });
  assert.deepEqual(l.map((x) => x.key), ['director', 'decade', 'state']);
  assert.equal(F.title('movie', l, 3), 'Películas · Ridley Scott · Años 80 · Sin ver · 3 títulos');
  assert.equal(F.title('series', [], 1), 'Series · 1 título');
  l = F.add(l, { key: 'decade', value: 1990, name: 'Años 90' });
  assert.equal(l.filter((x) => x.key === 'decade').length, 1);
  assert.equal(l[1].value, '1990');
  const q = F.params(l, { kind: 'movie', sort: 'az' });
  assert.deepEqual(q, { kind: 'movie', sort: 'az', director: 'Ridley Scott', decade: '1990', state: 'unwatched' });
  assert.equal(api.qs(q), '?kind=movie&sort=az&director=Ridley%20Scott&decade=1990&state=unwatched');
  assert.deepEqual(F.params(l, { kind: 'movie' }, ['decade']).decade, undefined, 'el panel de una categoría no la envía');
  assert.equal(api.qs({ actor: ['A', 'B'], genre: '', x: null }), '?actor=A&actor=B', 'repetibles y sin vacíos');
  assert.deepEqual(F.fromQuery({ director: 'Ridley Scott', decade: '1980', kind: 'movie' }).map((x) => x.key + '=' + x.value), ['director=Ridley Scott', 'decade=1980']);
  assert.deepEqual(F.remove(l, 'director').map((x) => x.key), ['decade', 'state']);
  assert.ok(F.has(l, 'state', 'unwatched') && !F.has(l, 'state', 'watched') && F.has(l, 'director'));
});

test('enlaces de la ficha: filas y número solo con 2 o más', () => {
  const rows = F.linkRows({
    director: [{ name: 'Ridley Scott', count: 5 }],
    cast: [{ name: 'Michael Fassbender', count: 2 }, { name: 'Billy Crudup', count: 1 }],
    country: [{ name: 'Estados Unidos', count: 41 }],
    genres: [{ name: 'Terror', count: 7 }],
    year: { name: '2017', count: 1 },
    decade: { name: 'Años 2010', value: '2010', count: 10 },
    runtime: { name: 'Más de 120 min', value: 'long', count: 24 },
    quality: [{ name: '1080p', value: '1080', count: 0 }],
  }, 'movie');
  assert.deepEqual(rows.map((r) => r.label), ['Dirección', 'Reparto', 'Más']);
  assert.equal(rows[0].items[0].text, 'Ridley Scott (5)');
  assert.deepEqual(rows[1].items.map((x) => x.text), ['Michael Fassbender (2)', 'Billy Crudup']);
  assert.deepEqual(rows[2].items.map((x) => x.key + '=' + x.value + ':' + x.text),
    ['country=Estados Unidos:Estados Unidos (41)', 'year=2017:2017', 'decade=2010:Años 2010 (10)', 'genre=Terror:Terror (7)', 'runtime=long:Más de 120 min (24)']);
  assert.equal(rows[2].items[2].name, 'Años 2010');
  assert.equal(F.linkRows({ director: [{ name: 'X', count: 1 }] }, 'series')[0].label, 'Creada por');
  assert.deepEqual(F.linkRows(null), []);
});

test('panel: grupos, cabeceras y orden alfabético en listas largas', () => {
  const g = F.GROUPS.find((x) => x.id === 'director');
  const it = F.pickerItems(g, { director: [{ value: 'Oliver Stone', count: 8 }, { value: 'Álex de la Iglesia', count: 2 }, { value: 'Martin Scorsese', count: 4 }] });
  assert.deepEqual(it.map((x) => x.name), ['Álex de la Iglesia', 'Martin Scorsese', 'Oliver Stone']);
  const when = F.GROUPS.find((x) => x.id === 'when');
  const w = F.pickerItems(when, { decade: [{ value: '1980', name: 'Años 80', count: 2 }], year: [{ value: '1982', count: 1 }] });
  assert.deepEqual(w.map((x) => x.head || x.name), ['Década', 'Años 80', 'Año', '1982']);
});

test('salto por letra', () => {
  const n = [null, 'Abel', 'Álvaro', 'Bea', 'Berta', 'Carla', 'Ñu', '9 reinas'];
  assert.equal(F.letter('Álvaro'), 'A');
  assert.equal(F.letter('9 reinas'), '#');
  assert.equal(F.letterJump(n, 1, 1), 3, 'A → B');
  assert.equal(F.letterJump(n, 3, 1), 5, 'B → C');
  assert.equal(F.letterJump(n, 4, -1), 3, 'en mitad de la B: al principio de la B');
  assert.equal(F.letterJump(n, 3, -1), 1, 'principio de la B: al principio de la A');
  assert.equal(F.letterJump(n, 5, -1), 3);
  assert.equal(F.letterJump(n, 7, 1), 7, 'sin más letras se queda');
  assert.equal(F.letterJump(n, 1, -1), 1, 'antes de la A solo hay entradas sin nombre');
  assert.equal(F.letterJump(n, 0, 1), 1, 'desde el interruptor, a la primera letra');
});

test('kiosco: índice circular, semilla, imagen grande, insignias y ciclo de filtros', () => {
  assert.equal(F.circ(-1, 5), 4);
  assert.equal(F.circ(5, 5), 0);
  assert.equal(F.circ(12, 5), 2);
  assert.equal(F.circ(3, 0), 0);
  const s = F.seed();
  assert.ok(Number.isInteger(s) && s > 0);
  assert.equal(F.big('/api/v1/titles/t7-69/backdrop?w=1280'), '/api/v1/titles/t7-69/backdrop?w=1920');
  assert.equal(F.big(''), '');
  assert.deepEqual(F.kioskBadges({ video: { width: 3840, hdr: 'HDR10' }, audio: [{ language: 'spa' }], subtitles: [] }), ['4K', 'HDR']);
  assert.deepEqual(F.kioskBadges({ video: { width: 1920 }, audio: [{ language: 'eng' }], subtitles: [{ language: 'spa', text: true }] }), ['VOSE']);
  assert.deepEqual(F.kioskBadges({ video: {}, audio: [{ language: 'eng' }], subtitles: [{ language: 'spa', text: false }] }), [], 'PGS no cuenta: no se ve');
  assert.deepEqual(F.kioskBadges(null), []);
  assert.equal(F.cycle(['Drama', 'Terror'], ''), 'Drama');
  assert.equal(F.cycle(['Drama', 'Terror'], 'Terror'), '');
});

const R = (left, top, w, h) => ({ left, top, width: w, height: h, right: left + w, bottom: top + h });

test('navegación de la ficha: botones y filas de píldoras de anchos distintos', () => {
  // Botones (y=600), Dirección (1 píldora), Reparto (5), Más (4). Etiqueta de 150 px a la izquierda.
  const c = [
    R(90, 600, 300, 70), R(410, 600, 340, 70), // 0 Reproducir, 1 Desde el principio
    R(244, 700, 220, 46), // 2 Ridley Scott
    R(244, 760, 260, 46), R(512, 760, 200, 46), R(720, 760, 240, 46), R(968, 760, 200, 46), R(1176, 760, 200, 46), // 3-7 reparto
    R(244, 820, 200, 46), R(452, 820, 90, 46), R(550, 820, 160, 46), R(718, 820, 120, 46), // 8-11 más
  ];
  assert.equal(nav.pick(c[0], c, 'down'), 2, 'de Reproducir a Dirección');
  assert.equal(nav.pick(c[1], c, 'down'), 2, 'del segundo botón también a la única píldora de Dirección');
  assert.equal(nav.pick(c[2], c, 'down'), 3, 'de Dirección al primer actor');
  assert.equal(nav.pick(c[6], c, 'down'), 11, 'de un actor a la derecha, a la última de Más');
  assert.equal(nav.pick(c[6], c, 'up'), 2, 'de un actor a la derecha, arriba a Dirección (fila corta)');
  assert.equal(nav.pick(c[2], c, 'up'), 0, 'de Dirección a Reproducir');
  assert.equal(nav.pick(c[5], c, 'up'), 2);
  assert.equal(nav.pick(c[9], c, 'up'), 4, 'de Más al actor de encima (el más cercano en horizontal)');
  assert.equal(nav.pick(c[3], c, 'right'), 4);
  assert.equal(nav.pick(c[7], c, 'right'), -1, 'fin de la fila: no salta a otra');
});

test('Chromium 85: ningún JS ni CSS de tv/ usa ?. ?? ni aspect-ratio', () => {
  const files = [];
  const walk = (d) => fs.readdirSync(d, { withFileTypes: true }).forEach((e) => {
    const p = path.join(d, e.name);
    if (e.isDirectory()) { if (e.name !== 'tests') walk(p); } else if (/\.(js|css|html)$/.test(e.name)) files.push(p);
  });
  walk(root);
  assert.ok(files.length >= 8);
  for (const f of files) {
    const src = fs.readFileSync(f, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');
    assert.ok(!/\?\.(?!\d)/.test(src), f + ': ?.');
    assert.ok(!/\?\?/.test(src), f + ': ??');
    assert.ok(!/aspect-ratio/.test(src), f + ': aspect-ratio');
    assert.ok(!/\b(replaceAll|structuredClone|\.at\(|Object\.hasOwn)\b/.test(src), f + ': API posterior a Chromium 85');
  }
});

test('menú y versión', () => {
  const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
  const order = [...html.matchAll(/data-go="(\w+)"/g)].map((m) => m[1]);
  assert.deepEqual(order, ['home', 'movies', 'series', 'kiosk', 'search', 'settings']);
  assert.ok(html.indexOf('js/filters.js') < html.indexOf('js/screens.js'));
  const cfg = fs.readFileSync(path.join(root, 'config.xml'), 'utf8');
  const app = fs.readFileSync(path.join(root, 'js', 'app.js'), 'utf8');
  const v = /<widget[^>]*version="([\d.]+)"/.exec(cfg)[1];
  assert.ok(app.includes("YF.version = '" + v + "'"), 'misma versión en config.xml y app.js');
});
