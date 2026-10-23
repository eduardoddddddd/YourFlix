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
    card.appendChild(el('div', 'cap', label + ' · quedan ' + YF.clock(c.duration - c.position)));
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
  var SORTS = [['az', 'A → Z'], ['added', 'Añadidas'], ['year', 'Por año'], ['rating', 'Valoración']];
  S.grid = function (kind) {
    var s = screen(kind === 'series' ? 'series' : 'movies');
    var sc = el('div', 'scroller');
    s.el.appendChild(sc);
    var head = el('div', 'grid-head'), grid = el('div', 'grid');
    sc.appendChild(head); sc.appendChild(grid);
    var q = { kind: kind, sort: 'az', genre: '', offset: 0, limit: 42 }, total = 0, loadingMore = false;

    function chip(label, on, action) { var c = el('span', 'chip' + (on ? ' on' : ''), label); return YF.f(c, action); }

    function renderHead(genres) {
      YF.clear(head);
      head.appendChild(el('h1', '', (kind === 'series' ? 'Series' : 'Películas') + (q.genre ? ' · ' + q.genre : '')));
      var chips = el('div');
      SORTS.forEach(function (o) { chips.appendChild(chip(o[1], q.sort === o[0], function () { q.sort = o[0]; reload(); })); });
      head.appendChild(chips);
      var gl = el('div');
      gl.appendChild(chip('Todos los géneros', !q.genre, function () { q.genre = ''; reload(); }));
      genres.forEach(function (g) { gl.appendChild(chip(g.name, q.genre === g.name, function () { q.genre = g.name; reload(); })); });
      head.appendChild(gl);
    }

    function more() {
      if (loadingMore || q.offset >= total) return;
      loadingMore = true;
      api.titles(q).then(function (r) {
        total = r.total;
        r.items.forEach(function (t) {
          var c = posterCard(t, function () { if (Array.prototype.indexOf.call(grid.children, c) > grid.children.length - 15) more(); });
          grid.appendChild(c);
        });
        q.offset += r.items.length;
        loadingMore = false;
        if (!grid.children.length) grid.appendChild(el('div', 'empty', 'Nada por aquí.'));
      }).catch(function () { loadingMore = false; YF.toast('No se pudo cargar'); });
    }

    var genresCache = null;
    function reload() {
      q.offset = 0; total = 1;
      YF.clear(grid);
      sc.scrollTop = 0;
      var g = genresCache ? Promise.resolve(genresCache) : api.get('/genres').catch(function () { return []; });
      return g.then(function (list) {
        genresCache = list;
        renderHead(list.slice(0, 14));
        more();
        var first = head.querySelector('.chip.on');
        YF.nav.focus(first);
        return first;
      });
    }
    s.refresh = reload;
    return s;
  };

  // ---------- Ficha ----------
  function chooseFile(files) {
    var withProgress = files.filter(function (f) { return f.progress && !f.progress.watched && f.progress.position > 60; });
    if (withProgress.length) return withProgress[0];
    return files.filter(function (f) { return f.compat.status !== 'optimize'; })[0] || files[0];
  }

  function epLabel(f) { return f.season != null ? f.season + '×' + (f.episode < 10 ? '0' : '') + f.episode : ''; }

  // play: comprueba compatibilidad, decide si reanudar y lanza el reproductor.
  S.play = function (d, f, startAt, onExit) {
    var go = function () {
      YF.player.play({
        file: f, title: d.title.title,
        label: f.season != null ? epLabel(f) + (f.episodeInfo && f.episodeInfo.name ? ' · ' + f.episodeInfo.name : '') : (d.title.year || ''),
        startAt: startAt || 0, onExit: onExit
      });
    };
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
      var ppl = el('div', 'people');
      if (meta.director && meta.director.length) { ppl.appendChild(document.createTextNode((t.kind === 'series' ? 'Creada por ' : 'Dirección ') )); ppl.appendChild(el('b', '', meta.director.join(', '))); }
      if (meta.cast && meta.cast.length) { ppl.appendChild(document.createTextNode('   ·   Reparto ')); ppl.appendChild(el('b', '', meta.cast.slice(0, 5).join(', '))); }
      box.appendChild(ppl);
      if (!files.length) { box.appendChild(el('div', 'warn bad', 'Ningún archivo de este título está disponible ahora (¿disco desconectado?).')); return null; }
      return t.kind === 'series' ? renderSeries(d, box, files) : renderMovie(d, box, files);
    }

    function techBlock(box, f) {
      var b = el('div', 'badges');
      YF.techBadges(f).forEach(function (x, i) { b.appendChild(el('span', 'badge' + (i === 0 ? ' gold' : ''), x)); });
      b.appendChild(el('span', 'badge', f.source + ' · ' + YF.bytes(f.size)));
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
      if (files.length > 1) btns.appendChild(YF.f(el('span', 'btn', 'Copia ' + (copyIdx + 1) + '/' + files.length + ': ' + f.source + ' ⟳'), function () { copyIdx = (copyIdx + 1) % files.length; var fresh = render(d); YF.nav.focus(s.el.querySelectorAll('.btn')[s.el.querySelectorAll('.btn').length - 1] || fresh); }));
      box.appendChild(btns);
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
      if (seasons.length > 1) {
        var tabs = el('div', 'seasons');
        seasons.forEach(function (n) {
          tabs.appendChild(YF.f(el('span', 'chip' + (n === season ? ' on' : ''), n ? 'Temporada ' + n : 'Otros'), function () { season = n; render(d); YF.nav.focus(s.el.querySelector('.seasons .chip.on')); }));
        });
        box.appendChild(tabs);
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
      box.appendChild(el('p', '', 'Mando: flechas para moverse, OK para elegir, Atrás para volver. En el vídeo: ◀ ▶ 10 s, ⏪ ⏩ 30 s, verde audio, amarillo subtítulos.'));
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
