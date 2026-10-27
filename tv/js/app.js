/* Arranque, pila de pantallas, menú, teclas del mando, diálogos y avisos. */
(function (YF) {
  'use strict';
  var el = YF.el, $ = YF.$, nav = YF.nav;
  YF.version = '0.4.0';

  var app = YF.app = { stack: [], backAt: 0 };
  var screensEl = $('screens'), menu = $('menu');

  // ---------- Pantallas ----------
  function current() { return app.stack[app.stack.length - 1]; }

  // Desplazamientos de una pantalla: Chromium los pierde al ocultarla con display:none (tema 1).
  var SCROLLERS = '.scroller, .track, .links .lrow';
  function saveScroll(x) {
    x._scroll = Array.prototype.map.call(x.el.querySelectorAll(SCROLLERS), function (e) { return [e, e.scrollTop, e.scrollLeft]; });
  }
  function restoreScroll(x) {
    (x._scroll || []).forEach(function (v) { if (v[0].isConnected) { v[0].scrollTop = v[1]; v[0].scrollLeft = v[2]; } });
    x._scroll = null;
  }

  // activate: muestra s (y, si s es un panel superpuesto, también la de debajo).
  function activate(s, focusEl) {
    var under = s.overlay ? app.stack[app.stack.length - 2] : null;
    app.stack.forEach(function (x) {
      var show = x === s || x === under, was = !x.el.classList.contains('hidden');
      if (was && !show) saveScroll(x);
      x.el.classList.toggle('hidden', !show);
      if (show && !was) restoreScroll(x);
    });
    $('app').classList.remove('fullscreen');
    nav.setRoots(s.overlay ? [s.root] : [menu, s.root]);
    var items = menu.querySelectorAll('.menu-item');
    for (var i = 0; i < items.length; i++) items[i].classList.toggle('active', items[i].dataset.go === app.stack[0].name);
    if (focusEl) nav.focus(focusEl);
    if (s.onShow) s.onShow();
  }

  // open: añade una pantalla encima (Atrás vuelve a la anterior, con su foco).
  app.open = function (s) {
    var cur = current();
    if (cur) cur.last = nav.current;
    if (app.stack.length >= 5) { var old = app.stack.splice(1, 1)[0]; screensEl.removeChild(old.el); } // limitar memoria
    app.stack.push(s);
    screensEl.appendChild(s.el);
    activate(s);
    var loading = el('div', 'loading', 'Cargando…');
    s.el.appendChild(loading);
    (s.refresh ? s.refresh() : Promise.resolve(null)).then(function (first) {
      if (loading.parentNode) loading.parentNode.removeChild(loading);
      if (current() === s && first && (!nav.current || !s.el.contains(nav.current))) nav.focus(first);
    }).catch(function (e) {
      if (loading.parentNode) loading.textContent = 'No se pudo cargar: ' + e.message;
    });
  };

  // go: sección del menú (vacía la pila).
  app.go = function (name) {
    if (app.stack.length && app.stack[0].name === name && app.stack.length === 1) { if (current().onShow) current().onShow(); return; }
    app.stack.forEach(function (x) { screensEl.removeChild(x.el); });
    app.stack = [];
    var s = name === 'movies' ? YF.screens.grid('movie') : name === 'series' ? YF.screens.grid('series') : YF.screens[name]();
    app.open(s);
  };

  // Tras ver algo, las pantallas de debajo («Seguir viendo», progreso) se recargan al volver.
  // Las que tienen onReturn(dirty) deciden ellas qué refrescar (la lista no se recarga entera).
  app.markDirty = function () { app.stack.forEach(function (x) { x.dirty = true; }); };

  app.back = function () {
    if (app.stack.length > 1) {
      var s = app.stack.pop();
      screensEl.removeChild(s.el);
      var prev = current();
      activate(prev, prev.last);
      if (prev.onReturn) { var d = prev.dirty; prev.dirty = false; prev.onReturn(d); return; }
      if (prev.dirty && prev.refresh) {
        prev.dirty = false;
        prev.refresh().then(function (first) {
          if (current() === prev && (!nav.current || !document.body.contains(nav.current))) nav.focus(first);
        });
      }
      return;
    }
    // En la raíz: primero al menú; con el foco en el menú, dos veces Atrás para salir.
    if (!menu.contains(nav.current)) { nav.focus(menu.querySelector('.menu-item.active') || menu.querySelector('.menu-item')); return; }
    if (Date.now() - app.backAt < 2000) { try { tizen.application.getCurrentApplication().exit(); } catch (_) {} return; }
    app.backAt = Date.now();
    YF.toast('Pulsa Atrás otra vez para salir');
  };

  // Tras ver algo: refrescar Inicio (Seguir viendo) y la pantalla visible.
  app.current = current;

  app.refreshAll = function () {
    var s = current();
    if (s && s.refresh) { s.dirty = false; s.refresh().then(function (first) { if (!nav.current || !document.body.contains(nav.current)) nav.focus(first); }); }
  };

  // ---------- Diálogo y aviso ----------
  var dialogPrev = null;
  YF.dialog = function (text, buttons, extra) {
    var d = $('dialog'), box = $('dialogButtons');
    $('dialogText').textContent = text;
    YF.clear(box);
    var card = d.querySelector('.dialog-card');
    Array.prototype.slice.call(card.querySelectorAll('.extra')).forEach(function (x) { x.parentNode.removeChild(x); });
    (extra || []).forEach(function (x) { x.classList.add('extra'); card.insertBefore(x, box); });
    var firstBtn = null;
    buttons.forEach(function (b) {
      var e = YF.f(el('span', 'btn' + (b.primary ? ' primary' : ''), b.label), function () { closeDialog(); if (b.action) b.action(); });
      box.appendChild(e);
      if (!firstBtn) firstBtn = e;
    });
    dialogPrev = { roots: nav.roots, focus: nav.current };
    d.classList.remove('hidden');
    nav.setRoots([d]);
    nav.focus((extra && extra.length && d.querySelector('.extra .f')) || firstBtn);
  };
  function closeDialog() {
    $('dialog').classList.add('hidden');
    if (dialogPrev) { nav.setRoots(dialogPrev.roots); nav.focus(dialogPrev.focus, true); dialogPrev = null; }
  }
  function dialogOpen() { return !$('dialog').classList.contains('hidden'); }

  var toastTimer = 0;
  YF.toast = function (text) {
    var t = $('toast');
    t.textContent = text;
    t.classList.remove('hidden');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { t.classList.add('hidden'); }, 3000);
  };

  // ---------- Teclas ----------
  var KEYS = { 37: 'Left', 38: 'Up', 39: 'Right', 40: 'Down', 13: 'Enter', 10009: 'Back', 27: 'Back', 8: 'Back',
    415: 'Play', 19: 'Pause', 10252: 'PlayPause', 413: 'Stop', 417: 'FastForward', 412: 'Rewind',
    403: 'Red', 404: 'Green', 405: 'Yellow', 406: 'Blue', 457: 'Info', 427: 'ChannelUp', 428: 'ChannelDown',
    // navegador del Mac, para probar sin mando
    71: 'Green', 89: 'Yellow', 66: 'Blue', 70: 'FastForward', 82: 'Rewind', 32: 'PlayPause' };
  var REGISTER = ['MediaPlayPause', 'MediaPlay', 'MediaPause', 'MediaStop', 'MediaFastForward', 'MediaRewind',
    'ColorF0Red', 'ColorF1Green', 'ColorF2Yellow', 'ColorF3Blue', 'Info', 'ChannelUp', 'ChannelDown'];

  function registerKeys() {
    if (!window.tizen || !tizen.tvinputdevice) return;
    REGISTER.forEach(function (k) { try { tizen.tvinputdevice.registerKey(k); } catch (_) {} }); // una a una: registerKeyBatch falla entera si falta una
  }

  document.addEventListener('keydown', function (e) {
    var k = KEYS[e.keyCode];
    if (!k) return;
    e.preventDefault();
    if (YF.player.active) { YF.player.key(k); return; }
    if (dialogOpen()) {
      if (k === 'Back') closeDialog();
      else if (k === 'Enter') nav.enter();
      else if (k === 'Left' || k === 'Right' || k === 'Up' || k === 'Down') nav.move(k.toLowerCase());
      return;
    }
    // Pantallas con teclas propias (kiosco, panel «Filtrar por»): si la usan, no sigue.
    var cur = current();
    if (cur && cur.key && cur.key(k)) return;
    switch (k) {
      case 'Left': case 'Right': case 'Up': case 'Down': nav.move(k.toLowerCase()); break;
      case 'Enter': nav.enter(); break;
      case 'Back': app.back(); break;
      case 'ChannelDown': for (var i = 0; i < 4; i++) nav.move('down'); break;
      case 'ChannelUp': for (var j = 0; j < 4; j++) nav.move('up'); break;
    }
  });

  // ---------- Arranque ----------
  function splash(text) {
    YF.clear(screensEl);
    app.stack = [];
    var s = el('div', 'screen');
    s.appendChild(el('div', 'loading', text));
    screensEl.appendChild(s);
    return s;
  }

  app.boot = function () {
    var s = splash('Buscando el servidor…');
    nav.setRoots([menu]);
    YF.api.discover(function (msg) { s.firstChild.textContent = msg; }).then(function (r) {
      if (!r) {
        s.firstChild.textContent = 'No encuentro el servidor YourFlix en la red. Comprueba que está encendido.';
        var retry = YF.f(el('span', 'btn primary', 'Reintentar'), app.boot);
        var manual = YF.f(el('span', 'btn', 'Escribir dirección IP'), function () { YF.screens.ipDialog(); });
        var box = el('div', 'loading');
        box.style.top = '560px';
        box.appendChild(retry); box.appendChild(manual);
        s.appendChild(box);
        nav.setRoots([s]);
        nav.focus(retry);
        return;
      }
      screensEl.removeChild(s);
      app.go('home');
    });
  };

  // Menú
  var items = menu.querySelectorAll('.menu-item');
  for (var i = 0; i < items.length; i++) {
    (function (it) { YF.f(it, function () { app.go(it.dataset.go); }); })(items[i]);
  }

  function clock() { var d = new Date(); $('clock').textContent = (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes(); }
  setInterval(clock, 15000); clock();

  registerKeys();
  app.boot();
})(window.YF = window.YF || {});
