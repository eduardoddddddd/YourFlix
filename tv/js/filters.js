/* Filtros de las listas, enlaces de la ficha y cálculos del kiosco. Todo puro (sin DOM), para
 * poder probarlo en Node. Un filtro es {key, value, name}: key es el parámetro de /titles,
 * value lo que se envía y name lo que se muestra («Años 80» para decade=1980). */
(function (YF) {
  'use strict';

  var F = YF.filters = {};

  // Categorías de «Filtrar por» y el orden de las píldoras activas.
  F.KEYS = ['genre', 'director', 'actor', 'person', 'country', 'decade', 'year', 'runtime', 'rating', 'quality', 'audio', 'subs', 'state'];

  // Grupos del panel «Filtrar por»: cada uno pide a /facets sus categorías (only=…).
  F.GROUPS = [
    { id: 'genre', label: 'Género', keys: ['genre'] },
    { id: 'director', label: 'Director', keys: ['director'], letters: true },
    { id: 'actor', label: 'Actor', keys: ['actor'], letters: true, all: true },
    { id: 'when', label: 'Año/Década', keys: ['decade', 'year'] },
    { id: 'country', label: 'País', keys: ['country'], letters: true },
    { id: 'more', label: 'Más', keys: ['runtime', 'rating', 'quality', 'audio', 'subs', 'state'] }
  ];

  F.HEADS = { genre: 'Género', director: 'Dirección', actor: 'Reparto', person: 'Persona', country: 'País', decade: 'Década', year: 'Año',
    runtime: 'Duración', rating: 'Nota', quality: 'Calidad', audio: 'Audio', subs: 'Subtítulos', state: 'Estado' };

  F.SORTS = [['az', 'A → Z'], ['added', 'Añadidas'], ['year', 'Por año'], ['rating', 'Valoración'], ['runtime', 'Duración']];

  // add: un valor por categoría (el nuevo sustituye al de la misma clave); categorías distintas se suman (Y).
  F.add = function (list, f) {
    var out = (list || []).filter(function (x) { return x.key !== f.key; });
    out.push({ key: f.key, value: String(f.value), name: f.name || String(f.value) });
    out.sort(function (a, b) { return F.KEYS.indexOf(a.key) - F.KEYS.indexOf(b.key); });
    return out;
  };

  F.remove = function (list, key) { return (list || []).filter(function (x) { return x.key !== key; }); };

  F.has = function (list, key, value) {
    return (list || []).some(function (x) { return x.key === key && (value == null || x.value === String(value)); });
  };

  // params: objeto para api.titles / api.facets. skip: claves que no se envían (el panel de esa categoría).
  F.params = function (list, base, skip) {
    var q = {};
    Object.keys(base || {}).forEach(function (k) { q[k] = base[k]; });
    (list || []).forEach(function (x) {
      if (skip && skip.indexOf(x.key) >= 0) return;
      if (q[x.key] == null) q[x.key] = x.value;
      else q[x.key] = [].concat(q[x.key], x.value);
    });
    return q;
  };

  // fromQuery: lo contrario de params (para leer filtros de una dirección o de un objeto).
  F.fromQuery = function (q) {
    var list = [];
    F.KEYS.forEach(function (k) { if (q && q[k] != null && q[k] !== '') [].concat(q[k]).forEach(function (v) { list.push({ key: k, value: String(v), name: String(v) }); }); });
    return list;
  };

  // «Películas · Ridley Scott · Años 80 · 3 títulos»
  F.title = function (kind, list, total) {
    var bits = [kind === 'series' ? 'Series' : 'Películas'];
    (list || []).forEach(function (x) { bits.push(x.name); });
    if (total != null) bits.push(total + (total === 1 ? ' título' : ' títulos'));
    return bits.join(' · ');
  };

  // Texto de una píldora con su número entre paréntesis solo si hay 2 o más títulos.
  F.pill = function (name, count) { return count >= 2 ? name + ' (' + count + ')' : name; };

  // linkRows: de los «links» de la ficha a filas de píldoras [{label, items:[{key, value, name, text}]}].
  F.linkRows = function (links, kind) {
    if (!links) return [];
    var rows = [];
    var item = function (key, l, value) { return { key: key, value: String(value != null ? value : l.name), name: l.name, text: F.pill(l.name, l.count) }; };
    var list = function (key, arr) { return (arr || []).filter(function (l) { return l && l.name; }).map(function (l) { return item(key, l); }); };
    var d = list('director', links.director);
    if (d.length) rows.push({ label: kind === 'series' ? 'Creada por' : 'Dirección', items: d });
    var c = list('actor', links.cast);
    if (c.length) rows.push({ label: 'Reparto', items: c });
    var m = list('country', links.country);
    if (links.year && links.year.name) m.push(item('year', links.year, links.year.name));
    if (links.decade && links.decade.value) m.push(item('decade', links.decade, links.decade.value));
    m = m.concat(list('genre', links.genres));
    if (links.runtime && links.runtime.value) m.push(item('runtime', links.runtime, links.runtime.value));
    if (m.length) rows.push({ label: 'Más', items: m });
    return rows;
  };

  // Lista del panel a partir de /facets: [{head}] y [{key, value, name, count}], por grupos.
  // Las listas largas (personas, países) van por orden alfabético, para poder saltar por letra.
  F.pickerItems = function (group, facets) {
    var out = [];
    group.keys.forEach(function (k) {
      var vals = ((facets || {})[k] || []).map(function (v) { return { key: k, value: String(v.value), name: v.name || String(v.value), count: v.count }; });
      if (group.letters) vals.sort(function (a, b) { return F.fold(a.name) < F.fold(b.name) ? -1 : F.fold(a.name) > F.fold(b.name) ? 1 : 0; });
      if (!vals.length) return;
      if (group.keys.length > 1) out.push({ head: F.HEADS[k] });
      out = out.concat(vals);
    });
    return out;
  };

  // fold: mayúsculas sin acentos («Álex» → «ALEX»), para ordenar y saltar por letra.
  var ACC = { 'Á': 'A', 'À': 'A', 'Â': 'A', 'Ä': 'A', 'Ã': 'A', 'Å': 'A', 'É': 'E', 'È': 'E', 'Ê': 'E', 'Ë': 'E', 'Í': 'I', 'Ì': 'I', 'Î': 'I', 'Ï': 'I',
    'Ó': 'O', 'Ò': 'O', 'Ô': 'O', 'Ö': 'O', 'Õ': 'O', 'Ø': 'O', 'Ú': 'U', 'Ù': 'U', 'Û': 'U', 'Ü': 'U', 'Ç': 'C', 'Ý': 'Y', 'Ł': 'L', 'Š': 'S', 'Ž': 'Z' };
  F.fold = function (s) { return String(s || '').toUpperCase().replace(/[^\x00-\x7FÑ]/g, function (c) { return ACC[c] || c; }); };

  // letter: inicial para el salto («#» si no empieza por letra).
  F.letter = function (name) { var c = F.fold(name).charAt(0); return /[A-ZÑ]/.test(c) ? c : '#'; };

  // letterJump: índice del primer elemento de la letra siguiente (dir 1) o anterior (dir -1).
  // Si en dir -1 no se está en el primero de la letra actual, va a ese primero. Se salta las
  // entradas sin nombre (cabeceras, interruptores). Sin más letras, se queda donde está.
  F.letterJump = function (names, idx, dir) {
    var L = function (i) { return names[i] == null ? null : F.letter(names[i]); };
    var cur = L(idx), i;
    if (dir > 0) {
      for (i = idx + 1; i < names.length; i++) if (L(i) != null && L(i) !== cur) return i;
      return idx;
    }
    var first = idx;
    for (i = idx - 1; i >= 0; i--) { if (L(i) == null) continue; if (L(i) !== cur) break; first = i; }
    if (first !== idx && cur != null) return first;
    // Primero de la letra anterior.
    var prev = null, at = idx;
    for (i = idx - 1; i >= 0; i--) {
      if (L(i) == null) continue;
      if (prev == null) { if (L(i) === cur) continue; prev = L(i); }
      if (L(i) !== prev) break;
      at = i;
    }
    return at;
  };

  // circ: índice circular del kiosco (−1 → último).
  F.circ = function (i, n) { return n > 0 ? ((i % n) + n) % n : 0; };

  // Semilla del orden aleatorio: fija durante la sesión (la anterior es siempre la misma).
  F.seed = function () { return 1 + Math.floor(Math.random() * 99999); };

  // Imagen de fondo a 1920 px (las listas la dan a 1280).
  F.big = function (url) { return url ? String(url).replace(/([?&]w=)\d+/, '$11920') : ''; };

  // Insignias del kiosco: 4K, HDR y VOSE (sin audio en castellano pero con subtítulos en castellano).
  F.kioskBadges = function (f) {
    if (!f) return [];
    var out = [], v = f.video || {}, isEs = function (l) { l = (l || '').toLowerCase(); return l === 'spa' || l === 'es' || l === 'esp'; };
    if (v.width >= 3800) out.push('4K');
    if (v.hdr) out.push('HDR');
    var esAudio = (f.audio || []).some(function (a) { return isEs(a.language); });
    var esSubs = (f.subtitles || []).some(function (s) { return s.text && isEs(s.language); });
    if (!esAudio && esSubs) out.push('VOSE');
    return out;
  };

  // Ciclo de un valor en una lista para los filtros del kiosco ('' = todos).
  F.cycle = function (values, cur) {
    var all = [''].concat(values), i = all.indexOf(cur == null ? '' : cur);
    return all[(i + 1) % all.length];
  };
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
