/* Cliente de la API /api/v1 (docs/API.md) y descubrimiento del servidor en la red local. */
(function (YF) {
  'use strict';

  var PORT = 8787;
  var DEFAULTS = ['http://192.168.1.3:' + PORT]; // Mac; el ROG Ally se descubre o se guarda
  var api = YF.api = { base: '' };

  function request(method, url, body, timeout) {
    return new Promise(function (resolve, reject) {
      var x = new XMLHttpRequest();
      x.open(method, url);
      x.timeout = timeout || 10000;
      if (body != null) x.setRequestHeader('Content-Type', 'application/json');
      x.onload = function () {
        if (x.status === 204) return resolve(null);
        if (x.status < 200 || x.status >= 300) return reject(new Error('HTTP ' + x.status));
        try { resolve(JSON.parse(x.responseText)); } catch (e) { reject(e); }
      };
      x.onerror = function () { reject(new Error('sin conexión')); };
      x.ontimeout = function () { reject(new Error('tiempo agotado')); };
      x.send(body != null ? JSON.stringify(body) : null);
    });
  }

  api.get = function (path) { return request('GET', api.base + '/api/v1' + path); };
  api.put = function (path, body) { return request('PUT', api.base + '/api/v1' + path, body); };
  api.url = function (path) { return path ? api.base + path : ''; };

  api.titles = function (q) {
    var parts = [];
    Object.keys(q || {}).forEach(function (k) { if (q[k] !== '' && q[k] != null) parts.push(k + '=' + encodeURIComponent(q[k])); });
    return api.get('/titles' + (parts.length ? '?' + parts.join('&') : ''));
  };
  api.title = function (id) { return api.get('/titles/' + encodeURIComponent(id)); };
  api.progress = function (fileId, position, duration) {
    return api.put('/progress/' + encodeURIComponent(fileId), { position: position, duration: duration }).catch(function () {});
  };

  function ping(base, timeout) {
    return request('GET', base + '/api/v1/ping', null, timeout).then(function (r) {
      if (!r || r.name !== 'YourFlix') throw new Error('no es YourFlix');
      return { base: base, info: r };
    });
  }

  // Candidatos de una /24: todas las direcciones salvo la propia, en paralelo por tandas.
  api.subnetCandidates = function (ip) {
    var m = /^(\d+\.\d+\.\d+)\.(\d+)$/.exec(ip || '');
    if (!m) return [];
    var out = [];
    for (var i = 1; i < 255; i++) if (String(i) !== m[2]) out.push('http://' + m[1] + '.' + i + ':' + PORT);
    return out;
  };

  function firstOf(bases, timeout) {
    return new Promise(function (resolve) {
      var left = bases.length, done = false;
      if (!left) return resolve(null);
      bases.forEach(function (b) {
        ping(b, timeout).then(function (r) { if (!done) { done = true; resolve(r); } }, function () {
          if (--left === 0 && !done) resolve(null);
        });
      });
    });
  }

  function tvIP() {
    try { return window.webapis && webapis.network && webapis.network.getIp(); } catch (_) { return ''; }
  }

  // Orden: ?server= (navegador) → guardado → por defecto → barrido de la red de la TV.
  api.discover = function (onStep) {
    var step = onStep || function () {};
    var q = /[?&]server=([^&]+)/.exec(location.search);
    var saved = YF.storage.get('server', '');
    var known = [];
    if (q) known.push(decodeURIComponent(q[1]));
    if (saved) known.push(saved);
    if (location.protocol.indexOf('http') === 0) known.push(location.origin);
    DEFAULTS.forEach(function (d) { if (known.indexOf(d) < 0) known.push(d); });
    step('Buscando el servidor…');
    return firstOf(known, 3000).then(function (r) {
      if (r) return r;
      var ip = tvIP(), cands = api.subnetCandidates(ip);
      if (!cands.length) return null;
      step('Buscando el servidor en la red ' + ip.replace(/\.\d+$/, '.x') + '…');
      var batches = [];
      for (var i = 0; i < cands.length; i += 64) batches.push(cands.slice(i, i + 64));
      return batches.reduce(function (p, batch) {
        return p.then(function (found) { return found || firstOf(batch, 1500); });
      }, Promise.resolve(null));
    }).then(function (r) {
      if (r) { api.base = r.base; api.info = r.info; YF.storage.set('server', r.base); }
      return r;
    });
  };

  api.use = function (base) {
    return ping(base, 4000).then(function (r) { api.base = r.base; api.info = r.info; YF.storage.set('server', r.base); return r; });
  };
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
