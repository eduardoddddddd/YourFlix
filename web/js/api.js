/* Cliente de /api/v1 (docs/API.md). La web la sirve el mismo servidor, así que basta con rutas
 * relativas; ?server=IP:8787 (con http) sirve para probarla desde otro servidor estático. */
(function (YF) {
  'use strict';
  var m = /[?&]server=([^&#]+)/.exec(location.search);
  var api = YF.api = { base: m ? decodeURIComponent(m[1]).replace(/\/+$/, '') : '' };

  function request(method, path, body, opts) {
    var init = { method: method, headers: {} };
    if (body != null) { init.headers['Content-Type'] = 'application/json'; init.body = JSON.stringify(body); }
    if (opts && opts.keepalive) init.keepalive = true;
    return fetch(api.base + '/api/v1' + path, init).then(function (r) {
      if (r.status === 204) return null;
      if (!r.ok) return r.text().then(function (t) { var e = new Error(t.trim() || 'HTTP ' + r.status); e.status = r.status; throw e; });
      return r.json();
    });
  }

  api.get = function (path) { return request('GET', path); };
  api.url = function (path) { return path ? api.base + path : ''; };

  api.titles = function (q) {
    var parts = [];
    Object.keys(q || {}).forEach(function (k) { if (q[k] !== '' && q[k] != null) parts.push(k + '=' + encodeURIComponent(q[k])); });
    return api.get('/titles' + (parts.length ? '?' + parts.join('&') : ''));
  };
  api.title = function (id) { return api.get('/titles/' + encodeURIComponent(id)); };
  api.subtitles = function (fileId) { return api.get('/files/' + encodeURIComponent(fileId) + '/subtitles'); };
  api.vttURL = function (fileId, sid) { return api.base + '/api/v1/files/' + encodeURIComponent(fileId) + '/vtt/' + encodeURIComponent(sid); };
  api.streamURL = function (fileId) { return api.base + '/api/v1/files/' + encodeURIComponent(fileId) + '/stream'; };

  // Progreso compartido con la TV («Seguir viendo» es el mismo). keepalive: también al cerrar la pestaña.
  api.progress = function (fileId, position, duration, keepalive) {
    return request('PUT', '/progress/' + encodeURIComponent(fileId), { position: Math.floor(position), duration: Math.floor(duration) }, { keepalive: keepalive })
      .catch(function () {});
  };

  // Estado de la conversión en vivo (se pide una vez).
  var remux = null;
  api.remux = function () {
    if (!remux) remux = api.get('/remux').catch(function () { return { available: false, videoEncoder: '' }; });
    return remux;
  };

  // Para la conversión de este reproductor (al salir o cambiar de pista).
  api.stopHLS = function (base, session) {
    var url = api.base + base + 'stop?s=' + encodeURIComponent(session);
    try { if (navigator.sendBeacon && navigator.sendBeacon(url)) return; } catch (_) {}
    fetch(url, { method: 'POST', keepalive: true }).catch(function () {});
  };
})(window.YF = window.YF || {});
