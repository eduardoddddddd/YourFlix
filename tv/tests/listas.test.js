// Favoritos y Pendientes de ver: peticiones a la API y navegación hacia los botones ♥ y +
// superpuestos a la foto de cada capítulo.
const test = require('node:test');
const assert = require('node:assert');
const { nav } = require('../js/nav.js');
const { api } = require('../js/api.js');

const R = (left, top, w, h) => ({ left, top, width: w, height: h, right: left + w, bottom: top + h });

test('api.setList: PUT para añadir, DELETE para quitar, capítulo por parámetros', async () => {
  const calls = [];
  global.XMLHttpRequest = class {
    open(m, u) { this.m = m; this.u = u; }
    setRequestHeader() {}
    send() { calls.push([this.m, this.u]); this.status = 204; this.onload(); }
  };
  api.base = 'http://192.168.1.13:8787';
  await api.setList('favorites', 't 1', true);
  await api.setList('watchlist', 'bear', false, { season: 1, episode: 3 });
  await api.setList('watchlist', 'bear', true, { season: 0, episode: 2 });
  assert.deepEqual(calls, [
    ['PUT', 'http://192.168.1.13:8787/api/v1/lists/favorites/t%201'],
    ['DELETE', 'http://192.168.1.13:8787/api/v1/lists/watchlist/bear?season=1&episode=3'],
    ['PUT', 'http://192.168.1.13:8787/api/v1/lists/watchlist/bear?season=0&episode=2'],
  ]);
  delete global.XMLHttpRequest;
});

test('capítulos: ▲ desde la foto va a sus propios botones, no a los del vecino', () => {
  // Dos capítulos de 400 px separados 24 px; ♥ y + (52 px) arriba a la derecha de cada foto.
  const ep1 = R(90, 723, 400, 320), ep2 = R(514, 723, 400, 320);
  const fav1 = R(358, 733, 52, 52), wl1 = R(418, 733, 52, 52), fav2 = R(782, 733, 52, 52), wl2 = R(842, 733, 52, 52);
  const pill = R(240, 640, 180, 44); // una píldora de la ficha, más arriba
  const cands = [ep1, ep2, fav1, wl1, fav2, wl2, pill];
  assert.equal(nav.pick(ep2, cands, 'up'), 4, '▲ desde 1×02: su ♥');
  assert.equal(nav.pick(fav1, cands, 'right'), 3, '▶ de ♥ a +');
  assert.equal(nav.pick(fav1, cands, 'up'), 6, '▲ desde ♥: la fila de arriba');
});
