/* Pantallas de YourFlix: Inicio, Películas/Series, Ficha, Buscar y Ajustes.
 * Cada pantalla es {name, el, root (para nav), last (último foco), refresh?}. */
(function (YF) {
  'use strict';
  var el = YF.el, api = YF.api, S = YF.screens = {};

  // ---------- Imágenes diferidas: solo se piden cuando la tarjeta está cerca de la vista ----------
  var io = ('IntersectionObserver' in window) ? new IntersectionObserver(function (entries) {
    entries.forEach(function (en) {
      if (en.isIntersecting) { var img = en.target; img.src = img.dataset.src; io.unobserve(img); }
    });
  }, { rootMargin: '400px 700px' }) : null;

  function lazyImg(src, cls) {
    var img = el('img', cls);
    img.alt = '';
    if (!src) return img;
    img.dataset.src = api.url(src);
    if (io) io.observe(img); else img.src = img.dataset.src;
    img.onerror = function () { img.style.visibility = 'hidden'; };
    return img;
  }

  function metaLine(t, meta) {
    var bits = [];
    if (t.year) bits.push(t.year);
    if (meta && meta.runtime) bits.push(YF.runtime(meta.runtime));
    var g = (meta && meta.genres) || t.genres || [];
    if (g.length) bits.push(g.slice(0, 3).join(', '));
    var line = el('div', 'meta-line', bits.join(' · '));
    var r = (meta && meta.rating) || t.rating;
    if (r) { line.appendChild(document.createTextNode('  ')); line.appendChild(el('span', 'star', '★ ' + Number(r).toFixed(1))); }
    return line;
  }

  // ---------- Tarjetas ----------
  function posterCard(t, onFocus) {
    var c = el('div', 'card');
    var art = el('div', 'art');
    if (t.poster) art.appendChild(lazyImg(t.poster));
    else art.appendChild(el('div', 'ph', t.title));
    if (t.matched === false) art.appendChild(el('span', 'badge-pending', 'sin ficha'));
    c.appendChild(art);
    c.appendChild(el('div', 'cap', t.title + (t.year ? ' (' + t.year + ')' : '')));
    return YF.f(c, function () { YF.app.open(S.detail(t.id)); }, onFocus ? function () { onFocus(t); } : null);
  }

  function continueCard(c, onFocus) {
    var card = el('div', 'card wide');
    var art = el('div', 'art');
    art.appendChild(lazyImg(c.still || c.backdrop || c.poster));
    var bar = el('div', 'pbar');
    bar.style.width = Math.min(100, c.position / (c.duration || 1) * 100) + '%';
    art.appendChild(bar);
    card.appendChild(art);
    var label = c.title + (c.season != null ? ' · ' + c.season + '×' + (c.episode < 10 ? '0' : '') + c.episode + (c.episodeName ? ' ' + c.episodeName : '') : '');
    card.appendChild(el('div', 'cap', (c.next ? 'Siguiente: ' : '') + label + (c.next ? '' : ' · quedan ' + YF.clock(c.duration - c.position))));
    return YF.f(card, function () {
      api.title(c.titleId).then(function (d) {
        var f = d.files.filter(function (x) { return x.id === c.fileId; })[0];
        if (f) S.play(d, f, c.position, function () { YF.app.refreshAll(); });
      }).catch(function () { YF.toast('No se pudo abrir'); });
    }, onFocus ? function () { onFocus({ id: c.titleId, title: c.title, year: c.year, backdrop: c.backdrop }); } : null);
  }

  function row(title, cards) {
    var r = el('section', 'row');
    r.appendChild(el('h2', '', title));
    var track = el('div', 'track');
    cards.forEach(function (c) { track.appendChild(c); });
    r.appendChild(track);
    return r;
  }

  function screen(name) {
    var s = { name: name, el: el('div', 'screen') };
    s.root = s.el;
    return s;
  }

  // ---------- Inicio ----------
  S.home = function () {
    // El héroe queda fijo de fondo y cambia con el título enfocado; las filas se desplazan
    // debajo, una a la vez (patrón habitual en las teles).
    var s = screen('home');
    var hero = el('div', 'hero'), imgA = el('img', 'hero-img'), imgB = el('img', 'hero-img');
    var info = el('div', 'hero-info');
    hero.appendChild(imgA); hero.appendChild(imgB); hero.appendChild(el('div', 'hero-shade')); hero.appendChild(info);
    s.el.appendChild(hero);
    var sc = el('div', 'home-rows');
    var rows = el('div');
    sc.appendChild(rows);
    s.el.appendChild(sc);
    sc.appendChild(el('div', 'loading', 'Cargando la colección…'));
    var cache = {}, shownId = '', front = imgA;

    // El héroe muestra el título enfocado: fondo, año, géneros y sinopsis.
    var setHero = YF.debounce(function (t) {
      if (!t || t.id === shownId) return;
      shownId = t.id;
      var render = function (d) {
        if (shownId !== t.id) return;
        var meta = (d && d.title.meta) || {};
        YF.clear(info);
        info.appendChild(el('div', 'eyebrow', t.kind === 'series' ? 'Serie' : 'Película'));
        info.appendChild(el('h1', '', t.title));
        info.appendChild(metaLine(t, meta));
        info.appendChild(el('div', 'synopsis', meta.synopsis || ''));
        var src = (d && d.backdrop) || t.backdrop;
        if (src) {
          var back = front === imgA ? imgB : imgA;
          back.onload = function () { back.classList.add('shown'); front.classList.remove('shown'); front = back; };
          back.src = api.url(src);
        }
      };
      if (cache[t.id]) render(cache[t.id]);
      else api.title(t.id).then(function (d) { cache[t.id] = d; render(d); }).catch(function () { render(null); });
    }, 350);

    s.refresh = function () {
      return Promise.all([
        api.get('/continue').catch(function () { return []; }),
        api.titles({ sort: 'added', limit: 20 }),
        api.titles({ sort: 'rating', kind: 'movie', limit: 20 }),
        api.titles({ kind: 'series', sort: 'rating', limit: 20 }),
        api.get('/genres').catch(function () { return []; })
      ]).then(function (r) {
        var cont = r[0], added = r[1].items, rated = r[2].items, series = r[3].items, genres = r[4];
        YF.clear(rows);
        var loading = sc.querySelector('.loading');
        if (loading) loading.parentNode.removeChild(loading);
        if (!added.length) { rows.appendChild(el('div', 'empty', 'La colección está vacía. Añade fuentes en el servidor: yourflix source add …')); return; }
        if (cont.length) rows.appendChild(row('Seguir viendo', cont.map(function (c) { return continueCard(c, setHero); })));
        rows.appendChild(row('Añadidas recientemente', added.map(function (t) { return posterCard(t, setHero); })));
        rows.appendChild(row('Películas mejor valoradas', rated.map(function (t) { return posterCard(t, setHero); })));
        if (series.length) rows.appendChild(row('Series', series.map(function (t) { return posterCard(t, setHero); })));
        var top = genres.filter(function (g) { return g.count >= 4; }).slice(0, 4);
        return Promise.all(top.map(function (g) { return api.titles({ genre: g.name, sort: 'rating', limit: 20 }); })).then(function (lists) {
          lists.forEach(function (l, i) { if (l.items.length) rows.appendChild(row(top[i].name, l.items.map(function (t) { return posterCard(t, setHero); }))); });
          var first = cont.length ? { id: cont[0].titleId, title: cont[0].title, year: cont[0].year, backdrop: cont[0].backdrop } : rated[0] || added[0];
          shownId = ''; setHero(first);
          return rows.querySelector('.f');
        });
      });
    };
    return s;
  };

  // ---------- Películas / Series ----------
  // filters: filtros de partida (desde una píldora de la ficha). La pantalla sigue en la pila al
  // abrir una ficha, así que al volver conserva tarjetas, desplazamiento y foco (tema 1).
  S.grid = function (kind, filters) {
    var FL = YF.filters;
    var s = screen(kind === 'series' ? 'series' : 'movies');
    var sc = el('div', 'scroller');
    s.el.appendChild(sc);
    var head = el('div', 'grid-head'), grid = el('div', 'grid');
    sc.appendChild(head); sc.appendChild(grid);
    var q = { kind: kind, sort: 'az', offset: 0, limit: 42 }, total = 0, loadingMore = false, gen = 0;
    s.filters = filters || [];
    var h1 = null;

    function chip(label, cls, id, action) {
      var c = el('span', 'chip' + (cls ? ' ' + cls : ''), label);
      c.dataset.k = id;
      return YF.f(c, action);
    }

    function setFilters(list) { s.filters = list; reload(true); }
    s.setFilters = setFilters;

    function renderHead() {
      var was = YF.nav.current && head.contains(YF.nav.current) ? YF.nav.current.dataset.k : (s.last && head.contains(s.last) ? s.last.dataset.k : '');
      YF.clear(head);
      h1 = head.appendChild(el('h1', '', FL.title(kind, s.filters, total || null)));
      var sorts = el('div', 'chips');
      FL.SORTS.forEach(function (o) { sorts.appendChild(chip(o[1], q.sort === o[0] ? 'on' : '', 'sort:' + o[0], function () { q.sort = o[0]; reload(true); })); });
      head.appendChild(sorts);
      var by = el('div', 'chips');
      by.appendChild(el('span', 'chips-label', 'Filtrar por'));
      FL.GROUPS.forEach(function (g) {
        var on = g.keys.some(function (k) { return FL.has(s.filters, k); });
        by.appendChild(chip(g.label + ' ▸', on ? 'on' : '', 'by:' + g.id, function () { YF.app.open(S.picker(s, kind, g)); }));
      });
      if (s.filters.length) by.appendChild(chip('▶ Ver en kiosco', 'kiosk', 'kiosk', function () { YF.app.open(S.kiosk({ kind: kind, filters: s.filters })); }));
      head.appendChild(by);
      if (s.filters.length) {
        var act = el('div', 'chips');
        s.filters.forEach(function (f) {
          act.appendChild(chip(f.name + '  ✕', 'active', 'rm:' + f.key, function () { setFilters(FL.remove(s.filters, f.key)); }));
        });
        head.appendChild(act);
      }
      // Mismo botón tras redibujar; si era una píldora quitada, la primera de «Filtrar por».
      var again = (was && head.querySelector('[data-k="' + was + '"]')) || (was ? head.querySelector('[data-k^="rm:"]') || head.querySelector('[data-k^="by:"]') : null);
      return again;
    }

    function card(t) {
      var c = posterCard(t, function () { if (Array.prototype.indexOf.call(grid.children, c) > grid.children.length - 15) more(); });
      c.dataset.id = t.id;
      return c;
    }

    function fetchPage(offset, limit) {
      return api.titles(FL.params(s.filters, { kind: q.kind, sort: q.sort, offset: offset, limit: limit }));
    }

    function setTotal(n) { total = n; if (h1) h1.textContent = FL.title(kind, s.filters, total); }

    function more() {
      if (loadingMore || q.offset >= total) return;
      loadingMore = true;
      var my = gen;
      fetchPage(q.offset, q.limit).then(function (r) {
        if (my !== gen) return;
        setTotal(r.total);
        r.items.forEach(function (t) { grid.appendChild(card(t)); });
        q.offset += r.items.length;
        loadingMore = false;
        if (!grid.children.length) grid.appendChild(el('div', 'empty', 'Nada por aquí con estos filtros.'));
      }).catch(function () { loadingMore = false; YF.toast('No se pudo cargar'); });
    }

    // reload: vuelve a empezar (cambio de orden o de filtros). keepFocus: el foco sigue en la cabecera.
    function reload(keepFocus) {
      gen++;
      q.offset = 0; total = 1; loadingMore = false;
      YF.clear(grid);
      sc.scrollTop = 0;
      var again = renderHead();
      more();
      var first = again || head.querySelector('.chip.on');
      var mine = YF.app.current && YF.app.current() === s;
      if (mine && (keepFocus || !YF.nav.current || !document.body.contains(YF.nav.current))) YF.nav.focus(first);
      else if (!mine && again) s.last = again;
      return Promise.resolve(first);
    }
    s.refresh = function () { return reload(false); };

    // Al volver: si se vio algo, solo cambia la lista cuando hay filtro de estado (visto / sin ver).
    // Entonces se recargan los mismos títulos que había y el foco vuelve a la tarjeta por su ID.
    s.onReturn = function (dirty) {
      if (!dirty || !FL.has(s.filters, 'state')) return;
      var id = s.last && s.last.dataset ? s.last.dataset.id : '';
      var n = Math.max(q.limit, Math.min(500, grid.querySelectorAll('.card').length)), my = ++gen;
      fetchPage(0, n).then(function (r) {
        if (my !== gen) return;
        setTotal(r.total);
        YF.clear(grid);
        r.items.forEach(function (t) { grid.appendChild(card(t)); });
        q.offset = r.items.length;
        if (!grid.children.length) grid.appendChild(el('div', 'empty', 'Nada por aquí con estos filtros.'));
        var back = (id && grid.querySelector('[data-id="' + id + '"]')) || grid.querySelector('.card') || head.querySelector('.chip.on');
        if (YF.app.current() === s && (!YF.nav.current || !document.body.contains(YF.nav.current) || grid.contains(YF.nav.current))) YF.nav.focus(back);
      }).catch(function () {});
    };
    return s;
  };

  // ---------- Panel «Filtrar por» ----------
  // Encima de la lista (overlay): ▲▼ recorre, OK elige y la lista de debajo cambia al momento,
  // ◀▶ salta de letra en las listas largas, Atrás cierra. Teclas propias: con 2.000 actores
  // no conviene medir cada elemento en cada pulsación. La lista se dibuja por ventanas.
  var ROWS = 13;
  S.picker = function (gridScreen, kind, group) {
    var FL = YF.filters;
    var s = screen('picker');
    s.overlay = true;
    s.el.className = 'screen picker-screen';
    var panel = el('div', 'picker'), title = el('h2', '', 'Filtrar por ' + group.label.toLowerCase());
    var list = el('div', 'plist'), side = el('div', 'pletter'), hint = el('p', 'phint');
    panel.appendChild(title); panel.appendChild(list); panel.appendChild(hint);
    s.el.appendChild(panel); s.el.appendChild(side);
    var items = [], sel = 0, all = false;
    hint.textContent = '▲▼ elegir · OK aplicar' + (group.letters ? ' · ◀▶ letra' : '') + ' · Atrás cerrar';

    function load() {
      var q = FL.params(gridScreen.filters, { kind: kind, only: group.keys.join(',') }, group.keys);
      if (all) q.all = 1;
      return api.facets(q).then(function (r) {
        items = FL.pickerItems(group, r.facets);
        if (group.all) items.unshift({ toggle: true });
        if (group.keys.length === 1) items.unshift({ key: group.keys[0], value: '', name: 'Todos', count: r.total });
        sel = 0;
        for (var i = 0; i < items.length; i++) if (isOn(items[i]) && items[i].value) { sel = i; break; }
        if (items.length && items[sel].head) sel = step(sel, 1);
        draw();
        return null;
      });
    }

    function isOn(it) { return it.key && (it.value ? FL.has(gridScreen.filters, it.key, it.value) : !FL.has(gridScreen.filters, it.key)); }
    function step(i, d) { var j = i + d; while (j >= 0 && j < items.length && items[j].head) j += d; return j < 0 || j >= items.length ? i : j; }

    function draw() {
      YF.clear(list);
      var start = Math.max(0, Math.min(sel - Math.floor(ROWS / 2), items.length - ROWS));
      for (var i = start; i < Math.min(items.length, start + ROWS); i++) {
        var it = items[i], row;
        if (it.head) row = el('div', 'phead', it.head);
        else if (it.toggle) row = el('div', 'pitem toggle', all ? '◉ Todos los actores' : '○ Solo con 2 o más títulos · OK: todos');
        else {
          row = el('div', 'pitem' + (isOn(it) ? ' on' : ''));
          row.appendChild(el('span', 'pname', it.name));
          if (it.count != null) row.appendChild(el('span', 'pcount', String(it.count)));
        }
        if (i === sel) row.classList.add('sel');
        list.appendChild(row);
      }
      if (!items.length) list.appendChild(el('div', 'phead', 'Nada que elegir con los filtros puestos.'));
      var it2 = items[sel];
      side.textContent = group.letters && it2 && it2.name && it2.value ? FL.letter(it2.name) : '';
      side.classList.toggle('hidden', !side.textContent);
    }

    function choose() {
      var it = items[sel];
      if (!it) return;
      if (it.toggle) { all = !all; load(); return; }
      var next = FL.remove(gridScreen.filters, it.key);
      // Año y década se excluyen: elegir uno quita el otro.
      if (it.key === 'year') next = FL.remove(next, 'decade');
      if (it.key === 'decade') next = FL.remove(next, 'year');
      if (it.value && !isOn(it)) next = FL.add(next, it);
      gridScreen.setFilters(next);
      draw();
    }

    s.key = function (k) {
      switch (k) {
        case 'Up': sel = step(sel, -1); draw(); return true;
        case 'Down': sel = step(sel, 1); draw(); return true;
        case 'ChannelUp': for (var a = 0; a < ROWS; a++) sel = step(sel, -1); draw(); return true;
        case 'ChannelDown': for (var b = 0; b < ROWS; b++) sel = step(sel, 1); draw(); return true;
        case 'Left': case 'Right':
          if (group.letters) { sel = FL.letterJump(items.map(function (x) { return x.value ? x.name : null; }), sel, k === 'Right' ? 1 : -1); draw(); }
          return true;
        case 'Enter': choose(); return true;
        case 'Back': YF.app.back(); return true;
      }
      return true;
    };
    s.refresh = function () { YF.nav.blur(); return load(); };
    return s;
  };

  // ---------- Ficha ----------
  function chooseFile(files) {
    var withProgress = files.filter(function (f) { return f.progress && !f.progress.watched && f.progress.position > 60; });
    if (withProgress.length) return withProgress[0];
    return files.filter(function (f) { return f.compat.status !== 'optimize'; })[0] || files[0];
  }

  function epLabel(f) { return f.season != null ? f.season + '×' + (f.episode < 10 ? '0' : '') + f.episode : ''; }

  function resumeAt(f) { var p = f.progress; return p && !p.watched && p.position > 60 ? p.position : 0; }

  // playOpts: opciones del reproductor; en series, next() da el capítulo siguiente (en cadena).
  S.playOpts = function (d, f, startAt, onExit) {
    var files = d.files || [], series = d.title.kind === 'series';
    return {
      file: f, title: d.title.title,
      label: f.season != null ? epLabel(f) + (f.episodeInfo && f.episodeInfo.name ? ' · ' + f.episodeInfo.name : '') : (d.title.year || ''),
      startAt: startAt || 0, onExit: onExit,
      next: series ? function () {
        var n = files[files.indexOf(f) + 1];
        return n ? S.playOpts(d, n, resumeAt(n), onExit) : null;
      } : null
    };
  };

  // play: comprueba compatibilidad, decide si reanudar y lanza el reproductor.
  S.play = function (d, f, startAt, onExit) {
    var go = function () { YF.player.play(S.playOpts(d, f, startAt, onExit)); };
    if (f.compat.status === 'optimize') {
      YF.dialog('Atención: ' + f.compat.reasons.join('; ') + '. ¿Reproducir igualmente?', [
        { label: 'Reproducir', primary: true, action: go }, { label: 'Cancelar' }]);
    } else go();
  };

  S.detail = function (id) {
    var s = screen('detail');
    s.refresh = function () {
      return api.title(id).then(function (d) { return render(d); });
    };
    var copyIdx = 0, season = null;

    function render(d) {
      var t = d.title, meta = t.meta || {}, files = d.files || [];
      YF.clear(s.el);
      if (d.backdrop) { var bg = el('img', 'detail-bg'); bg.src = api.url(d.backdrop); s.el.appendChild(bg); }
      s.el.appendChild(el('div', 'detail-shade'));
      if (d.poster && t.kind !== 'series') { var p = el('img', 'poster-big'); p.src = api.url(d.poster); s.el.appendChild(p); }
      var box = el('div', 'detail' + (t.kind === 'series' ? ' series' : ''));
      s.el.appendChild(box);
      box.appendChild(el('div', 'eyebrow', (t.kind === 'series' ? 'Serie' : 'Película') + (t.matchSource === 'filename' ? ' · ficha pendiente' : '')));
      box.appendChild(el('h1', '', t.title));
      box.appendChild(metaLine(t, meta));
      if (meta.tagline) box.appendChild(el('div', 'tagline', meta.tagline));
      if (meta.synopsis) box.appendChild(el('div', 'synopsis', meta.synopsis));
      if (!d.links) {
        var ppl = el('div', 'people');
        if (meta.director && meta.director.length) { ppl.appendChild(document.createTextNode((t.kind === 'series' ? 'Creada por ' : 'Dirección ') )); ppl.appendChild(el('b', '', meta.director.join(', '))); }
        if (meta.cast && meta.cast.length) { ppl.appendChild(document.createTextNode('   ·   Reparto ')); ppl.appendChild(el('b', '', meta.cast.slice(0, 5).join(', '))); }
        box.appendChild(ppl);
      }
      if (!files.length) { box.appendChild(el('div', 'warn bad', 'Ningún archivo de este título está disponible ahora (¿disco desconectado?).')); linkRows(box, d); return null; }
      return t.kind === 'series' ? renderSeries(d, box, files) : renderMovie(d, box, files);
    }

    // Filas de píldoras (Dirección, Reparto, País · Año · Década · Géneros · Duración). OK abre la
    // lista filtrada encima de la ficha; Atrás vuelve aquí con el foco en la misma píldora.
    function linkRows(box, d) {
      var rows = YF.filters.linkRows(d.links, d.title.kind);
      if (!rows.length) return;
      var wrap = el('div', 'links');
      rows.forEach(function (r) {
        var line = el('div', 'lrow');
        line.appendChild(el('span', 'llabel', r.label));
        r.items.forEach(function (it) {
          var p = YF.f(el('span', 'chip pill', it.text), function () { YF.app.open(S.grid(d.title.kind, YF.filters.add([], it))); });
          p.dataset.k = it.key + ':' + it.value;
          line.appendChild(p);
        });
        wrap.appendChild(line);
      });
      box.appendChild(wrap);
    }

    function techBlock(box, f) {
      var b = el('div', 'badges');
      YF.techBadges(f).forEach(function (x, i) { b.appendChild(el('span', 'badge' + (i === 0 ? ' gold' : ''), x)); });
      b.appendChild(el('span', 'badge', YF.bytes(f.size)));
      box.appendChild(b);
      if (f.compat.status !== 'direct') box.appendChild(el('div', 'warn' + (f.compat.status === 'optimize' ? ' bad' : ''), '⚠ ' + f.compat.reasons.join(' · ')));
    }

    function renderMovie(d, box, files) {
      if (copyIdx >= files.length) copyIdx = 0;
      var f = copyIdx ? files[copyIdx] : chooseFile(files);
      copyIdx = files.indexOf(f);
      techBlock(box, f);
      var btns = el('div');
      var pr = f.progress, resume = pr && !pr.watched && pr.position > 60;
      var onExit = function () { s.refresh().then(function (first) { YF.nav.focus(first); }); };
      var main = YF.f(el('span', 'btn primary', resume ? '▶ Reanudar desde ' + YF.clock(pr.position) : '▶ Reproducir'), function () { S.play(d, f, resume ? pr.position : 0, onExit); });
      btns.appendChild(main);
      if (resume) btns.appendChild(YF.f(el('span', 'btn', '↺ Desde el principio'), function () { S.play(d, f, 0, onExit); }));
      if (files.length > 1) btns.appendChild(YF.f(el('span', 'btn', 'Copia ' + (copyIdx + 1) + '/' + files.length + ' ⟳'), function () { copyIdx = (copyIdx + 1) % files.length; var fresh = render(d); YF.nav.focus(s.el.querySelectorAll('.btn')[s.el.querySelectorAll('.btn').length - 1] || fresh); }));
      box.appendChild(btns);
      linkRows(box, d);
      return main;
    }

    function renderSeries(d, box, files) {
      var seasons = [];
      files.forEach(function (f) { var n = f.season == null ? 0 : f.season; if (seasons.indexOf(n) < 0) seasons.push(n); });
      seasons.sort(function (a, b) { return a - b; });
      var next = chooseFile(files);
      if (season == null) season = next.season == null ? 0 : next.season;
      var pr = next.progress, resume = pr && !pr.watched && pr.position > 60;
      var onExit = function () { s.refresh().then(function (first) { YF.nav.focus(first); }); };
      techBlock(box, next);
      var main = YF.f(el('span', 'btn primary', (resume ? '▶ Reanudar ' : '▶ Ver ') + epLabel(next)), function () { S.play(d, next, resume ? pr.position : 0, onExit); });
      var btns = el('div');
      btns.appendChild(main);
      box.appendChild(btns);
      linkRows(box, d);
      if (seasons.length > 1) {
        var tabs = el('span', 'seasons');
        seasons.forEach(function (n) {
          tabs.appendChild(YF.f(el('span', 'chip' + (n === season ? ' on' : ''), n ? 'Temporada ' + n : 'Otros'), function () { season = n; render(d); YF.nav.focus(s.el.querySelector('.seasons .chip.on')); }));
        });
        btns.appendChild(tabs); // en la fila del botón: debajo van las píldoras y los capítulos
      }
      var eps = el('div', 'episodes row');
      var track = el('div', 'track');
      files.filter(function (f) { return (f.season == null ? 0 : f.season) === season; }).forEach(function (f) {
        var e = el('div', 'episode'), art = el('div', 'art'), info = f.episodeInfo || {};
        if (info.still) art.appendChild(lazyImg(info.still));
        if (f.season != null) art.appendChild(el('span', 'num', epLabel(f)));
        if (f.progress && f.progress.watched) art.appendChild(el('span', 'watched', '✓'));
        else if (f.progress && f.progress.position > 60) { var b = el('div', 'pbar'); b.style.width = Math.min(100, f.progress.position / (f.duration || 1) * 100) + '%'; art.appendChild(b); }
        e.appendChild(art);
        e.appendChild(el('div', 'ep-name', info.name || f.label || ('Capítulo ' + (f.episode || ''))));
        e.appendChild(el('div', 'ep-desc', info.overview || YF.clock(f.duration)));
        var p = f.progress, res = p && !p.watched && p.position > 60;
        track.appendChild(YF.f(e, function () { S.play(d, f, res ? p.position : 0, onExit); }));
      });
      eps.appendChild(track);
      s.el.appendChild(eps);
      return main;
    }
    return s;
  };

  // ---------- Kiosco ----------
  // Una ficha a pantalla completa cada vez, en orden aleatorio fijo durante la sesión (la semilla
  // no cambia, así que la anterior es siempre la misma). Teclas propias: ◀▶ cambian, OK muestra
  // Reproducir · Ver ficha, ▼ abre los filtros, Atrás vuelve al menú (o a la lista de la que vino).
  var kioskSeed = 0, AUTO_MS = 20000;
  S.kiosk = function (opts) {
    opts = opts || {};
    var FL = YF.filters, $ = YF.$;
    if (!kioskSeed) kioskSeed = FL.seed();
    var s = screen('kiosk');
    s.el.className = 'screen kiosk';
    var layers = [el('div', 'kslide shown'), el('div', 'kslide')], front = 0;
    s.el.appendChild(layers[0]); s.el.appendChild(layers[1]);
    var btns = el('div', 'kbtns hidden'), bar = el('div', 'kbar hidden'), hint = el('div', 'khint'), counter = el('div', 'kcount');
    s.el.appendChild(btns); s.el.appendChild(bar); s.el.appendChild(hint); s.el.appendChild(counter);
    var base = opts.filters || [];
    var cfg = { kind: opts.kind || 'movie', unwatched: false, genre: '', decade: '', auto: false };
    var items = [], i = 0, cache = {}, mode = 'view', showGen = 0, autoTimer = 0, btn = 0, keep = [], facetsCache = {}, barDirty = false;
    var genreNames = [], decades = [];

    function filters() {
      var list = base.slice();
      if (cfg.unwatched) list = FL.add(list, { key: 'state', value: 'unwatched', name: 'Sin ver' });
      if (cfg.genre) list = FL.add(list, { key: 'genre', value: cfg.genre, name: cfg.genre });
      if (cfg.decade) list = FL.add(list, { key: 'decade', value: cfg.decade, name: decadeName(cfg.decade) });
      return list;
    }
    function decadeName(v) { var d = decades.filter(function (x) { return x.value === v; })[0]; return d ? d.name : 'Años ' + v; }

    function load() {
      var q = FL.params(filters(), { kind: cfg.kind === 'all' ? '' : cfg.kind, sort: 'random', seed: kioskSeed, limit: 500 });
      return api.titles(q).then(function (r) { items = r.items; i = 0; show(0); return null; });
    }

    function detail(id) {
      if (!cache[id]) cache[id] = api.title(id).catch(function (e) { delete cache[id]; throw e; });
      return cache[id];
    }

    function go(d) { if (items.length) { i = FL.circ(i + d, items.length); show(d); } }

    function show() {
      var my = ++showGen;
      var label = base.length ? 'Kiosco · ' + base.map(function (x) { return x.name; }).join(' · ') + ' · ' : '';
      if (!items.length) {
        counter.textContent = label;
        var l = layers[front];
        YF.clear(l);
        l.appendChild(el('div', 'empty', 'No hay títulos con estos filtros. Pulsa ▼ para cambiarlos.'));
        return;
      }
      counter.textContent = label + (i + 1) + ' / ' + items.length;
      var t = items[i];
      detail(t.id).then(function (d) { if (my === showGen) paint(t, d, my); }, function () { if (my === showGen) YF.toast('No se pudo cargar la ficha'); });
    }

    function chooseFileK(files) { return files.length ? chooseFile(files) : null; }

    // paint: dibuja en la capa de detrás y hace el fundido cuando la imagen está lista.
    function paint(t, d, my) {
      var back = layers[1 - front], tt = d.title, meta = tt.meta || {}, links = d.links || {};
      YF.clear(back);
      var bg = el('img', 'kbg');
      back.appendChild(bg);
      back.appendChild(el('div', 'kshade'));
      if (d.poster) { var p = el('img', 'kposter'); p.alt = ''; p.src = api.url(d.poster); back.appendChild(p); }
      var info = el('div', 'kinfo');
      back.appendChild(info);
      info.appendChild(el('div', 'eyebrow', tt.kind === 'series' ? 'Serie' : 'Película'));
      info.appendChild(el('h1', '', tt.title));
      var bits = [];
      if (tt.year) bits.push(tt.year);
      if (meta.runtime) bits.push(YF.runtime(meta.runtime));
      var line = el('div', 'meta-line', bits.join(' · '));
      if (meta.rating || t.rating) { line.appendChild(document.createTextNode(bits.length ? ' · ' : '')); line.appendChild(el('span', 'star', '★ ' + Number(meta.rating || t.rating).toFixed(1))); }
      var countries = (links.country || []).map(function (c) { return c.name; });
      if (countries.length) line.appendChild(document.createTextNode(' · ' + countries.join(', ')));
      info.appendChild(line);
      var genres = meta.genres || t.genres || [];
      if (genres.length) info.appendChild(el('div', 'kgenres', genres.join(' · ')));
      var ppl = el('div', 'people');
      if (meta.director && meta.director.length) { ppl.appendChild(document.createTextNode(tt.kind === 'series' ? 'Creada por ' : 'Dirección ')); ppl.appendChild(el('b', '', meta.director.join(', '))); }
      if (meta.cast && meta.cast.length) { ppl.appendChild(document.createTextNode((ppl.firstChild ? '   ·   ' : '') + 'Reparto ')); ppl.appendChild(el('b', '', meta.cast.slice(0, 5).join(', '))); }
      info.appendChild(ppl);
      if (meta.tagline) info.appendChild(el('div', 'tagline', meta.tagline));
      if (meta.synopsis) info.appendChild(el('div', 'synopsis', meta.synopsis));
      var f = chooseFileK(d.files || []);
      var b = el('div', 'badges');
      FL.kioskBadges(f).forEach(function (x) { b.appendChild(el('span', 'badge gold', x)); });
      info.appendChild(b);
      var pr = f && f.progress;
      if (pr && !pr.watched && pr.position > 60) {
        var pw = el('div', 'kprogress'), fill = el('span');
        fill.style.width = Math.min(100, pr.position / (f.duration || 1) * 100) + '%';
        pw.appendChild(fill);
        info.appendChild(pw);
        info.appendChild(el('div', 'kleft', 'A medias · quedan ' + YF.clock((f.duration || 0) - pr.position)));
      }
      var done = false;
      var swap = function () {
        if (done || my !== showGen) return;
        done = true;
        back.classList.add('shown');
        layers[front].classList.remove('shown');
        front = 1 - front;
        preload();
      };
      bg.onload = swap; bg.onerror = function () { bg.style.visibility = 'hidden'; swap(); };
      var src = FL.big(t.backdrop) || d.backdrop;
      if (src) bg.src = api.url(src); else swap();
      setTimeout(swap, 1500);
      s.cur = { t: t, d: d, f: f };
    }

    // La siguiente queda en caché (ficha, fondo y carátula); la anterior ya se vio.
    function preload() {
      if (items.length < 2) return;
      [1, -1].forEach(function (dd) {
        var n = items[FL.circ(i + dd, items.length)];
        detail(n.id).then(function (d) {
          keep = keep.slice(-6);
          var a = new Image(); a.src = api.url(FL.big(n.backdrop) || d.backdrop); keep.push(a);
          if (d.poster) { var b = new Image(); b.src = api.url(d.poster); keep.push(b); }
        }).catch(function () {});
      });
    }

    // ---- Modos: menu (foco en el menú), view, buttons (Reproducir · Ver ficha), filter (barra) ----
    function setHint() {
      hint.textContent = mode === 'buttons' ? '▲▼ elegir · OK aceptar · Atrás volver'
        : mode === 'filter' ? '◀▶▲▼ moverse · OK cambiar · Atrás aplicar y cerrar'
        : '◀ ▶ cambiar · OK reproducir o ver ficha · ▼ filtros' + (cfg.auto ? ' · pase automático cada 20 s' : '') + ' · Atrás ' + (YF.app.stack[0] === s ? 'menú' : 'volver');
    }

    function setView() {
      mode = 'view';
      $('app').classList.add('fullscreen');
      YF.nav.blur();
      btns.classList.add('hidden'); bar.classList.add('hidden');
      setHint();
    }

    function setMenu() {
      mode = 'menu';
      stopAuto(true);
      $('app').classList.remove('fullscreen');
      var m = $('menu');
      YF.nav.focus(m.querySelector('.menu-item.active') || m.querySelector('.menu-item'));
      hint.textContent = '▼ volver al kiosco';
    }

    function drawBtns() {
      YF.clear(btns);
      ['▶ Reproducir', 'Ver ficha'].forEach(function (label, n) { btns.appendChild(el('span', 'btn' + (n === btn ? ' focused' : '') + (n === 0 ? ' primary' : ''), label)); });
    }

    function setButtons() {
      if (!s.cur) return;
      mode = 'buttons'; btn = 0;
      drawBtns();
      btns.classList.remove('hidden');
      setHint();
    }

    function act() {
      var c = s.cur;
      if (!c) return;
      if (btn === 1) { YF.app.open(S.detail(c.t.id)); return; }
      if (!c.f) { YF.toast('Ningún archivo disponible ahora'); return; }
      var pr = c.f.progress, resume = pr && !pr.watched && pr.position > 60;
      setView();
      S.play(c.d, c.f, resume ? pr.position : 0, function () {
        s.dirty = false;
        if (YF.app.current() === s) { delete cache[c.t.id]; show(); setView(); }
      });
    }

    function startAuto() {
      clearInterval(autoTimer);
      autoTimer = setInterval(function () {
        if (YF.app.current() === s && mode === 'view' && !YF.player.active) go(1);
      }, AUTO_MS);
    }
    function stopAuto(quiet) {
      if (!cfg.auto) return false;
      cfg.auto = false;
      clearInterval(autoTimer);
      if (!quiet) YF.toast('Pase automático parado');
      setHint();
      return true;
    }

    // Barra de filtros: Películas / Series / Todo · Sin ver · un género · una década · pase automático.
    function drawBar() {
      var was = YF.nav.current && bar.contains(YF.nav.current) ? YF.nav.current.dataset.k : 'kind:' + cfg.kind;
      YF.clear(bar);
      var chip = function (label, on, k, fn) {
        var c = el('span', 'chip' + (on ? ' on' : ''), label);
        c.dataset.k = k;
        bar.appendChild(YF.f(c, function () { if (fn() !== false) barDirty = true; drawBar(); }));
      };
      if (base.length) bar.appendChild(el('span', 'chips-label', base.map(function (x) { return x.name; }).join(' · ')));
      [['movie', 'Películas'], ['series', 'Series'], ['all', 'Todo']].forEach(function (o) {
        chip(o[1], cfg.kind === o[0], 'kind:' + o[0], function () { if (cfg.kind !== o[0]) { cfg.kind = o[0]; cfg.genre = ''; cfg.decade = ''; loadFacets(); } });
      });
      chip('Sin ver', cfg.unwatched, 'unwatched', function () { cfg.unwatched = !cfg.unwatched; });
      chip('Género: ' + (cfg.genre || 'todos') + ' ▸', !!cfg.genre, 'genre', function () { cfg.genre = FL.cycle(genreNames, cfg.genre); });
      chip('Década: ' + (cfg.decade ? decadeName(cfg.decade) : 'todas') + ' ▸', !!cfg.decade, 'decade', function () { cfg.decade = FL.cycle(decades.map(function (x) { return x.value; }), cfg.decade); });
      chip('Pase automático: ' + (cfg.auto ? 'cada 20 s' : 'no'), cfg.auto, 'auto', function () { cfg.auto = !cfg.auto; return false; });
      YF.nav.focus(bar.querySelector('[data-k="' + was + '"]') || bar.querySelector('.chip'), true);
    }

    function loadFacets() {
      var k = cfg.kind;
      var done = function (r) { genreNames = (r.genre || []).map(function (g) { return g.value; }); decades = r.decade || []; if (mode === 'filter') drawBar(); };
      if (facetsCache[k]) { done(facetsCache[k]); return; }
      api.facets(FL.params(base, { kind: k === 'all' ? '' : k, only: 'genre,decade' })).then(function (r) { facetsCache[k] = r.facets || {}; if (cfg.kind === k) done(facetsCache[k]); }).catch(function () {});
    }

    function setFilter() {
      mode = 'filter'; barDirty = false;
      bar.classList.remove('hidden');
      setHint();
      loadFacets();
      drawBar();
    }

    function closeBar() {
      var reload = barDirty;
      setView();
      if (cfg.auto) startAuto(); else clearInterval(autoTimer);
      if (reload) load();
    }

    s.key = function (k) {
      if (mode === 'menu') {
        if (k === 'Down') { setView(); return true; }
        return false;
      }
      if (mode !== 'filter' && stopAuto()) return true; // cualquier tecla para el pase automático
      if (mode === 'view') {
        switch (k) {
          case 'Left': go(-1); break;
          case 'Right': go(1); break;
          case 'Enter': setButtons(); break;
          case 'Down': setFilter(); break;
          case 'Up': if (YF.app.stack[0] === s) setMenu(); break;
          case 'Back': if (YF.app.stack[0] === s) setMenu(); else { clearInterval(autoTimer); YF.app.back(); } break;
        }
        return true;
      }
      if (mode === 'buttons') {
        if (k === 'Up' || k === 'Down' || k === 'Left' || k === 'Right') { btn = 1 - btn; drawBtns(); }
        else if (k === 'Enter') act();
        else if (k === 'Back') setView();
        return true;
      }
      // filter: la barra se recorre con el navegador espacial.
      if (k === 'Back') { closeBar(); return true; }
      if (k === 'Up' || k === 'Down') { if (!YF.nav.move(k.toLowerCase())) closeBar(); return true; }
      return false;
    };

    // Al mostrarse (al abrirlo o al volver de la ficha o del reproductor): mismo título.
    s.onShow = function () { setView(); };
    s.onReturn = function (dirty) { if (dirty && s.cur) { delete cache[s.cur.t.id]; show(); } };
    s.refresh = function () { setView(); return load(); };
    return s;
  };

  // ---------- Buscar ----------
  var LETTERS = 'ABCDEFGHIJKLMNÑOPQRSTUVWXYZ1234567890'.split('');
  S.search = function () {
    var s = screen('search');
    var box = el('div', 'search'), query = el('div', 'query'), kb = el('div', 'keyboard'), results = el('div', 'results');
    s.el.appendChild(box);
    box.appendChild(query); box.appendChild(kb); box.appendChild(results);
    var text = YF.storage.get('lastSearch', '');
    function showQuery() { YF.clear(query); query.appendChild(document.createTextNode(text || '')); query.appendChild(el('span', 'caret', '▍')); if (!text) query.appendChild(el('span', 'meta-line', '  título, director o reparto')); }
    var run = YF.debounce(function () {
      YF.storage.set('lastSearch', text);
      if (text.trim().length < 2) { YF.clear(results); return; }
      api.titles({ q: text.trim(), limit: 24 }).then(function (r) {
        YF.clear(results);
        if (!r.items.length) results.appendChild(el('p', 'meta-line', 'Sin resultados para «' + text + '»'));
        r.items.forEach(function (t) { results.appendChild(posterCard(t)); });
      }).catch(function () {});
    }, 350);
    function type(ch) { text += ch; showQuery(); run(); }
    LETTERS.forEach(function (ch) { kb.appendChild(YF.f(el('span', 'key', ch), function () { type(ch.toLowerCase()); })); });
    kb.appendChild(YF.f(el('span', 'key wide', 'Espacio'), function () { type(' '); }));
    kb.appendChild(YF.f(el('span', 'key wide', '⌫ Borrar'), function () { text = text.slice(0, -1); showQuery(); run(); }));
    s.refresh = function () { showQuery(); run(); return Promise.resolve(kb.querySelector('.key')); };
    return s;
  };

  // ---------- Ajustes ----------
  S.settings = function () {
    var s = screen('settings');
    var box = el('div', 'settings');
    s.el.appendChild(box);
    s.refresh = function () {
      YF.clear(box);
      box.appendChild(el('h1', '', 'Ajustes'));
      var info = api.info || {};
      var line = function (k, v) { var p = el('p', '', k + ': '); p.appendChild(el('span', 'value', v)); box.appendChild(p); };
      line('Servidor', api.base || 'sin conectar');
      line('Versión del servidor', (info.version || '?') + ' · id ' + (info.serverId || '?'));
      line('App', 'YourFlix TV ' + YF.version);
      var btns = el('div');
      btns.style.marginTop = '30px';
      var again = YF.f(el('span', 'btn primary', 'Buscar servidor en la red'), function () { YF.storage.set('server', ''); YF.app.boot(); });
      btns.appendChild(again);
      btns.appendChild(YF.f(el('span', 'btn', 'Escribir dirección IP'), function () { S.ipDialog(); }));
      box.appendChild(btns);
      box.appendChild(el('p', '', 'Mando: flechas para moverse, OK para elegir, Atrás para volver. En el vídeo: ◀ ▶ 10 s, ⏪ ⏩ 30 s, ▼ botones de audio y subtítulos (o verde y amarillo).'));
      return Promise.resolve(again);
    };
    return s;
  };

  // Teclado numérico para escribir la IP del servidor (puerto 8787 por defecto).
  S.ipDialog = function (initial) {
    var ip = initial || ((api.base || '').replace(/^https?:\/\//, '').replace(/:\d+$/, '')) || '192.168.1.';
    var shown = el('div', 'query');
    var pad = el('div', 'keyboard');
    function draw() { YF.clear(shown); shown.appendChild(document.createTextNode(ip)); shown.appendChild(el('span', 'caret', '▍')); }
    '1234567890.'.split('').forEach(function (ch) { pad.appendChild(YF.f(el('span', 'key', ch), function () { ip += ch; draw(); })); });
    pad.appendChild(YF.f(el('span', 'key wide', '⌫ Borrar'), function () { ip = ip.slice(0, -1); draw(); }));
    draw();
    YF.dialog('Dirección del servidor YourFlix', [
      { label: 'Conectar', primary: true, action: function () {
        api.use('http://' + ip + ':8787').then(function () { YF.toast('Conectado a ' + ip); YF.app.boot(); }, function () { YF.toast('No responde ' + ip + ':8787'); S.ipDialog(ip); });
      } },
      { label: 'Cancelar' }], [shown, pad]);
  };
})(window.YF = window.YF || {});
