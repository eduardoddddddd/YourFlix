/* Cómo reproducir cada archivo en este navegador (ADR-006). Lógica pura: se prueba en Node.
 *  1. Directo (/stream, el archivo tal cual) si el navegador admite contenedor y códecs.
 *  2. HLS con el vídeo copiado y el audio a AAC (H.264, o HEVC si el navegador lo decodifica).
 *  3. HLS con el vídeo recodificado a H.264 (XviD, HEVC sin soporte, H.264 de 10 bits), si el
 *     servidor tiene codificador. Si no, se explica por qué no se puede ver. */
(function (YF, U) {
  'use strict';
  var P = YF.playback = {};

  var AUDIO = { aac: 'mp4a.40.2', mp3: 'mp4a.69', ac3: 'ac-3', eac3: 'ec-3', opus: 'opus', flac: 'flac', vorbis: 'vorbis' };
  var CONTAINER = { mp4: 'video/mp4', mov: 'video/mp4', mkv: 'video/x-matroska', webm: 'video/webm' };
  var AVC = { baseline: '42e0', 'constrained baseline': '42e0', main: '4d40', high: '6400' };

  P.is10bit = function (v) { return (v.bitDepth || 8) > 8 || /10|4:2:2|4:4:4/.test(v.profile || ''); };

  // Cadena RFC 6381 aproximada (la API no da el nivel: se deduce del tamaño).
  P.videoCodec = function (v) {
    v = v || {};
    var big = (v.height || 0) > 1100 || (v.width || 0) > 2000;
    if (v.codec === 'h264') {
      var p = AVC[(v.profile || 'high').toLowerCase()];
      if (!p || P.is10bit(v)) return '';
      return 'avc1.' + p + (big ? '33' : (v.height || 0) > 720 ? '29' : '1f');
    }
    if (v.codec === 'hevc') return P.is10bit(v) ? 'hvc1.2.4.L' + (big ? '153' : '123') + '.B0' : 'hvc1.1.6.L' + (big ? '153' : '123') + '.B0';
    if (v.codec === 'vp9') return 'vp09.00.40.08';
    if (v.codec === 'av1') return 'av01.0.08M.08';
    return '';
  };

  P.audioCodec = function (a) { return (a && AUDIO[a.codec]) || ''; };

  P.defaultAudio = function (file) {
    var audio = (file && file.audio) || [];
    return audio.filter(function (a) { return a.default; })[0] || audio[0] || null;
  };

  P.audioByIndex = function (file, index) {
    return ((file && file.audio) || []).filter(function (a) { return a.index === index; })[0] || null;
  };

  // Pista de audio al abrir: la del último idioma elegido, si la hay; si no, la de por defecto.
  P.pickAudio = function (file, prefLang) {
    var audio = (file && file.audio) || [];
    if (prefLang) {
      var same = audio.filter(function (a) { return U.lang(a.language) === prefLang; });
      if (same.length) return (same.filter(function (a) { return a.default; })[0] || same[0]).index;
    }
    var d = P.defaultAudio(file);
    return d ? d.index : -1;
  };

  // «ES 5.1 · AC3 · Comentarios»
  P.audioLabel = function (a) {
    if (!a) return 'Sin audio';
    var ch = a.channels >= 8 ? '7.1' : a.channels >= 6 ? '5.1' : a.channels === 1 ? 'mono' : a.channels === 2 ? 'estéreo' : '';
    var bits = [U.lang(a.language) === '?' ? 'Pista ' + a.index : U.lang(a.language)];
    if (ch) bits[0] += ' ' + ch;
    bits.push(String(a.codec || '').toUpperCase().replace('EAC3', 'E-AC3'));
    if (a.title) bits.push(a.title);
    return bits.join(' · ');
  };

  // Tipo MIME para preguntar al navegador por la reproducción directa ('' si no tiene sentido).
  P.directType = function (file, audio) {
    var mime = CONTAINER[file.container], v = P.videoCodec(file.video);
    if (!mime || !v) return '';
    if (!audio) return mime + '; codecs="' + v + '"';
    var a = P.audioCodec(audio);
    return a ? mime + '; codecs="' + v + ', ' + a + '"' : '';
  };

  // decide: cómo reproducir file con la pista de audio audioIndex.
  // caps: {canPlay(tipo) → bool, hls: 'native'|'hlsjs'|'', h264, hevc, hevc10: bool, transcode: bool}
  P.decide = function (file, audioIndex, caps) {
    var v = file.video || {}, audio = P.audioByIndex(file, audioIndex), def = P.defaultAudio(file);
    var codec = (v.codec || '?').toUpperCase().replace('MPEG4', 'XviD');
    // Directo: solo con la pista por defecto (el navegador no deja elegir otra de forma fiable).
    if (!audio || audio === def) {
      var t = P.directType(file, audio);
      if (t && caps.canPlay(t)) return { method: 'direct', mode: '', why: 'Se reproduce tal cual, sin convertir nada.' };
    }
    if (!caps.hls) return { method: 'none', mode: '', why: caps.remuxOff ? 'Este archivo necesita convertirse y el servidor tiene desactivada la conversión en vivo (--remux-jobs 0).' : 'Este navegador no reproduce HLS, y el archivo necesita convertirse.' };
    var copy = false;
    if (v.codec === 'h264') copy = caps.h264 && !P.is10bit(v);
    else if (v.codec === 'hevc') copy = P.is10bit(v) ? caps.hevc10 : caps.hevc;
    var aWhy = audio && audio.codec !== 'aac' ? ' (audio ' + String(audio.codec).toUpperCase() + ' → AAC)' : '';
    if (copy) return { method: 'hls', mode: 'copy', why: 'El servidor adapta el archivo al vuelo sin tocar el vídeo' + aWhy + '.' };
    if (caps.transcode) {
      return { method: 'hls', mode: 'h264', why: 'El servidor recodifica el vídeo (' + codec + (P.is10bit(v) ? ' 10 bits' : '') + ' → H.264) al vuelo: tarda unos segundos en arrancar y en cada salto.' };
    }
    var why = v.codec === 'hevc' ? 'Este navegador no decodifica HEVC' + (P.is10bit(v) ? ' de 10 bits' : '') : 'Los navegadores no reproducen vídeo ' + codec + (P.is10bit(v) ? ' de 10 bits' : '');
    return { method: 'none', mode: '', why: why + ' y el servidor no tiene recodificación de vídeo activada (opción --video-encoder). Se puede ver en la TV.' };
  };

  P.variant = function (audioIndex, mode) { return (audioIndex >= 0 ? 'a' + audioIndex : 'an') + '-' + mode; };

  P.hlsBase = function (fileId, variant) { return '/api/v1/files/' + encodeURIComponent(fileId) + '/hls/' + variant + '/'; };

  P.masterURL = function (fileId, variant, session, start) {
    return P.hlsBase(fileId, variant) + 'master.m3u8?s=' + encodeURIComponent(session) + (start > 0 ? '&t=' + Math.floor(start) : '');
  };

  P.session = function () { return 'w' + Math.random().toString(36).slice(2, 12); };

  // Subtítulos al empezar. pref: null (nunca eligió), 'off' o idioma («ES»). Los forzados en el
  // idioma del audio se activan aunque esté en 'off' (como en la TV).
  P.pickSubtitle = function (tracks, pref, audioLang) {
    var L = function (t) { return U.lang(t.language); };
    tracks = tracks || [];
    if (pref && pref !== 'off') {
      var full = tracks.filter(function (t) { return L(t) === pref && !t.forced; });
      if (full.length) return (full.filter(function (t) { return t.default; })[0] || full[0]).id;
    }
    var forced = tracks.filter(function (t) { return t.forced && L(t) === audioLang; })[0];
    return forced ? forced.id : null;
  };

  P.resumeAt = function (f) { var p = f && f.progress; return p && !p.watched && p.position > 60 ? p.position : 0; };

  // Archivo que propone la ficha: el que está a medias; si no, el primero.
  P.chooseFile = function (files) {
    var going = (files || []).filter(function (f) { return P.resumeAt(f) > 0; });
    if (going.length) return going[0];
    // En una serie, el primer capítulo sin ver.
    var unseen = (files || []).filter(function (f) { return f.season != null && !(f.progress && f.progress.watched); });
    return unseen[0] || (files || [])[0] || null;
  };

  P.nextFile = function (files, f) {
    var i = (files || []).indexOf(f);
    return i >= 0 && i + 1 < files.length && f.season != null ? files[i + 1] : null;
  };

  // ¿Toca guardar el progreso? Cada 15 s reproduciendo; siempre al pausar, saltar o salir.
  P.shouldSave = function (lastSaved, now, force) { return force || now - lastSaved >= 15000; };

  // Capacidades del navegador (solo en el navegador).
  P.detectCaps = function (video, Hls, transcode) {
    // Safari y todos los navegadores de iPhone/iPad (son WebKit): HLS nativo.
    var apple = /Apple/.test(navigator.vendor || '');
    var native = !!(video.canPlayType && video.canPlayType('application/vnd.apple.mpegurl'));
    var hlsjs = !!(Hls && Hls.isSupported && Hls.isSupported());
    var hls = apple && native ? 'native' : hlsjs ? 'hlsjs' : native ? 'native' : '';
    var MS = window.ManagedMediaSource || window.MediaSource;
    var check = hls === 'hlsjs' ? function (t) { try { return MS.isTypeSupported(t); } catch (_) { return false; } }
      : function (t) { return !!video.canPlayType(t); };
    return {
      hls: hls,
      h264: check('video/mp4; codecs="avc1.640029"'),
      hevc: check('video/mp4; codecs="hvc1.1.6.L123.B0"'),
      hevc10: check('video/mp4; codecs="hvc1.2.4.L153.B0"'),
      transcode: !!transcode,
      canPlay: function (t) { return video.canPlayType(t) === 'probably'; }
    };
  };
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}),
   typeof module !== 'undefined' && module.exports ? require('./util.js') : window.YF);
