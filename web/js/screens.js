/* Pantallas: Inicio, Películas, Series, Búsqueda y Ficha. Cada una pinta en un contenedor y
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
  var SORTS = [['az', 'A → Z'], ['added', 'Añadidas'], ['year', 'Por año'], ['rating', 'Valoración']];
  var genresCache = null;

  S.grid = function (box, kind, query) {
    var routeName = kind === 'series' ? 'series' : 'peliculas';
    var q = { kind: kind, sort: query.orden || 'az', genre: query.genero || '', offset: 0, limit: 60 };
    YF.clear(box);
    var head = el('div', 'page-head');
    head.appendChild(el('h1', '', kind === 'series' ? 'Series' : 'Películas'));
    var filters = el('div', 'filters');
    var chips = el('div', 'chips');
    chips.setAttribute('role', 'group');
    chips.setAttribute('aria-label', 'Orden');
    SORTS.forEach(function (o) {
      var c = el('button', 'chip' + (q.sort === o[0] ? ' on' : ''), o[1]);
      c.type = 'button';
      c.setAttribute('aria-pressed', q.sort === o[0] ? 'true' : 'false');
      c.addEventListener('click', function () { location.replace(YF.route([routeName], { orden: o[0], genero: q.genre })); });
      chips.appendChild(c);
    });
    filters.appendChild(chips);
    var sel = el('select', 'select');
    sel.setAttribute('aria-label', 'Género');
    var all = el('option', '', 'Todos los géneros');
    all.value = '';
    sel.appendChild(all);
    sel.addEventListener('change', function () { location.replace(YF.route([routeName], { orden: q.sort, genero: sel.value })); });
    filters.appendChild(sel);
    var count = el('span', 'count');
    filters.appendChild(count);
    head.appendChild(filters);
    box.appendChild(head);
    var grid = el('div', 'grid');
    box.appendChild(grid);
    var sentinel = el('div', 'sentinel');
    box.appendChild(sentinel);

    (genresCache ? Promise.resolve(genresCache) : api.get('/genres')).then(function (g) {
      genresCache = g || [];
      genresCache.forEach(function (x) {
        var o = el('option', '', x.name);
        o.value = x.name;
        if (x.name === q.genre) o.selected = true;
        sel.appendChild(o);
      });
    }).catch(function () {});

    var total = 1, busy = false;
    function more() {
      if (busy || q.offset >= total) return Promise.resolve();
      busy = true;
      return api.titles(q).then(function (r) {
        total = r.total;
        count.textContent = r.total + (r.total === 1 ? ' título' : ' títulos');
        r.items.forEach(function (t) { grid.appendChild(posterCard(t)); });
        q.offset += r.items.length;
        busy = false;
        if (!r.items.length) total = q.offset;
        if (!grid.children.length) grid.appendChild(el('div', 'empty', 'Nada por aquí.'));
      }).catch(function (e) { busy = false; YF.toast('No se pudo cargar: ' + e.message); });
    }
    var io = new IntersectionObserver(function (en) { if (en[0].isIntersecting) more(); }, { rootMargin: '900px 0px' });
    io.observe(sentinel);
    S.cleanup = function () { io.disconnect(); };
    return more();
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
      if (ppl.childNodes.length) info.appendChild(ppl);
      if (!files.length) { info.appendChild(el('p', 'how none', 'Ningún archivo de este título está disponible ahora.')); return; }
      if (t.kind === 'series') renderSeries(wrap, info, t, files, caps, query);
      else renderMovie(info, t, files, caps, query);
    }).catch(function (e) { errorBox(box, e.status === 404 ? 'Este título ya no existe.' : 'No se pudo cargar la ficha (' + e.message + ').', e.status === 404 ? null : function () { S.detail(box, id, query); }); });
  };

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
})(window.YF = window.YF || {});
