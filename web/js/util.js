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
    var query = {}, lists = {};
    qs.split('&').forEach(function (kv) {
      if (!kv) return;
      var i = kv.indexOf('='), k = i < 0 ? kv : kv.slice(0, i), v = i < 0 ? '' : kv.slice(i + 1);
      try {
        k = decodeURIComponent(k); v = decodeURIComponent(v.replace(/\+/g, ' '));
        query[k] = v;
        (lists[k] = lists[k] || []).push(v); // las claves repetidas (actor=A&actor=B) se conservan aquí
      } catch (_) {}
    });
    return { parts: parts, query: query, lists: lists };
  };

  // Un valor de lista se escribe como clave repetida.
  YF.route = function (parts, query) {
    var h = '#/' + parts.map(encodeURIComponent).join('/'), q = [];
    Object.keys(query || {}).forEach(function (k) {
      [].concat(query[k]).forEach(function (v) {
        if (v !== '' && v != null) q.push(encodeURIComponent(k) + '=' + encodeURIComponent(v));
      });
    });
    return h + (q.length ? '?' + q.join('&') : '');
  };

  // ---------- Filtros de las listas (en la dirección, en castellano) ----------
  // [clave en la dirección, parámetro de /titles y /facets, se puede repetir (Y)]
  var FILTERS = [['genero', 'genre', 1], ['director', 'director', 1], ['actor', 'actor', 1], ['persona', 'person', 1],
    ['pais', 'country', 1], ['decada', 'decade', 0], ['anio', 'year', 0], ['duracion', 'runtime', 0], ['nota', 'rating', 0],
    ['calidad', 'quality', 1], ['audio', 'audio', 1], ['subs', 'subs', 1], ['estado', 'state', 0]];
  var BY_URL = {}, BY_API = {};
  FILTERS.forEach(function (f) { BY_URL[f[0]] = f; BY_API[f[1]] = f; });
  YF.filterKey = function (apiKey) { return BY_API[apiKey] ? BY_API[apiKey][0] : ''; };
  YF.filterApiKey = function (k) { return BY_URL[k] ? BY_URL[k][1] : ''; };

  // lists de parseRoute → [{k: 'director', v: 'Ridley Scott'}, …] (claves desconocidas fuera, sin repetidos).
  YF.readFilters = function (lists) {
    var out = [];
    FILTERS.forEach(function (f) {
      var vs = (lists && lists[f[0]]) || [];
      if (!f[2]) vs = vs.length ? [vs[vs.length - 1]] : [];
      vs.forEach(function (v) {
        v = String(v).trim();
        if (v && !out.some(function (x) { return x.k === f[0] && x.v === v; })) out.push({ k: f[0], v: v });
      });
    });
    return out;
  };

  // Para la dirección ({director: ['Ridley Scott'], decada: ['1980']}) o para la API ({decade: ['1980']}).
  YF.filterQuery = function (filters, forApi, base) {
    var q = {};
    Object.keys(base || {}).forEach(function (k) { q[k] = base[k]; });
    (filters || []).forEach(function (f) {
      var k = forApi ? YF.filterApiKey(f.k) : f.k;
      if (k) (q[k] = Array.isArray(q[k]) ? q[k] : []).push(f.v);
    });
    return q;
  };

  // Suma un filtro: las claves que admiten lista se acumulan; las demás se sustituyen.
  YF.withFilter = function (filters, k, v) {
    v = String(v);
    var multi = BY_URL[k] && BY_URL[k][2];
    var out = (filters || []).filter(function (f) { return f.k !== k || (multi && f.v !== v); });
    out.push({ k: k, v: v });
    return out;
  };
  YF.withoutFilter = function (filters, k, v) {
    return (filters || []).filter(function (f) { return !(f.k === k && (v == null || f.v === String(v))); });
  };
  YF.hasFilter = function (filters, k, v) { return (filters || []).some(function (f) { return f.k === k && f.v === String(v); }); };

  var RUNTIME = { 'short': 'Menos de 90 min', mid: 'De 90 a 120 min', 'long': 'Más de 120 min' };
  var QUALITY = { '4k': '4K', hdr: 'HDR', '1080': '1080p', '720': '720p', sd: 'SD' };
  var STATE = { unwatched: 'Sin ver', partial: 'A medias', watched: 'Vistas' };
  YF.decadeName = function (d) { d = String(d); return 'Años ' + (+d < 2000 && d.length === 4 ? d.slice(2) : d); };
  // Texto de una píldora de filtro activo.
  YF.filterLabel = function (k, v) {
    switch (k) {
      case 'decada': return YF.decadeName(v);
      case 'duracion': return RUNTIME[v] || v;
      case 'nota': return '★ ' + v + ' o más';
      case 'calidad': return QUALITY[v] || v;
      case 'audio': return 'Audio ' + YF.langName(v).toLowerCase();
      case 'subs': return 'Subtítulos ' + YF.langName(v).toLowerCase();
      case 'estado': return STATE[v] || v;
      case 'persona': return 'Con ' + v;
      default: return String(v);
    }
  };

  // «Películas · Ridley Scott · Años 80 · 3 títulos».
  YF.listHeading = function (kindLabel, filters, total) {
    var parts = [kindLabel].concat((filters || []).map(function (f) { return YF.filterLabel(f.k, f.v); }));
    if (total != null) parts.push(total + (total === 1 ? ' título' : ' títulos'));
    return parts.join(' · ');
  };

  // Índice circular del kiosco: -1 → el último; n → el primero.
  YF.wrapIndex = function (i, n) { return n > 0 ? ((Math.floor(i) % n) + n) % n : 0; };

  // Busca sin tildes ni mayúsculas: «scor» encuentra «Martin Scorsese»; «ñ» sigue siendo «n».
  YF.fold = function (s) { return String(s || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase(); };

  // Cambia el ancho de una URL de imagen de la API («…/poster?w=300» → «…?w=600»).
  YF.imgWidth = function (url, w) { return url ? url.replace(/([?&])w=\d+/, '$1w=' + w) : ''; };

  YF.storage = {
    get: function (k, def) { try { var v = localStorage.getItem('yourflix.web.' + k); return v == null ? def : JSON.parse(v); } catch (_) { return def; } },
    set: function (k, v) { try { localStorage.setItem('yourflix.web.' + k, JSON.stringify(v)); } catch (_) {} }
  };
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
