/* Utilidades sin dependencias. En el navegador cuelgan de window.YF; en Node (tests), de module.exports. */
(function (YF) {
  'use strict';

  // Crea un elemento. Los textos van siempre por textContent, nunca como HTML.
  YF.el = function (tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  };

  YF.clear = function (e) { while (e.firstChild) e.removeChild(e.firstChild); return e; };

  // 6863 → «1:54:23»; 300 → «5:00».
  YF.clock = function (sec) {
    if (!(sec >= 0) || sec === Infinity) return '--:--';
    sec = Math.floor(sec);
    var h = Math.floor(sec / 3600), m = Math.floor(sec / 60) % 60, s = sec % 60;
    return (h ? h + ':' + (m < 10 ? '0' : '') : '') + m + ':' + (s < 10 ? '0' : '') + s;
  };

  // 116 → «1 h 56 min».
  YF.runtime = function (min) {
    if (!(min > 0)) return '';
    var h = Math.floor(min / 60), m = min % 60;
    return (h ? h + ' h ' : '') + (m ? m + ' min' : '');
  };

  YF.bytes = function (n) { return n >= 1e9 ? (n / 1e9).toFixed(1).replace('.', ',') + ' GB' : Math.round(n / 1e6) + ' MB'; };

  var LANG = { spa: 'ES', es: 'ES', eng: 'EN', en: 'EN', fre: 'FR', fra: 'FR', fr: 'FR', ita: 'IT', it: 'IT', ger: 'DE', deu: 'DE', de: 'DE',
    jpn: 'JA', ja: 'JA', kor: 'KO', ko: 'KO', por: 'PT', pt: 'PT', cat: 'CA', ca: 'CA', eus: 'EU', baq: 'EU', glg: 'GL', chi: 'ZH', zho: 'ZH', rus: 'RU' };
  YF.lang = function (code) { return LANG[(code || '').toLowerCase()] || (code ? code.toUpperCase() : '?'); };

  var LANG_NAME = { ES: 'Español', EN: 'Inglés', FR: 'Francés', IT: 'Italiano', DE: 'Alemán', JA: 'Japonés', KO: 'Coreano', PT: 'Portugués',
    CA: 'Catalán', EU: 'Euskera', GL: 'Gallego', ZH: 'Chino', RU: 'Ruso' };
  YF.langName = function (code) { var l = YF.lang(code); return LANG_NAME[l] || (code ? l : 'Desconocido'); };

  // Distintivos técnicos de un archivo de la API: 4K, HDR, HEVC 10 bits, 5.1, audios, subtítulos.
  YF.techBadges = function (f) {
    var out = [], v = f.video || {}, audio = f.audio || [], subs = f.subtitles || [];
    if (v.width >= 3800) out.push('4K'); else if (v.height >= 1000 || v.width >= 1900) out.push('1080p'); else if (v.height >= 700) out.push('720p'); else if (v.height) out.push('SD');
    if (v.hdr) out.push(v.hdr);
    if (v.codec) out.push(v.codec.toUpperCase().replace('MPEG4', 'XviD') + (v.bitDepth >= 10 ? ' 10 bits' : ''));
    var maxCh = 0, langs = [];
    audio.forEach(function (a) { maxCh = Math.max(maxCh, a.channels || 0); var l = YF.lang(a.language); if (langs.indexOf(l) < 0) langs.push(l); });
    if (maxCh >= 6) out.push(maxCh >= 8 ? '7.1' : '5.1');
    if (langs.length) out.push('Audio ' + langs.join('·'));
    var sl = [];
    subs.forEach(function (s) { if (s.text) { var l = YF.lang(s.language); if (sl.indexOf(l) < 0) sl.push(l); } });
    if (sl.length) out.push('Sub ' + sl.join('·'));
    return out;
  };

  YF.epLabel = function (f) { return f && f.season != null ? f.season + '×' + (f.episode < 10 ? '0' : '') + f.episode : ''; };

  YF.debounce = function (fn, ms) {
    var t = 0;
    return function () { var a = arguments, self = this; clearTimeout(t); t = setTimeout(function () { fn.apply(self, a); }, ms); };
  };

  // Rutas con almohadilla: «#/titulo/abc?x=1» → {parts: ['titulo', 'abc'], query: {x: '1'}}.
  YF.parseRoute = function (hash) {
    var h = String(hash || '').replace(/^#\/?/, '');
    var qi = h.indexOf('?'), path = qi < 0 ? h : h.slice(0, qi), qs = qi < 0 ? '' : h.slice(qi + 1);
    var parts = path.split('/').filter(function (p) { return p; }).map(function (p) { try { return decodeURIComponent(p); } catch (_) { return p; } });
    var query = {};
    qs.split('&').forEach(function (kv) {
      if (!kv) return;
      var i = kv.indexOf('='), k = i < 0 ? kv : kv.slice(0, i), v = i < 0 ? '' : kv.slice(i + 1);
      try { query[decodeURIComponent(k)] = decodeURIComponent(v.replace(/\+/g, ' ')); } catch (_) {}
    });
    return { parts: parts, query: query };
  };

  YF.route = function (parts, query) {
    var h = '#/' + parts.map(encodeURIComponent).join('/');
    var q = Object.keys(query || {}).filter(function (k) { return query[k] !== '' && query[k] != null; })
      .map(function (k) { return encodeURIComponent(k) + '=' + encodeURIComponent(query[k]); });
    return h + (q.length ? '?' + q.join('&') : '');
  };

  // Cambia el ancho de una URL de imagen de la API («…/poster?w=300» → «…?w=600»).
  YF.imgWidth = function (url, w) { return url ? url.replace(/([?&])w=\d+/, '$1w=' + w) : ''; };

  YF.storage = {
    get: function (k, def) { try { var v = localStorage.getItem('yourflix.web.' + k); return v == null ? def : JSON.parse(v); } catch (_) { return def; } },
    set: function (k, v) { try { localStorage.setItem('yourflix.web.' + k, JSON.stringify(v)); } catch (_) {} }
  };
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
