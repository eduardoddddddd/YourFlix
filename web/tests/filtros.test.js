// node --test web/tests/*.test.js — filtros en la dirección e índice circular del kiosco.
const test = require('node:test');
const assert = require('node:assert');
const U = require('../js/util.js');

test('dirección → filtros → dirección', () => {
  const r = U.parseRoute('#/peliculas?director=Ridley%20Scott&decada=1980&actor=A&actor=B&orden=year&nada=1');
  assert.deepEqual(r.lists.actor, ['A', 'B'], 'las claves repetidas se conservan');
  assert.equal(r.query.actor, 'B', 'query sigue dando el último valor');
  const f = U.readFilters(r.lists);
  assert.deepEqual(f, [{ k: 'director', v: 'Ridley Scott' }, { k: 'actor', v: 'A' }, { k: 'actor', v: 'B' }, { k: 'decada', v: '1980' }]);
  assert.deepEqual(U.filterQuery(f, true, { kind: 'movie' }), { kind: 'movie', director: ['Ridley Scott'], actor: ['A', 'B'], decade: ['1980'] });
  const h = U.route(['peliculas'], U.filterQuery(f, false, { orden: 'year' }));
  assert.equal(h, '#/peliculas?orden=year&director=Ridley%20Scott&actor=A&actor=B&decada=1980');
  assert.deepEqual(U.readFilters(U.parseRoute(h).lists), f, 'ida y vuelta');
  assert.deepEqual(U.readFilters(U.parseRoute('#/x?decada=1980&decada=1990&genero=Drama&genero=Drama').lists),
    [{ k: 'genero', v: 'Drama' }, { k: 'decada', v: '1990' }], 'una sola década (la última) y sin repetidos');
  assert.deepEqual(U.readFilters({}), []);
});

test('sumar y quitar filtros', () => {
  let f = U.withFilter([], 'director', 'Ridley Scott');
  f = U.withFilter(f, 'decada', '1980');
  f = U.withFilter(f, 'decada', '1990');
  assert.deepEqual(f, [{ k: 'director', v: 'Ridley Scott' }, { k: 'decada', v: '1990' }], 'la década se sustituye');
  f = U.withFilter(f, 'director', 'Woody Allen');
  assert.equal(f.filter((x) => x.k === 'director').length, 2, 'los directores se suman (Y)');
  assert.equal(U.withFilter(f, 'director', 'Woody Allen').length, 3, 'sin duplicados');
  assert.ok(U.hasFilter(f, 'decada', 1990));
  assert.deepEqual(U.withoutFilter(f, 'director', 'Ridley Scott').map((x) => x.v), ['1990', 'Woody Allen']);
  assert.deepEqual(U.withoutFilter(f, 'director').map((x) => x.v), ['1990']);
  assert.equal(U.filterApiKey('anio'), 'year');
  assert.equal(U.filterKey('country'), 'pais');
});

test('textos de filtros y título de la lista', () => {
  assert.equal(U.filterLabel('decada', '1980'), 'Años 80');
  assert.equal(U.filterLabel('decada', '2010'), 'Años 2010');
  assert.equal(U.filterLabel('duracion', 'mid'), 'De 90 a 120 min');
  assert.equal(U.filterLabel('calidad', '4k'), '4K');
  assert.equal(U.filterLabel('subs', 'spa'), 'Subtítulos español');
  assert.equal(U.filterLabel('estado', 'unwatched'), 'Sin ver');
  assert.equal(U.listHeading('Películas', [{ k: 'director', v: 'Ridley Scott' }], 5), 'Películas · Ridley Scott · 5 títulos');
  assert.equal(U.listHeading('Series', [], 1), 'Series · 1 título');
  assert.equal(U.listHeading('Películas', []), 'Películas');
  assert.ok(U.fold('Martin Scorsese').includes(U.fold('SCOR')));
  assert.ok(U.fold('Pedro Almodóvar').includes(U.fold('almodovar')));
});

test('índice circular del kiosco', () => {
  assert.equal(U.wrapIndex(0, 46), 0);
  assert.equal(U.wrapIndex(-1, 46), 45);
  assert.equal(U.wrapIndex(46, 46), 0);
  assert.equal(U.wrapIndex(47, 46), 1);
  assert.equal(U.wrapIndex(-47, 46), 45);
  assert.equal(U.wrapIndex(3, 0), 0, 'lista vacía');
  assert.equal(U.wrapIndex(5, 1), 0);
});
