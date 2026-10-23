/* Utilidades sin dependencias. En la TV cuelgan de window.YF; en Node (tests), de module.exports. */
(function (YF) {
  'use strict';

  // Crea un elemento. Los textos van siempre por textContent (nunca innerHTML con datos).
  YF.el = function (tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text != null) e.textContent = text;
    return e;
  };

  YF.clear = function (e) { while (e.firstChild) e.removeChild(e.firstChild); return e; };

  YF.$ = function (id) { return document.getElementById(id); };

  // 6863 → «1:54:23»; 300 → «5:00».
  YF.clock = function (sec) {
    if (!(sec >= 0)) return '--:--';
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

  YF.bytes = function (n) { return n >= 1e9 ? (n / 1e9).toFixed(1) + ' GB' : Math.round(n / 1e6) + ' MB'; };

  var LANG = { spa: 'ES', es: 'ES', eng: 'EN', en: 'EN', fre: 'FR', fra: 'FR', ita: 'IT', ger: 'DE', deu: 'DE', jpn: 'JA', kor: 'KO', por: 'PT', cat: 'CA', eus: 'EU', glg: 'GL' };
  YF.lang = function (code) { return LANG[(code || '').toLowerCase()] || (code ? code.toUpperCase() : '?'); };

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

  YF.debounce = function (fn, ms) {
    var t = 0;
    return function () { var a = arguments, self = this; clearTimeout(t); t = setTimeout(function () { fn.apply(self, a); }, ms); };
  };

  YF.storage = {
    get: function (k, def) { try { var v = localStorage.getItem('yourflix.v1.' + k); return v == null ? def : JSON.parse(v); } catch (_) { return def; } },
    set: function (k, v) { try { localStorage.setItem('yourflix.v1.' + k, JSON.stringify(v)); } catch (_) {} }
  };
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
