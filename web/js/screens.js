/* Pantallas: Inicio, Películas, Series (con «Filtrar por»), Búsqueda y Ficha (con enlaces). El Kiosco, en kiosk.js. Cada una pinta en un contenedor y
 * devuelve una promesa. Los datos van siempre por textContent. */
(function (YF) {
  'use strict';
  var el = YF.el, api = YF.api, P = YF.playback, S = YF.screens = {};

  // ---------- Iconos: SVG creados nodo a nodo ----------
  var ICONS = {
    play: 'M8 5v14l11-7z',
    pause: 'M6 19h4V5H6v14zm8-14v14h4V5h-4z',
    back: 'M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z',
    replay: 'M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z',
    forward: 'M12 5V1l5 5-5 5V7c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6h2c0 4.42-3.58 8-8 8s-8-3.58-8-8 3.58-8 8-8z',
    full: 'M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z',
    unfull: 'M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-11V5h-2v5h5V8h-3z',
    vol: 'M3 9v6h4l5 5V4L7 9H3zm13.5 3A4.5 4.5 0 0 0 14 7.97v8.05A4.5 4.5 0 0 0 16.5 12zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z',
    mute: 'M3 9v6h4l5 5V4L7 9H3zm18.7 3.3-1.4-1.4L18 13.2l-2.3-2.3-1.4 1.4 2.3 2.3-2.3 2.3 1.4 1.4 2.3-2.3 2.3 2.3 1.4-1.4-2.3-2.3z',
    subs: 'M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zM4 12h4v2H4v-2zm10 6H4v-2h10v2zm6 0h-4v-2h4v2zm0-4H10v-2h10v2z',
    audio: 'M12 3a9 9 0 0 0-9 9v7c0 1.1.9 2 2 2h4v-8H5v-1a7 7 0 0 1 14 0v1h-4v8h4c1.1 0 2-.9 2-2v-7a9 9 0 0 0-9-9z',
    next: 'M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z',
    left: 'M15.41 7.41 14 6l-6 6 6 6 1.41-1.41L10.83 12z',
    right: 'M10 6 8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z',
    kiosk: 'M4 6h16v10H4V6zm-2-2v14h20V4H2zm8 4.5v5l4-2.5-4-2.5zM7 20h10v2H7z',
    search: 'M15.5 14h-.79l-.28-.27A6.47 6.47 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z',
    restart: 'M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z'
  };
  var NS = 'http://www.w3.org/2000/svg';
  YF.icon = function (name) {
    var svg = document.createElementNS(NS, 'svg'), path = document.createElementNS(NS, 'path');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('aria-hidden', 'true');
    path.setAttribute('d', ICONS[name] || '');
    svg.appendChild(path);
    return svg;
  };

  YF.toast = function (msg, ms) {
    var t = document.getElementById('toast');
    t.textContent = msg;
    t.classList.remove('hidden');
    clearTimeout(YF.toast.timer);
    YF.toast.timer = setTimeout(function () { t.classList.add('hidden'); }, ms || 3500);
  };

  function img(src, cls, alt) {
    var i = el('img', cls);
    i.alt = alt || '';
    i.loading = 'lazy';
    i.decoding = 'async';
    if (src) i.src = api.url(src);
    i.onerror = function () { i.style.visibility = 'hidden'; };
    return i;
  }

  function link(href, cls, label) {
    var a = el('a', cls);
    a.href = href;
    if (label) a.setAttribute('aria-label', label);
    return a;
  }

  function button(label, cls, iconName, onClick) {
    var b = el('button', 'btn' + (cls ? ' ' + cls : ''));
    b.type = 'button';
    if (iconName) b.appendChild(YF.icon(iconName));
    b.appendChild(document.createTextNode(label));
    if (onClick) b.addEventListener('click', onClick);
    return b;
  }

  function linkButton(label, href, cls, iconName) {
    var a = link(href, 'btn' + (cls ? ' ' + cls : ''));
    if (iconName) a.appendChild(YF.icon(iconName));
    a.appendChild(document.createTextNode(label));
    return a;
  }

  function metaLine(t, meta) {
    var bits = [];
    if (t.year) bits.push(t.year);
    if (meta && meta.runtime) bits.push(YF.runtime(meta.runtime));
    var g = (meta && meta.genres) || t.genres || [];
    if (g.length) bits.push(g.slice(0, 3).join(', '));
    var line = el('div', 'meta-line', bits.join(' · '));
    var r = (meta && meta.rating) || t.rating;
    if (r) { line.appendChild(document.createTextNode(bits.length ? '  ·  ' : '')); line.appendChild(el('span', 'star', '★ ' + Number(r).toFixed(1).replace('.', ','))); }
    return line;
  }

  function errorBox(box, msg, retry) {
    YF.clear(box);
    var d = el('div', 'error-box');
    d.appendChild(el('p', '', msg));
    if (retry) d.appendChild(button('Reintentar', 'primary', null, retry));
    box.appendChild(d);
  }

  function loading(box, msg) {
    YF.clear(box);
    var d = el('div', 'loading');
    d.appendChild(el('div', 'spinner'));
    d.appendChild(el('div', '', msg || 'Cargando…'));
    box.appendChild(d);
  }
  S.loading = loading;

  // ---------- Tarjetas ----------
  function posterCard(t) {
    var a = link('#/titulo/' + encodeURIComponent(t.id), 'card', t.title + (t.year ? ' (' + t.year + ')' : ''));
    a.dataset.id = t.id;
    var art = el('div', 'art');
    if (t.poster) {
      var i = img(t.poster, '', '');
      i.srcset = api.url(t.poster) + ' 300w, ' + api.url(YF.imgWidth(t.poster, 600)) + ' 600w';
      i.sizes = '(max-width: 720px) 130px, 200px';
      art.appendChild(i);
    } else art.appendChild(el('div', 'ph', t.title));
    if (t.matched === false) art.appendChild(el('span', 'badge-pending', 'sin ficha'));
    a.appendChild(art);
    a.appendChild(el('div', 'cap', t.title + (t.year ? ' (' + t.year + ')' : '')));
    return a;
  }

  function continueLabel(c) {
    return c.title + (c.season != null ? ' · ' + YF.epLabel(c) + (c.episodeName ? ' ' + c.episodeName : '') : '');
  }

  function continueCard(c) {
    var a = link('#/ver/' + encodeURIComponent(c.titleId) + '/' + encodeURIComponent(c.fileId), 'card wide', 'Seguir viendo ' + continueLabel(c));
    var art = el('div', 'art');
    art.appendChild(img(c.still || c.backdrop || c.poster, '', ''));
    var bar = el('div', 'pbar');
    bar.style.width = Math.min(100, c.position / (c.duration || 1) * 100) + '%';
    art.appendChild(bar);
    var pb = el('span', 'play-badge');
    pb.appendChild(YF.icon('play'));
    art.appendChild(pb);
    a.appendChild(art);
    a.appendChild(el('div', 'cap', (c.next ? 'Siguiente: ' : '') + continueLabel(c) + (c.next ? '' : ' · quedan ' + YF.clock(c.duration - c.position))));
    return a;
  }

  function row(title, cards) {
    var r = el('section', 'row');
    r.appendChild(el('h2', '', title));
    var track = el('div', 'track');
    cards.forEach(function (c) { track.appendChild(c); });
    r.appendChild(track);
    ['prev', 'next'].forEach(function (dir) {
      var b = el('button', 'arrow ' + dir);
      b.type = 'button';
      b.setAttribute('aria-label', dir === 'prev' ? 'Anteriores' : 'Siguientes');
      b.tabIndex = -1;
      b.appendChild(YF.icon(dir === 'prev' ? 'left' : 'right'));
      b.addEventListener('click', function () { track.scrollBy({ left: (dir === 'prev' ? -1 : 1) * track.clientWidth * 0.85, behavior: 'smooth' }); });
      r.appendChild(b);
    });
    return r;
  }

  // ---------- Inicio ----------
  S.home = function (box) {
    loading(box, 'Cargando la colección…');
    return Promise.all([
      api.get('/continue').catch(function () { return []; }),
      api.titles({ sort: 'added', limit: 24 }),
      api.titles({ sort: 'rating', kind: 'movie', limit: 24 }),
      api.titles({ kind: 'series', sort: 'rating', limit: 24 }),
      api.get('/genres').catch(function () { return []; })
    ]).then(function (r) {
      var cont = r[0] || [], added = r[1].items, rated = r[2].items, series = r[3].items, genres = r[4] || [];
      YF.clear(box);
      if (!added.length) { box.appendChild(el('div', 'empty', 'La colección está vacía. Añade carpetas en el servidor y escanéalas.')); return; }
      box.appendChild(hero(cont[0], rated[0] || added[0]));
      var rows = el('div', 'rows');
      box.appendChild(rows);
      if (cont.length) rows.appendChild(row('Seguir viendo', cont.map(continueCard)));
      rows.appendChild(row('Añadidas recientemente', added.map(posterCard)));
      if (rated.length) rows.appendChild(row('Películas mejor valoradas', rated.map(posterCard)));
      if (series.length) rows.appendChild(row('Series', series.map(posterCard)));
      var top = genres.filter(function (g) { return g.count >= 4; }).slice(0, 5);
      return Promise.all(top.map(function (g) { return api.titles({ genre: g.name, sort: 'rating', limit: 24 }).catch(function () { return { items: [] }; }); })).then(function (lists) {
        lists.forEach(function (l, i) { if (l.items.length) rows.appendChild(row(top[i].name, l.items.map(posterCard))); });
      });
    }).catch(function (e) { errorBox(box, 'No se pudo cargar la colección (' + e.message + ').', function () { S.home(box); }); });
  };

  // El héroe: lo último a medias o la película mejor valorada.
  function hero(c, t) {
    var h = el('section', 'hero no-img');
    var bg = el('img', 'hero-img');
    bg.alt = '';
    var info = el('div', 'hero-info');
    h.appendChild(bg); h.appendChild(el('div', 'hero-shade')); h.appendChild(info);
    var id = c ? c.titleId : t.id;
    var title = c ? c.title : t.title;
    info.appendChild(el('div', 'eyebrow', c ? (c.next ? 'Siguiente capítulo' : 'Seguir viendo') : (t.kind === 'series' ? 'Serie' : 'Película')));
    info.appendChild(el('h1', '', title));
    var metaBox = el('div');
    info.appendChild(metaBox);
    var syn = el('div', 'synopsis');
    info.appendChild(syn);
    var btns = el('div', 'btns');
    if (c) btns.appendChild(linkButton(c.next || c.position < 60 ? 'Ver ' + (c.season != null ? YF.epLabel(c) : '') : 'Seguir · quedan ' + YF.clock(c.duration - c.position),
      '#/ver/' + encodeURIComponent(c.titleId) + '/' + encodeURIComponent(c.fileId), 'primary', 'play'));
    btns.appendChild(linkButton('Ficha', '#/titulo/' + encodeURIComponent(id), c ? '' : 'primary'));
    info.appendChild(btns);
    var src = (c && c.backdrop) || (t && t.backdrop);
    var shown = function () { bg.classList.add('shown'); h.classList.remove('no-img'); };
    if (src) { bg.onload = shown; bg.src = api.url(src); }
    api.title(id).then(function (d) {
      var meta = d.title.meta || {};
      metaBox.appendChild(metaLine(d.title, meta));
      syn.textContent = meta.synopsis || '';
      if (!src && d.backdrop) { bg.onload = shown; bg.src = api.url(d.backdrop); }
    }).catch(function () {});
    return h;
  }

  // ---------- Películas / Series ----------
  var SORTS = [['az', 'A → Z'], ['added', 'Añadidas'], ['year', 'Por año'], ['rating', 'Valoración'], ['runtime', 'Duración']];
  // «Filtrar por»: cada botón abre un desplegable con uno o varios grupos [categoría de /facets, clave en la dirección, título].
  var CATS = [
    { label: 'Género', groups: [['genre', 'genero', '']] },
    { label: 'Director', groups: [['director', 'director', '']] },
    { label: 'Actor', groups: [['actor', 'actor', '']], all: true },
    { label: 'Año/Década', groups: [['decade', 'decada', 'Décadas'], ['year', 'anio', 'Años']] },
    { label: 'País', groups: [['country', 'pais', '']] },
    { label: 'Más', groups: [['runtime', 'duracion', 'Duración'], ['rating', 'nota', 'Nota'], ['quality', 'calidad', 'Calidad'],
      ['audio', 'audio', 'Audio'], ['subs', 'subs', 'Subtítulos'], ['state', 'estado', 'Estado']] }
  ];
  var MAX_OPTS = 300;
  // Tema 1: por dirección, cuántos títulos había cargados y qué tarjeta se abrió (para volver a ella).
  var gridMemo = {};

  S.grid = function (box, kind, query, lists) {
    var routeName = kind === 'series' ? 'series' : 'peliculas', label = kind === 'series' ? 'Series' : 'Películas';
    var filters = YF.readFilters(lists || {});
    var sort = query.orden || 'az';
    var key = location.hash || '#/' + routeName, back = gridMemo[key];
    var q = YF.filterQuery(filters, true, { kind: kind, sort: sort, offset: 0, limit: 60 });
    var facets = null, allActors = null, drop = null;
    function href(fs, s) { return YF.route([routeName], YF.filterQuery(fs, false, { orden: (s || sort) === 'az' ? '' : (s || sort) })); }
    function go(fs) { closeDrop(); location.hash = href(fs); } // cada filtro, una entrada del historial: Atrás lo quita

    YF.clear(box);
    var head = el('div', 'page-head');
    var h1 = el('h1', '', YF.listHeading(label, filters));
    var count = el('span', 'count');
    h1.appendChild(count);
    head.appendChild(h1);

    var by = el('div', 'filter-by');
    by.appendChild(el('span', 'lbl', 'Filtrar por'));
    var byBtns = el('div', 'chips');
    CATS.forEach(function (c) {
      var n = filters.filter(function (f) { return c.groups.some(function (g) { return g[1] === f.k; }); }).length;
      var b = el('button', 'chip drop' + (n ? ' on' : ''), c.label + (n ? ' · ' + n : ''));
      b.type = 'button';
      b.setAttribute('aria-haspopup', 'true');
      b.setAttribute('aria-expanded', 'false');
      b.addEventListener('click', function (e) { e.stopPropagation(); if (drop && drop.btn === b) closeDrop(); else openDrop(c, b); });
      byBtns.appendChild(b);
    });
    by.appendChild(byBtns);
    head.appendChild(by);

    if (filters.length) {
      var act = el('div', 'active-filters');
      act.setAttribute('aria-label', 'Filtros activos');
      filters.forEach(function (f) {
        var t = YF.filterLabel(f.k, f.v);
        var p = el('button', 'pill', t);
        p.type = 'button';
        p.setAttribute('aria-label', 'Quitar el filtro ' + t);
        p.appendChild(el('span', 'x', '✕'));
        p.addEventListener('click', function () { go(YF.withoutFilter(filters, f.k, f.v)); });
        act.appendChild(p);
      });
      if (filters.length > 1) { var clr = el('button', 'pill clear', 'Quitar todos'); clr.type = 'button'; clr.addEventListener('click', function () { go([]); }); act.appendChild(clr); }
      act.appendChild(linkButton('Ver en kiosco', YF.route(['kiosco'], YF.filterQuery(filters, false, { tipo: routeName })), 'kiosk-btn', 'kiosk'));
      head.appendChild(act);
    }

    var chips = el('div', 'chips sorts');
    chips.setAttribute('role', 'group');
    chips.setAttribute('aria-label', 'Orden');
    SORTS.forEach(function (o) {
      var c = el('button', 'chip' + (sort === o[0] ? ' on' : ''), o[1]);
      c.type = 'button';
      c.setAttribute('aria-pressed', sort === o[0] ? 'true' : 'false');
      c.addEventListener('click', function () { location.replace(href(filters, o[0])); });
      chips.appendChild(c);
    });
    head.appendChild(chips);
    box.appendChild(head);
    var grid = el('div', 'grid');
    box.appendChild(grid);
    var sentinel = el('div', 'sentinel');
    box.appendChild(sentinel);
    grid.addEventListener('click', function (e) {
      var card = e.target.closest && e.target.closest('.card');
      if (card) gridMemo[key] = { n: q.offset, id: card.dataset.id };
    });

    var facetQ = YF.filterQuery(filters, true, { kind: kind });
    var facetsP = api.facets(facetQ).then(function (r) { facets = r.facets || {}; }).catch(function () { facets = {}; });

    // ---- Desplegable con búsqueda ----
    function closeDrop() {
      if (!drop) return;
      drop.btn.setAttribute('aria-expanded', 'false');
      if (drop.panel.parentNode) drop.panel.parentNode.removeChild(drop.panel);
      drop = null;
    }
    function openDrop(cat, btn) {
      closeDrop();
      var panel = el('div', 'fdrop');
      panel.setAttribute('role', 'dialog');
      panel.setAttribute('aria-label', 'Filtrar por ' + cat.label);
      var top = el('div', 'fdrop-top');
      top.appendChild(el('h3', '', cat.label));
      var allBtn = null;
      if (cat.all) {
        allBtn = el('button', 'chip small', 'Todos');
        allBtn.type = 'button';
        allBtn.setAttribute('aria-pressed', allActors ? 'true' : 'false');
        if (allActors) allBtn.classList.add('on');
        allBtn.title = 'Ver también los actores con un solo título';
        top.appendChild(allBtn);
      }
      var x = el('button', 'fdrop-close', '✕');
      x.type = 'button';
      x.setAttribute('aria-label', 'Cerrar');
      x.addEventListener('click', closeDrop);
      top.appendChild(x);
      panel.appendChild(top);
      var input = el('input', 'fdrop-search');
      input.type = 'search';
      input.placeholder = 'Buscar…';
      input.autocomplete = 'off';
      input.setAttribute('aria-label', 'Buscar en ' + cat.label);
      panel.appendChild(input);
      var list = el('div', 'fdrop-list');
      panel.appendChild(list);
      by.appendChild(panel);
      btn.setAttribute('aria-expanded', 'true');
      drop = { btn: btn, panel: panel };
      panel.addEventListener('click', function (e) { e.stopPropagation(); });

      function values(g) {
        var src = g[0] === 'actor' && allActors ? allActors : facets;
        return (src && src[g[0]]) || [];
      }
      function draw() {
        YF.clear(list);
        if (!facets) { list.appendChild(el('p', 'fdrop-msg', 'Cargando…')); return; }
        var text = YF.fold(input.value.trim()), shown = 0, hidden = 0;
        cat.groups.forEach(function (g) {
          var vs = values(g).filter(function (v) { return !text || YF.fold(v.name || v.value).indexOf(text) >= 0; });
          if (!vs.length) return;
          if (cat.groups.length > 1) list.appendChild(el('div', 'fdrop-group', g[2]));
          vs.forEach(function (v) {
            if (shown >= MAX_OPTS) { hidden++; return; }
            shown++;
            var on = YF.hasFilter(filters, g[1], v.value);
            var o = el('button', 'fopt' + (on ? ' on' : ''));
            o.type = 'button';
            o.setAttribute('aria-pressed', on ? 'true' : 'false');
            o.appendChild(el('span', 'ck', on ? '✓' : ''));
            o.appendChild(el('span', 'nm', v.name || v.value));
            o.appendChild(el('span', 'n', String(v.count)));
            o.addEventListener('click', function () { go(on ? YF.withoutFilter(filters, g[1], v.value) : YF.withFilter(filters, g[1], v.value)); });
            list.appendChild(o);
          });
        });
        if (hidden) list.appendChild(el('p', 'fdrop-msg', 'Y ' + hidden + ' más: escribe para buscar.'));
        if (!shown) list.appendChild(el('p', 'fdrop-msg', text ? 'Nada con «' + input.value.trim() + '».' : 'Nada que elegir con los filtros puestos.'));
      }
      input.addEventListener('input', draw);
      // ▲▼ entre la caja y las opciones; Escape cierra.
      panel.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') { e.preventDefault(); closeDrop(); btn.focus(); return; }
        if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
        var opts = Array.prototype.slice.call(list.querySelectorAll('.fopt'));
        if (!opts.length) return;
        e.preventDefault();
        var i = opts.indexOf(document.activeElement);
        if (e.key === 'ArrowDown') (opts[i + 1] || opts[opts.length - 1]).focus();
        else if (i <= 0) input.focus(); else opts[i - 1].focus();
      });
      if (allBtn) allBtn.addEventListener('click', function () {
        if (allActors) { allActors = null; allBtn.classList.remove('on'); allBtn.setAttribute('aria-pressed', 'false'); draw(); return; }
        allBtn.disabled = true;
        api.facets(YF.filterQuery(filters, true, { kind: kind, only: 'actor', all: 1 })).then(function (r) {
          allActors = r.facets || {};
          allBtn.classList.add('on'); allBtn.setAttribute('aria-pressed', 'true');
        }).catch(function (e) { YF.toast('No se pudo cargar: ' + e.message); }).then(function () { allBtn.disabled = false; draw(); });
      });
      draw();
      if (!facets) facetsP.then(function () { if (drop && drop.panel === panel) draw(); });
      if (window.matchMedia('(hover: hover)').matches) input.focus();
    }
    function outside() { closeDrop(); }
    document.addEventListener('click', outside);

    // ---- Rejilla con carga por páginas ----
    var total = 1, busy = false;
    function more(limit) {
      if (busy || q.offset >= total) return Promise.resolve();
      busy = true;
      q.limit = limit || 60;
      return api.titles(q).then(function (r) {
        total = r.total;
        count.textContent = r.total + (r.total === 1 ? ' título' : ' títulos');
        document.title = YF.listHeading(label, filters) + ' · YourFlix';
        r.items.forEach(function (t) { grid.appendChild(posterCard(t)); });
        q.offset += r.items.length;
        busy = false;
        if (!r.items.length) total = q.offset;
        if (!grid.children.length) grid.appendChild(el('div', 'empty', 'Nada por aquí con estos filtros.'));
      }).catch(function (e) { busy = false; total = q.offset; YF.toast('No se pudo cargar: ' + e.message); });
    }
    // Al volver de una ficha: tantos títulos como había (de 500 en 500) y la vista en la tarjeta de la que se salió.
    function fill(target) {
      return more(Math.min(500, Math.max(60, target - q.offset))).then(function () {
        if (q.offset < target && q.offset < total) return fill(target);
      });
    }
    var io = new IntersectionObserver(function (en) { if (en[0].isIntersecting) more(); }, { rootMargin: '900px 0px' });
    S.cleanup = function () { io.disconnect(); document.removeEventListener('click', outside); closeDrop(); };
    return fill(back ? back.n : 60).then(function () {
      io.observe(sentinel);
      if (!back || !back.id) return false;
      var card = null;
      Array.prototype.some.call(grid.children, function (c) { if (c.dataset && c.dataset.id === back.id) { card = c; return true; } return false; });
      if (!card) return false;
      card.scrollIntoView({ block: 'center' });
      try { card.focus({ preventScroll: true }); } catch (_) { card.focus(); }
      return true; // la posición ya está puesta: app.js no restaura el desplazamiento en píxeles
    });
  };

  // ---------- Buscar ----------
  S.search = function (box, query) {
    YF.clear(box);
    var head = el('div', 'page-head');
    head.appendChild(el('h1', '', 'Buscar'));
    var sb = el('div', 'search-box');
    sb.appendChild(YF.icon('search'));
    var input = el('input');
    input.type = 'search';
    input.placeholder = 'Título, director o reparto';
    input.setAttribute('aria-label', 'Buscar por título, director o reparto');
    input.autocomplete = 'off';
    input.enterKeyHint = 'search';
    input.value = query.q || '';
    sb.appendChild(input);
    head.appendChild(sb);
    var hint = el('p', 'hint', 'Escribe al menos dos letras.');
    head.appendChild(hint);
    box.appendChild(head);
    var grid = el('div', 'grid');
    box.appendChild(grid);
    var seq = 0;
    function run() {
      var text = input.value.trim(), my = ++seq;
      history.replaceState(null, '', YF.route(['buscar'], { q: text }));
      if (text.length < 2) { YF.clear(grid); hint.textContent = 'Escribe al menos dos letras.'; return; }
      hint.textContent = 'Buscando…';
      api.titles({ q: text, limit: 60 }).then(function (r) {
        if (my !== seq) return;
        YF.clear(grid);
        hint.textContent = r.total ? r.total + (r.total === 1 ? ' resultado' : ' resultados') : 'Sin resultados para «' + text + '».';
        r.items.forEach(function (t) { grid.appendChild(posterCard(t)); });
      }).catch(function (e) { if (my === seq) hint.textContent = 'Error al buscar: ' + e.message; });
    }
    input.addEventListener('input', YF.debounce(run, 300));
    input.addEventListener('keydown', function (e) { if (e.key === 'Enter') { run(); input.blur(); } });
    if (input.value) run();
    setTimeout(function () { if (window.matchMedia('(hover: hover)').matches || !input.value) input.focus(); }, 50);
    return Promise.resolve();
  };

  // ---------- Ficha ----------
  function fileLabel(f) {
    var b = YF.techBadges(f).slice(0, 3);
    b.push(YF.bytes(f.size));
    return b.join(' · ');
  }

  function howLine(f, caps) {
    var audio = P.pickAudio(f, YF.storage.get('audioLang', ''));
    var d = P.decide(f, audio, caps);
    return el('p', 'how ' + (d.method === 'direct' ? 'direct' : d.method === 'none' ? 'none' : ''), d.why);
  }

  function techBlock(box, f, caps) {
    var b = el('div', 'badges');
    YF.techBadges(f).forEach(function (x, i) { b.appendChild(el('span', 'badge' + (i === 0 ? ' gold' : ''), x)); });
    b.appendChild(el('span', 'badge', YF.bytes(f.size)));
    box.appendChild(b);
    box.appendChild(howLine(f, caps));
    var imgSubs = (f.subtitles || []).filter(function (s) { return !s.text; }).length;
    if (imgSubs) box.appendChild(el('p', 'how none', imgSubs + (imgSubs === 1 ? ' pista de subtítulos en imagen (PGS/DVD) no se puede mostrar.' : ' pistas de subtítulos en imagen (PGS/DVD) no se pueden mostrar.')));
  }

  function playHref(t, f, start) {
    return YF.route(['ver', t.id, f.id], start != null ? { t: Math.floor(start) } : null);
  }

  S.detail = function (box, id, query) {
    loading(box);
    return Promise.all([api.title(id), YF.app.caps()]).then(function (r) {
      var d = r[0], caps = r[1], t = d.title, meta = t.meta || {}, files = d.files || [];
      YF.clear(box);
      document.title = t.title + ' · YourFlix';
      var wrap = el('div', 'detail');
      box.appendChild(wrap);
      if (d.backdrop) { var bg = el('div', 'detail-bg'); bg.appendChild(img(d.backdrop, '', '')); wrap.appendChild(bg); }
      var body = el('div', 'detail-body');
      wrap.appendChild(body);
      var info = el('div', 'detail-info');
      body.appendChild(info);
      if (d.poster) { var p = img(d.poster, 'poster-big', 'Cartel de ' + t.title); p.loading = 'eager'; body.appendChild(p); }
      info.appendChild(el('div', 'eyebrow', (t.kind === 'series' ? 'Serie' : 'Película') + (t.matchSource === 'filename' ? ' · ficha pendiente' : '')));
      info.appendChild(el('h1', '', t.title));
      info.appendChild(metaLine(t, meta));
      if (meta.tagline) info.appendChild(el('div', 'tagline', meta.tagline));
      if (meta.synopsis) info.appendChild(el('div', 'synopsis', meta.synopsis));
      var ppl = el('div', 'people');
      if (meta.director && meta.director.length) { ppl.appendChild(document.createTextNode(t.kind === 'series' ? 'Creada por ' : 'Dirección ')); ppl.appendChild(el('b', '', meta.director.join(', '))); }
      if (meta.cast && meta.cast.length) { ppl.appendChild(document.createTextNode((ppl.childNodes.length ? '  ·  ' : '') + 'Reparto ')); ppl.appendChild(el('b', '', meta.cast.slice(0, 6).join(', '))); }
      if (d.links) info.appendChild(linkRows(t, d.links)); // servidor con categorías: enlaces
      else if (ppl.childNodes.length) info.appendChild(ppl);
      if (!files.length) { info.appendChild(el('p', 'how none', 'Ningún archivo de este título está disponible ahora.')); return; }
      if (t.kind === 'series') renderSeries(wrap, info, t, files, caps, query);
      else renderMovie(info, t, files, caps, query);
    }).catch(function (e) { errorBox(box, e.status === 404 ? 'Este título ya no existe.' : 'No se pudo cargar la ficha (' + e.message + ').', e.status === 404 ? null : function () { S.detail(box, id, query); }); });
  };

  // Filas de enlaces de la ficha: cada dato lleva a la lista filtrada. El número, solo si lo comparten 2 o más.
  function linkRows(t, links) {
    var route = t.kind === 'series' ? 'series' : 'peliculas', box = el('div', 'links');
    function a(k, value, name, n) {
      var q = {};
      q[k] = value;
      var x = link(YF.route([route], q), 'tag');
      x.appendChild(document.createTextNode(name));
      if (n >= 2) x.appendChild(el('span', 'n', ' (' + n + ')'));
      return x;
    }
    function list(k, items) { return (items || []).filter(function (x) { return x && x.name; }).map(function (x) { return a(k, x.value || x.name, x.name, x.count); }); }
    function rowOf(lbl, items) {
      if (!items.length) return;
      var r = el('div', 'link-row');
      r.appendChild(el('span', 'lbl', lbl));
      var v = el('div', 'vals');
      items.forEach(function (x) { v.appendChild(x); });
      r.appendChild(v);
      box.appendChild(r);
    }
    rowOf(t.kind === 'series' ? 'Creada por' : 'Dirección', list('director', links.director));
    rowOf('Reparto', list('actor', links.cast));
    rowOf('País', list('pais', links.country));
    rowOf('Año', list('anio', [links.year]).concat(list('decada', [links.decade])));
    rowOf('Géneros', list('genero', links.genres));
    rowOf('Duración', list('duracion', [links.runtime]));
    return box;
  }

  function renderMovie(info, t, files, caps, query) {
    var idx = Math.min(files.length - 1, Math.max(0, parseInt(query.copia, 10) || 0));
    var f = query.copia != null ? files[idx] : P.chooseFile(files);
    var tech = el('div');
    var btns = el('div', 'btns');
    function draw() {
      YF.clear(tech); YF.clear(btns);
      techBlock(tech, f, caps);
      var at = P.resumeAt(f);
      btns.appendChild(linkButton(at ? 'Reanudar desde ' + YF.clock(at) : 'Reproducir', playHref(t, f, null), 'primary', 'play'));
      if (at) btns.appendChild(linkButton('Desde el principio', playHref(t, f, 0), '', 'restart'));
    }
    info.appendChild(tech);
    info.appendChild(btns);
    if (files.length > 1) {
      var sel = el('select', 'select copy-select');
      sel.setAttribute('aria-label', 'Copia');
      files.forEach(function (x, i) { var o = el('option', '', 'Copia ' + (i + 1) + ': ' + fileLabel(x)); o.value = i; if (x === f) o.selected = true; sel.appendChild(o); });
      sel.addEventListener('change', function () { f = files[sel.value]; draw(); });
      info.appendChild(sel);
    }
    draw();
  }

  function renderSeries(wrap, info, t, files, caps, query) {
    var next = P.chooseFile(files);
    var at = P.resumeAt(next);
    techBlock(info, next, caps);
    var btns = el('div', 'btns');
    btns.appendChild(linkButton((at ? 'Reanudar ' : 'Ver ') + YF.epLabel(next), playHref(t, next, null), 'primary', 'play'));
    info.appendChild(btns);
    var seasons = [];
    files.forEach(function (f) { var n = f.season == null ? 0 : f.season; if (seasons.indexOf(n) < 0) seasons.push(n); });
    seasons.sort(function (a, b) { return a - b; });
    var season = query.temporada != null && seasons.indexOf(+query.temporada) >= 0 ? +query.temporada : (next.season == null ? 0 : next.season);
    var tabs = el('div', 'chips seasons');
    var list = el('div', 'episodes');
    function draw() {
      YF.clear(tabs); YF.clear(list);
      if (seasons.length > 1) seasons.forEach(function (n) {
        var c = el('button', 'chip' + (n === season ? ' on' : ''), n ? 'Temporada ' + n : 'Otros');
        c.type = 'button';
        c.setAttribute('aria-pressed', n === season ? 'true' : 'false');
        c.addEventListener('click', function () { season = n; history.replaceState(null, '', YF.route(['titulo', t.id], { temporada: n })); draw(); });
        tabs.appendChild(c);
      });
      files.filter(function (f) { return (f.season == null ? 0 : f.season) === season; }).forEach(function (f) {
        var ei = f.episodeInfo || {};
        var name = ei.name || f.label || ('Capítulo ' + (f.episode || ''));
        var a = link(playHref(t, f, null), 'episode', 'Ver ' + YF.epLabel(f) + ' ' + name);
        var art = el('div', 'art');
        if (ei.still) art.appendChild(img(ei.still, '', ''));
        if (f.season != null) art.appendChild(el('span', 'num', YF.epLabel(f)));
        if (f.progress && f.progress.watched) art.appendChild(el('span', 'watched', '✓'));
        else if (P.resumeAt(f)) { var b = el('div', 'pbar'); b.style.width = Math.min(100, f.progress.position / (f.duration || 1) * 100) + '%'; art.appendChild(b); }
        a.appendChild(art);
        var txt = el('div', 'txt');
        txt.appendChild(el('div', 'ep-name', name));
        txt.appendChild(el('div', 'ep-desc', ei.overview || YF.clock(f.duration)));
        a.appendChild(txt);
        list.appendChild(a);
      });
    }
    wrap.appendChild(tabs);
    wrap.appendChild(list);
    draw();
  }
  // Para otras pantallas (kiosk.js).
  S.h = { img: img, link: link, button: button, linkButton: linkButton, loading: loading, errorBox: errorBox, playHref: playHref };
})(window.YF = window.YF || {});
