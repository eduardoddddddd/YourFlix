/* AVPlay simulado para probar YourFlix en el navegador del Mac (copia adaptada de tools/spike-avplay). En la TV no hace nada:
 * webapis.js ya ha definido window.webapis. Marca como «cuelgue» lo que colgó
 * la TV en AstroIPTV: open/stop/close con prepare pendiente y setSelectTrack TEXT. */
(function () {
  'use strict';
  if (window.webapis && window.webapis.avplay) return;
  var st = 'NONE', t = 0, base = 0, dur = 0, url = '', listener = {}, preparing = false, silent = true, subTimer = 0, seeking = false, endTimer = 0;
  window.simHangs = [];
  function hang(why) { window.simHangs.push(why); console.error('[sim] CUELGUE: ' + why); }
  function cur() { return st === 'PLAYING' ? t + (Date.now() - base) : t; }
  function file() { var p = window.YF && window.YF.player; return p && p.opts && p.opts.file; }
  function subs() {
    clearInterval(subTimer);
    subTimer = setInterval(function () {
      if (st === 'PLAYING' && !silent && listener.onsubtitlechange) listener.onsubtitlechange(4500, 'Subtítulo simulado ' + Math.round(cur() / 1000) + ' s');
    }, 5000);
  }
  window.webapis = { avplay: {
    open: function (u) { if (preparing) hang('open con prepare pendiente'); url = u; st = 'IDLE'; t = 0; },
    setDisplayRect: function () {}, setDisplayMethod: function () {}, setStreamingProperty: function () {},
    setListener: function (l) { listener = l || {}; },
    prepareAsync: function (ok, err) {
      preparing = true;
      setTimeout(function () {
        preparing = false;
        if (/dts/i.test(url)) { st = 'IDLE'; err && err('PLAYER_ERROR_NOT_SUPPORTED_FORMAT'); return; }
        var f = file(); dur = f && f.duration ? Math.round(f.duration * 1000) : 120000;
        st = 'READY'; ok && ok();
      }, 600);
    },
    play: function () {
      if (st !== 'PLAYING') { base = Date.now(); st = 'PLAYING'; subs(); }
      clearInterval(endTimer);
      endTimer = setInterval(function () { if (st === 'PLAYING' && dur && cur() >= dur) { t = dur; st = 'READY'; clearInterval(endTimer); if (listener.onstreamcompleted) listener.onstreamcompleted(); } }, 200);
    },
    pause: function () { t = cur(); st = 'PAUSED'; },
    stop: function () { if (preparing) hang('stop con prepare pendiente'); t = 0; st = 'IDLE'; clearInterval(subTimer); },
    close: function () { if (preparing) hang('close con prepare pendiente'); st = 'NONE'; clearInterval(subTimer); },
    getState: function () { return st; },
    getCurrentTime: function () { return Math.min(cur(), dur); },
    getDuration: function () { return dur; },
    seekTo: function (ms, ok, err) {
      // Como la TV: otro salto mientras se rellena el búfer falla.
      if (seeking) { window.simHangs.push('seek durante otro seek'); return err && err('PLAYER_ERROR_SEEK_FAILED'); }
      if (ms > dur) return err && err('PLAYER_ERROR_SEEK_FAILED');
      seeking = true;
      var wasPlaying = st === 'PLAYING';
      setTimeout(function () {
        // Como la TV: la posición salta al destino y no avanza hasta rellenar el búfer.
        t = Math.max(0, Math.min(ms, dur)); if (wasPlaying) st = 'READY'; ok && ok();
        if (listener.onbufferingstart) listener.onbufferingstart();
        setTimeout(function () { seeking = false; if (wasPlaying) { base = Date.now(); st = 'PLAYING'; } if (listener.onbufferingcomplete) listener.onbufferingcomplete(); }, 700);
      }, 300);
    },
    jumpForward: function (ms, ok) { this.seekTo(cur() + ms, ok); },
    jumpBackward: function (ms, ok) { this.seekTo(cur() - ms, ok); },
    getTotalTrackInfo: function () {
      var f = file(), out = [];
      if (f && f.video) out.push({ index: 0, type: 'VIDEO', extra_info: '{}' });
      (f && f.audio || []).forEach(function (a) { out.push({ index: a.index, type: 'AUDIO', extra_info: JSON.stringify({ fourCC: a.codec, language: a.language || '' }) }); });
      (f && f.subtitles || []).forEach(function (s) { if (s.text) out.push({ index: s.index, type: 'TEXT', extra_info: JSON.stringify({ language: s.language || '' }) }); });
      return out;
    },
    getCurrentStreamInfo: function () { return this.getTotalTrackInfo().slice(0, 2); },
    setSelectTrack: function (type) { if (type === 'TEXT') hang('setSelectTrack TEXT'); },
    setSilentSubtitle: function (v) { silent = !!v; }
  } };
})();
