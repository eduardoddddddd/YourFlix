/* AVPlay simulado para probar la app en un navegador. En la TV no hace nada:
 * webapis.js ya ha definido window.webapis. Marca como «cuelgue» lo que colgó
 * la TV en AstroIPTV: open/stop/close con prepare pendiente y setSelectTrack TEXT. */
(function () {
  'use strict';
  if (window.webapis && window.webapis.avplay) return;
  var st = 'NONE', t = 0, base = 0, dur = 0, url = '', listener = {}, preparing = false, silent = true, subTimer = 0;
  window.simHangs = [];
  function hang(why) { window.simHangs.push(why); console.error('[sim] CUELGUE: ' + why); }
  function cur() { return st === 'PLAYING' ? t + (Date.now() - base) : t; }
  function file() { var s = window.spike && window.spike.state; return s && s.file; }
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
    play: function () { if (st !== 'PLAYING') { base = Date.now(); st = 'PLAYING'; subs(); } },
    pause: function () { t = cur(); st = 'PAUSED'; },
    stop: function () { if (preparing) hang('stop con prepare pendiente'); t = 0; st = 'IDLE'; clearInterval(subTimer); },
    close: function () { if (preparing) hang('close con prepare pendiente'); st = 'NONE'; clearInterval(subTimer); },
    getState: function () { return st; },
    getCurrentTime: function () { return Math.min(cur(), dur); },
    getDuration: function () { return dur; },
    seekTo: function (ms, ok) { setTimeout(function () { t = Math.max(0, Math.min(ms, dur)); base = Date.now(); ok && ok(); }, 300); },
    jumpForward: function (ms, ok) { this.seekTo(cur() + ms, ok); },
    jumpBackward: function (ms, ok) { this.seekTo(cur() - ms, ok); },
    getTotalTrackInfo: function () {
      var f = file(), out = [];
      (f && f.tracks || []).forEach(function (tr) { out.push({ index: tr.index, type: tr.type === 'subtitle' ? 'TEXT' : tr.type.toUpperCase(), extra_info: JSON.stringify({ fourCC: tr.codec, language: tr.language || '' }) }); });
      return out;
    },
    getCurrentStreamInfo: function () { return this.getTotalTrackInfo().slice(0, 2); },
    setSelectTrack: function (type) { if (type === 'TEXT') hang('setSelectTrack TEXT'); },
    setSilentSubtitle: function (v) { silent = !!v; }
  } };
})();
