/* Kiosco (#/kiosco): una ficha cada vez a pantalla completa, en orden aleatorio fijo durante la sesión
 * (sort=random&seed=N). Tres diapositivas en el DOM (anterior, actual y siguiente): la imagen de la
 * siguiente ya está cargada cuando se pasa. La dirección guarda la semilla y la posición
 * (#/kiosco?semilla=8213&i=14), así que al volver de la ficha o del reproductor se sigue en el mismo título. */
(function (YF) {
  'use strict';
  var el = YF.el, api = YF.api, P = YF.playback, S = YF.screens, H = S.h;
  var TIPOS = [['peliculas', 'Películas', 'movie'], ['series', 'Series', 'series'], ['todo', 'Todo', '']];
  var AUTO_MS = 20000, LIMIT = 500;
  var details = {}; // id → promesa de la ficha completa

  function detail(id) {
    if (!details[id]) details[id] = api.title(id).catch(function (e) { delete details[id]; throw e; });
    return details[id];
  }

  // Semilla fija durante la sesión (la misma al volver a la pestaña); nueva al abrir otra.
  function sessionSeed() {
    var s = 0;
    try { s = parseInt(sessionStorage.getItem('yourflix.web.kioskSeed'), 10) || 0; } catch (_) {}
    if (!s) {
      s = 1 + Math.floor(Math.random() * 9999);
      try { sessionStorage.setItem('yourflix.web.kioskSeed', String(s)); } catch (_) {}
    }
    return s;
  }

  // Insignias de la copia que se reproduciría: calidad, HDR y VOSE (audio no castellano con subtítulos en castellano).
  function badges(f) {
    var out = [], v = (f && f.video) || {};
    if (v.width >= 3800) out.push('4K'); else if (v.height >= 1000 || v.width >= 1900) out.push('1080p');
    if (v.hdr) out.push('HDR');
    var audio = ((f && f.audio) || []).map(function (a) { return YF.lang(a.language); });
    var subs = ((f && f.subtitles) || []).filter(function (s) { return s.text; }).map(function (s) { return YF.lang(s.language); });
    if (audio.length && audio.indexOf('ES') < 0 && subs.indexOf('ES') >= 0) out.push('VOSE');
    return out;
  }

  S.kiosk = function (box, query, lists) {
    details = {}; // el progreso puede haber cambiado (al volver del reproductor)
    var seed = parseInt(query.semilla, 10) || sessionSeed();
    var tipo = TIPOS.filter(function (x) { return x[0] === query.tipo; })[0] || TIPOS[0];
    var filters = YF.readFilters(lists || {});
    var items = [], i = Math.max(0, parseInt(query.i, 10) || 0), slides = [], cur = null, timer = 0;

    function href(over) {
      var o = { tipo: tipo[0] === 'peliculas' ? '' : tipo[0], fs: filters, i: i };
      Object.keys(over || {}).forEach(function (k) { o[k] = over[k]; });
      return YF.route(['kiosco'], YF.filterQuery(o.fs, false, { tipo: o.tipo, semilla: seed, i: o.i || '' }));
    }
    function go(over) { location.replace(href(Object.assign({ i: 0 }, over))); }

    document.body.classList.add('kiosk-on');
    YF.clear(box);
    var stage = el('section', 'kiosk');
    stage.setAttribute('aria-roledescription', 'carrusel');
    stage.setAttribute('aria-label', 'Kiosco');
    var slidesBox = el('div', 'k-slides');
    slidesBox.setAttribute('aria-live', 'polite');
    stage.appendChild(slidesBox);
    box.appendChild(stage);

    // ---- Barra de filtros: Películas/Series/Todo · Sin ver · género · década · pase automático ----
    var bar = el('div', 'k-bar');
    var others = filters.filter(function (f) { return f.k !== 'genero' && f.k !== 'decada' && f.k !== 'estado'; });
    var title = el('h1', 'k-title', others.length ? 'Kiosco de ' + others.map(function (f) { return YF.filterLabel(f.k, f.v); }).join(' · ') : 'Kiosco');
    bar.appendChild(title);
    var pills = el('div', 'chips k-pills');
    function pill(text, on, onClick, cls) {
      var b = el('button', 'chip' + (on ? ' on' : '') + (cls ? ' ' + cls : ''), text);
      b.type = 'button';
      b.setAttribute('aria-pressed', on ? 'true' : 'false');
      b.addEventListener('click', onClick);
      pills.appendChild(b);
      return b;
    }
    TIPOS.forEach(function (x) { pill(x[1], x === tipo, function () { if (x !== tipo) go({ tipo: x[0] === 'peliculas' ? '' : x[0] }); }); });
    var unseen = YF.hasFilter(filters, 'estado', 'unwatched');
    pill('Sin ver', unseen, function () { go({ fs: unseen ? YF.withoutFilter(filters, 'estado') : YF.withFilter(filters, 'estado', 'unwatched') }); });
    others.forEach(function (f) {
      var t = YF.filterLabel(f.k, f.v);
      var b = pill(t, true, function () { go({ fs: YF.withoutFilter(filters, f.k, f.v) }); }, 'pill');
      b.appendChild(el('span', 'x', '✕'));
      b.setAttribute('aria-label', 'Quitar el filtro ' + t);
    });
    function select(k, facet, first) {
      var s = el('select', 'select');
      s.setAttribute('aria-label', first);
      var o = el('option', '', first);
      o.value = '';
      s.appendChild(o);
      var curV = (filters.filter(function (f) { return f.k === k; })[0] || {}).v || '';
      if (curV) { o = el('option', '', YF.filterLabel(k, curV)); o.value = curV; o.selected = true; s.appendChild(o); }
      s.addEventListener('change', function () { go({ fs: s.value ? YF.withFilter(YF.withoutFilter(filters, k), k, s.value) : YF.withoutFilter(filters, k) }); });
      pills.appendChild(s);
      return function (fc) {
        ((fc && fc[facet]) || []).forEach(function (v) {
          if (v.value === curV) return;
          var op = el('option', '', (v.name || v.value) + ' (' + v.count + ')');
          op.value = v.value;
          s.appendChild(op);
        });
      };
    }
    var fillGenre = select('genero', 'genre', 'Todos los géneros');
    var fillDecade = select('decada', 'decade', 'Todas las décadas');
    var autoBtn = pill('Pase automático', false, function () { if (timer) stopAuto(); else startAuto(); }, 'auto');
    bar.appendChild(pills);
    stage.appendChild(bar);

    var nav = el('div', 'k-nav');
    var prevB = el('button', 'k-arrow prev'), nextB = el('button', 'k-arrow next');
    [[prevB, 'Anterior', 'left', -1], [nextB, 'Siguiente', 'right', 1]].forEach(function (x) {
      x[0].type = 'button';
      x[0].setAttribute('aria-label', x[1]);
      x[0].appendChild(YF.icon(x[2]));
      x[0].addEventListener('click', function () { move(x[3]); });
      stage.appendChild(x[0]);
    });
    var counter = el('div', 'k-count');
    nav.appendChild(counter);
    stage.appendChild(nav);

    // ---- Diapositivas ----
    function makeSlide() {
      var s = { el: el('article', 'k-slide'), idx: -1, id: '' };
      s.el.setAttribute('aria-hidden', 'true');
      s.el.inert = true;
      slidesBox.appendChild(s.el);
      return s;
    }

    function fill(s, idx) {
      var t = items[idx];
      s.idx = idx;
      if (s.id === t.id) return;
      s.id = t.id;
      var root = s.el;
      YF.clear(root);
      var bg = el('img', 'k-bg');
      bg.alt = '';
      bg.decoding = 'async';
      bg.onload = function () { bg.classList.add('shown'); };
      bg.onerror = function () { bg.style.visibility = 'hidden'; };
      if (t.backdrop) bg.src = api.url(YF.imgWidth(t.backdrop, 1920)); // la carga ya la precarga
      root.appendChild(bg);
      root.appendChild(el('div', 'k-shade'));
      var body = el('div', 'k-body');
      root.appendChild(body);
      if (t.poster) {
        var p = H.img(YF.imgWidth(t.poster, 400), 'k-poster', 'Cartel de ' + t.title);
        p.loading = 'eager';
        body.appendChild(p);
      }
      var info = el('div', 'k-info');
      body.appendChild(info);
      info.appendChild(el('div', 'eyebrow', t.kind === 'series' ? 'Serie' : 'Película'));
      info.appendChild(el('h2', 'k-name', t.title));
      var meta = el('div', 'meta-line');
      info.appendChild(meta);
      var extra = el('div', 'k-extra');
      info.appendChild(extra);
      var btns = el('div', 'btns');
      var play = H.linkButton('Reproducir', '#/titulo/' + encodeURIComponent(t.id), 'primary k-play', 'play');
      var ficha = H.linkButton('Ver ficha', '#/titulo/' + encodeURIComponent(t.id), 'k-ficha');
      btns.appendChild(play);
      btns.appendChild(ficha);
      info.appendChild(btns);
      s.play = play;
      s.ficha = ficha;
      metaText(meta, t, null, null);
      detail(t.id).then(function (d) {
        if (s.id !== t.id) return;
        var m = d.title.meta || {}, links = d.links || {}, files = d.files || [];
        var f = files.length ? P.chooseFile(files) : null;
        metaText(meta, t, m, links);
        if ((m.genres || t.genres || []).length) extra.appendChild(el('div', 'k-genres', (m.genres || t.genres).join(' · ')));
        var ppl = el('div', 'people');
        if (m.director && m.director.length) { ppl.appendChild(document.createTextNode(t.kind === 'series' ? 'Creada por ' : 'Dirección ')); ppl.appendChild(el('b', '', m.director.join(', '))); }
        if (m.cast && m.cast.length) { ppl.appendChild(document.createTextNode((ppl.childNodes.length ? '  ·  ' : '') + 'Reparto ')); ppl.appendChild(el('b', '', m.cast.slice(0, 5).join(', '))); }
        if (ppl.childNodes.length) extra.appendChild(ppl);
        if (m.tagline) extra.appendChild(el('div', 'tagline', m.tagline));
        if (m.synopsis) extra.appendChild(el('div', 'synopsis', m.synopsis));
        var bs = badges(f);
        if (bs.length) {
          var b = el('div', 'badges');
          bs.forEach(function (x, k) { b.appendChild(el('span', 'badge' + (k === 0 ? ' gold' : ''), x)); });
          extra.appendChild(b);
        }
        var at = P.resumeAt(f);
        if (at) {
          var pr = el('div', 'k-progress');
          var rail = el('div', 'rail'), done = el('div', 'done');
          done.style.width = Math.min(100, at / (f.duration || 1) * 100) + '%';
          rail.appendChild(done);
          pr.appendChild(rail);
          pr.appendChild(el('span', '', (f.season != null ? YF.epLabel(f) + ' · ' : '') + 'quedan ' + YF.clock(f.duration - at)));
          extra.appendChild(pr);
          play.lastChild.textContent = 'Reanudar';
        }
        if (f) play.href = H.playHref(d.title, f, null);
        if (!t.backdrop && d.backdrop) bg.src = api.url(YF.imgWidth(d.backdrop, 1920));
      }).catch(function () {});
    }

    // Año · duración · ★ nota · país
    function metaText(line, t, m, links) {
      YF.clear(line);
      var bits = [];
      if (t.year) bits.push(t.year);
      if (m && m.runtime) bits.push(YF.runtime(m.runtime));
      var r = (m && m.rating) || t.rating;
      if (r) bits.push('★ ' + Number(r).toFixed(1).replace('.', ','));
      var c = links && links.country && links.country.length ? links.country.map(function (x) { return x.name; }) : [];
      if (c.length) bits.push(c.slice(0, 2).join(', '));
      line.textContent = bits.join(' · ');
    }

    // Coloca las tres diapositivas alrededor de i, reutilizando las que ya tienen el título.
    function arrange() {
      var n = items.length, need = [YF.wrapIndex(i - 1, n), i, YF.wrapIndex(i + 1, n)], free = slides.slice(), got = [];
      need.forEach(function (ix, k) {
        var m = free.filter(function (s) { return s.idx === ix; })[0];
        if (m) { got[k] = m; free.splice(free.indexOf(m), 1); }
      });
      need.forEach(function (ix, k) { if (!got[k]) { got[k] = free.shift(); fill(got[k], ix); } });
      got.forEach(function (s, k) {
        s.el.classList.toggle('cur', k === 1);
        s.el.setAttribute('aria-hidden', k === 1 ? 'false' : 'true');
        s.el.inert = k !== 1;
      });
      cur = got[1];
      counter.textContent = (i + 1) + ' / ' + n;
      history.replaceState(null, '', href());
    }

    function move(d, auto) {
      if (!items.length) return;
      if (!auto) stopAuto();
      var hadFocus = slidesBox.contains(document.activeElement) || document.activeElement === document.body || !document.activeElement;
      i = YF.wrapIndex(i + d, items.length);
      arrange();
      if (hadFocus && cur.play) { try { cur.play.focus({ preventScroll: true }); } catch (_) {} }
    }

    function startAuto() {
      stopAuto();
      timer = setInterval(function () { move(1, true); }, AUTO_MS);
      autoBtn.classList.add('on');
      autoBtn.setAttribute('aria-pressed', 'true');
    }
    function stopAuto() {
      if (!timer) return;
      clearInterval(timer);
      timer = 0;
      autoBtn.classList.remove('on');
      autoBtn.setAttribute('aria-pressed', 'false');
    }

    // Teclado: ◀▶ cambian de título, ▲▼ entre Reproducir y Ver ficha, Intro reproduce. Cualquier tecla para el pase.
    function onKey(e) {
      if (YF.player.active || (e.target === autoBtn && (e.key === 'Enter' || e.key === ' '))) return; // eso lo pulsa
      stopAuto();
      var tag = (e.target && e.target.tagName) || '';
      if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA' || e.altKey || e.ctrlKey || e.metaKey) return;
      if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') { e.preventDefault(); move(e.key === 'ArrowLeft' ? -1 : 1); return; }
      if (!cur || !cur.play) return;
      if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        e.preventDefault();
        (document.activeElement === cur.play ? cur.ficha : cur.play).focus();
      } else if (e.key === 'Enter' && (document.activeElement === document.body || !document.activeElement)) {
        e.preventDefault();
        cur.play.click();
      }
    }
    function onPointer(e) { if (e.target !== autoBtn && !autoBtn.contains(e.target)) stopAuto(); }
    document.addEventListener('keydown', onKey);
    document.addEventListener('pointerdown', onPointer, true);

    // Deslizar en el móvil.
    var sx = 0, sy = 0, st = 0;
    slidesBox.addEventListener('touchstart', function (e) { var p = e.touches[0]; sx = p.clientX; sy = p.clientY; st = Date.now(); }, { passive: true });
    slidesBox.addEventListener('touchend', function (e) {
      var p = e.changedTouches[0], dx = p.clientX - sx, dy = p.clientY - sy;
      if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5 && Date.now() - st < 800) move(dx < 0 ? 1 : -1);
    }, { passive: true });

    S.cleanup = function () {
      stopAuto();
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('pointerdown', onPointer, true);
      document.body.classList.remove('kiosk-on');
    };

    api.facets({ kind: tipo[2], only: 'genre,decade' }).then(function (r) { fillGenre(r.facets); fillDecade(r.facets); }).catch(function () {});

    slidesBox.appendChild(el('div', 'k-wait spinner'));
    return api.titles(YF.filterQuery(filters, true, { kind: tipo[2], sort: 'random', seed: seed, limit: LIMIT })).then(function (r) {
      YF.clear(slidesBox);
      items = r.items || [];
      if (!items.length) {
        slidesBox.appendChild(el('div', 'empty', 'No hay títulos con este filtro.'));
        counter.textContent = '';
        prevB.disabled = nextB.disabled = true;
        history.replaceState(null, '', href({ i: 0 }));
        return;
      }
      if (i >= items.length) i = 0;
      slides = [makeSlide(), makeSlide(), makeSlide()];
      arrange();
      document.title = title.textContent + ' · YourFlix';
      if (window.matchMedia('(hover: hover)').matches) { try { cur.play.focus({ preventScroll: true }); } catch (_) {} }
    }).catch(function (e) { H.errorBox(slidesBox, 'No se pudo cargar el kiosco (' + e.message + ').', function () { location.reload(); }); });
  };
})(window.YF = window.YF || {});
