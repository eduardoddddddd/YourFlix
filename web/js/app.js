/* Arranque y rutas (#/…): Inicio, Películas, Series, Kiosco, Buscar, Ficha y Reproductor. */
(function (YF) {
  'use strict';
  var S = YF.screens, main = document.getElementById('main');
  var app = YF.app = { depth: 0 };
  var scrolls = {}, current = '', caps = null;

  // Capacidades del navegador + si el servidor recodifica vídeo (se calcula una vez).
  app.caps = function () {
    if (!caps) caps = YF.api.remux().then(function (s) {
      var c = YF.playback.detectCaps(document.createElement('video'), window.Hls, s.available && !!s.videoEncoder);
      if (!s.available) { c.hls = ''; c.remuxOff = true; } // --remux-jobs 0: solo reproducción directa
      return c;
    });
    return caps;
  };

  var NAV = { '': 'home', peliculas: 'movies', series: 'series', kiosco: 'kiosk', buscar: 'search', titulo: '', ver: '' };

  function render() {
    var r = YF.parseRoute(location.hash), name = r.parts[0] || '';
    if (!(name in NAV)) { location.replace('#/'); return; }
    if (name === 'ver' && r.parts.length >= 3) {
      // El reproductor va encima; si se entra directamente, debajo queda la ficha.
      if (!main.firstChild) show('#/titulo/' + encodeURIComponent(r.parts[1]), function () { return S.detail(main, r.parts[1], {}); });
      YF.player.open(r.parts[1], r.parts[2], r.query.t != null ? Math.max(0, +r.query.t || 0) : null);
      return;
    }
    var fromPlayer = YF.player.active;
    YF.player.close();
    var key = location.hash || '#/';
    if (key === current && !fromPlayer) return; // al volver del reproductor se repinta (progreso nuevo)
    show(key, function () {
      switch (name) {
        case 'peliculas': return S.grid(main, 'movie', r.query, r.lists);
        case 'series': return S.grid(main, 'series', r.query, r.lists);
        case 'kiosco': return S.kiosk(main, r.query, r.lists);
        case 'buscar': return S.search(main, r.query);
        case 'titulo': return S.detail(main, r.parts[1], r.query);
        default: return S.home(main);
      }
    });
    Array.prototype.forEach.call(document.querySelectorAll('#nav a'), function (a) {
      a.classList.toggle('active', a.dataset.nav === NAV[name]);
      if (a.dataset.nav === NAV[name]) a.setAttribute('aria-current', 'page'); else a.removeAttribute('aria-current');
    });
  }

  function show(key, draw) {
    if (current) scrolls[current] = window.scrollY;
    if (S.cleanup) { S.cleanup(); S.cleanup = null; }
    current = key;
    document.title = 'YourFlix';
    window.scrollTo(0, 0);
    var p = draw() || Promise.resolve();
    // true = la pantalla ya se ha situado sola (la lista, en la tarjeta de la que se salió).
    p.then(function (placed) { if (current === key && scrolls[key] && placed !== true) window.scrollTo(0, scrolls[key]); });
  }

  // Volver desde el reproductor: atrás en el historial si venimos de la web; si no, a la ficha.
  app.leavePlayer = function () {
    var r = YF.parseRoute(location.hash);
    if (app.depth > 0) history.back();
    else location.replace('#/titulo/' + encodeURIComponent(r.parts[1] || ''));
  };

  window.addEventListener('hashchange', function () { app.depth++; render(); });
  window.addEventListener('scroll', function () { document.getElementById('top').classList.toggle('solid', window.scrollY > 30); }, { passive: true });
  render();
})(window.YF = window.YF || {});
