/* Reproductor AVPlay. Reglas de AstroIPTV y de la fase 0 (docs/PRUEBAS-TV.md):
 *  - una sola apertura a la vez; nunca stop/close con un prepareAsync pendiente;
 *  - token por apertura para descartar callbacks tardíos;
 *  - nunca setSelectTrack('TEXT'): los subtítulos se activan con setSilentSubtitle y los dibuja la app;
 *  - saltos encolados: otro salto mientras AVPlay rellena el búfer devuelve PLAYER_ERROR_SEEK_FAILED. */
(function (YF) {
  'use strict';

  // SeekQueue: pura y testeable. seek(ms, done) hace el salto real y llama a done al reanudar.
  function SeekQueue(seek) { this.seek = seek; this.busy = false; this.next = null; this.last = null; }
  SeekQueue.prototype.request = function (ms) {
    this.last = ms;
    if (this.busy) { this.next = ms; return false; }
    this.busy = true;
    var self = this;
    // done(false): el salto terminó pero aún no se reproduce (reanudar antes de play): lo
    // pendiente se guarda para cuando arranque (block/release), en vez de saltar ya.
    this.seek(ms, function (flush) {
      self.busy = false;
      if (flush !== false && self.next != null) { var n = self.next; self.next = null; self.request(n); return; }
      if (self.next == null) self.last = null;
      if (self.onIdle) { var f = self.onIdle; self.onIdle = null; f(); }
    });
    return true;
  };
  // block: ocupa la cola sin saltar (arranque de la reproducción); devuelve la función que la libera.
  SeekQueue.prototype.block = function () {
    if (this.busy) return function () {};
    this.busy = true;
    var self = this;
    return function () {
      if (!self.busy) return;
      self.busy = false;
      if (self.next != null) { var n = self.next; self.next = null; self.request(n); }
    };
  };
  // Base para saltos relativos: el destino pendiente si lo hay (pulsaciones seguidas se suman).
  SeekQueue.prototype.base = function (current) { return this.next != null ? this.next : (this.last != null ? this.last : current); };
  YF.SeekQueue = SeekQueue;

  // Destino de un salto relativo, sin salirse del archivo (más allá del final, AVPlay falla).
  YF.seekTarget = function (base, delta, duration) {
    var t = base + delta;
    if (duration > 0) t = Math.min(t, duration - 5000);
    return Math.max(0, Math.floor(t));
  };

  if (typeof document === 'undefined') return; // Node: solo la parte pura

  var av = function () { return window.webapis && window.webapis.avplay; };
  var $ = YF.$;
  var P = YF.player = {
    active: false, token: 0, opened: false, preparing: false, wanted: null, closeAfterPrepare: false,
    opts: null, duration: 0, audio: [], audioPos: 0, subsOn: false, timers: {}
  };

  function trace(msg) { if (window.console) console.log('[av] ' + msg); }
  function call(name) {
    var args = Array.prototype.slice.call(arguments, 1);
    trace(name + ' ' + args.filter(function (a) { return typeof a !== 'function'; }).join(' '));
    return av()[name].apply(av(), args);
  }
  function now() { try { return av().getCurrentTime(); } catch (_) { return 0; } }
  function state() { try { return av().getState(); } catch (_) { return ''; } }

  // waitResume: llama a cb cuando la reproducción avanza de nuevo tras un salto o tras play.
  // Lo marca bufferingcomplete; respaldo, por si AVPlay no emite búfer (salto dentro de lo
  // ya cargado): que el tiempo avance pasado 1,5 s. Nunca más de 8 s.
  function waitResume(cb) {
    var finished = false, finish = function () { if (!finished) { finished = true; clearTimeout(guard); if (P.waitBuffer === finish) P.waitBuffer = null; cb(); } };
    var guard = setTimeout(finish, 8000);
    P.waitBuffer = finish;
    setTimeout(function () {
      if (finished) return;
      var t0 = now(), poll = setInterval(function () {
        if (finished) { clearInterval(poll); return; }
        if (now() > t0 + 300 || state() === 'PAUSED') { clearInterval(poll); finish(); }
      }, 250);
    }, 1500);
    return finish;
  }

  var queue = new SeekQueue(function (ms, done) {
    showSeek(ms);
    var end = function (flush) { showSeek(null); done(flush); };
    try {
      var st0 = state();
      av().seekTo(ms, function () {
        // Antes de play (reanudar): el búfer lo cargará play; lo pendiente espera al arranque.
        if (st0 !== 'PLAYING' && st0 !== 'PAUSED') return end(false);
        waitResume(end); // reproduciendo o en pausa: esperar a que termine de cargar
      }, function (e) { trace('seekTo error ' + e); end(); });
    } catch (e) { trace('seekTo excepción ' + e.message); end(); }
  });

  function nativeClose() {
    if (!P.opened) return;
    if (P.preparing) { P.closeAfterPrepare = true; trace('cierre aplazado: prepare pendiente'); return; }
    try { call('stop'); } catch (_) {}
    try { call('close'); } catch (_) {}
    P.opened = false;
  }

  function listener(token) {
    var mine = function () { return token === P.token; };
    return {
      onbufferingstart: function () { if (mine()) setState('Cargando…'); },
      onbufferingprogress: function () {},
      onbufferingcomplete: function () { if (mine()) { setState(''); if (P.waitBuffer) P.waitBuffer(); } },
      onstreamcompleted: function () { if (mine()) exit(true); },
      onerror: function (e) { if (mine()) fail('Error de reproducción: ' + e); },
      onevent: function () {},
      oncurrentplaytime: function () {},
      onsubtitlechange: function (duration, text) { if (mine() && P.subsOn) showSub(text, duration); }
    };
  }

  // play({file, title, label, startAt (s), onExit(positionS, durationS, completed)})
  P.play = function (opts) {
    if (!av()) { YF.toast('AVPlay no disponible'); return; }
    if (P.preparing) { P.wanted = opts; P.closeAfterPrepare = true; return; }
    nativeClose();
    P.active = true; P.opts = opts; P.duration = (opts.file.duration || 0) * 1000; P.audio = []; P.audioPos = 0;
    var token = ++P.token;
    document.documentElement.classList.add('video-mode');
    $('player').classList.remove('hidden');
    $('playerMsg').classList.add('hidden');
    $('subtitle').classList.add('hidden');
    $('osdTitle').textContent = opts.title;
    $('osdLabel').textContent = opts.label || '';
    $('osdSubs').textContent = P.subsOn ? 'sí' : 'no';
    $('osdAudio').textContent = '—';
    setState('Abriendo…');
    showOsd(true);
    try {
      call('open', YF.api.url(opts.file.stream));
      P.opened = true;
      call('setDisplayRect', 0, 0, 1920, 1080);
      try { call('setDisplayMethod', 'PLAYER_DISPLAY_MODE_LETTER_BOX'); } catch (_) {}
      if (opts.file.video && opts.file.video.width >= 3800) { try { call('setStreamingProperty', 'SET_MODE_4K', 'TRUE'); } catch (_) {} }
      av().setListener(listener(token));
      P.preparing = true;
      trace('prepareAsync');
      av().prepareAsync(function () { prepared(token); }, function (e) {
        P.preparing = false;
        if (token !== P.token || P.closeAfterPrepare) return afterCancel();
        fail('No se puede abrir: ' + e);
      });
    } catch (e) { P.preparing = false; fail('No se puede abrir: ' + e.message); }
  };

  function afterCancel() {
    P.closeAfterPrepare = false;
    nativeClose();
    if (P.wanted) { var w = P.wanted; P.wanted = null; P.play(w); }
  }

  function prepared(token) {
    P.preparing = false;
    trace('prepareAsync done');
    if (token !== P.token || P.closeAfterPrepare) return afterCancel();
    try { P.duration = av().getDuration() || P.duration; } catch (_) {}
    readAudio();
    try { call('setSilentSubtitle', !P.subsOn); } catch (_) {}
    var start = (P.opts.startAt || 0) * 1000;
    var go = function () {
      var release = queue.block(); // ⏩ durante el arranque se acumula y se aplica al empezar
      try { call('play'); } catch (e) { release(); return fail(e.message); }
      waitResume(release);
      setState(''); startTimers(); showOsd();
    };
    if (start > 10000 && start < P.duration - 10000) {
      // Por la cola: si el usuario pulsa ⏩ mientras tanto, se encola en vez de solaparse.
      queue.onIdle = go;
      queue.request(start);
    } else go();
  }

  function readAudio() {
    var all = [];
    try { all = av().getTotalTrackInfo() || []; } catch (_) {}
    P.audio = [];
    for (var i = 0; i < all.length; i++) {
      if (all[i].type !== 'AUDIO') continue;
      var lang = '';
      try { lang = JSON.parse(all[i].extra_info).language; } catch (_) {}
      P.audio.push({ index: all[i].index, lang: YF.lang(lang) });
    }
    $('osdAudio').textContent = P.audio.length ? P.audio[0].lang + (P.audio.length > 1 ? ' (1/' + P.audio.length + ')' : '') : '—';
  }

  function nextAudio() {
    if (P.preparing || queue.busy) return;
    if (P.audio.length < 2) { flash('Solo hay una pista de audio'); return; }
    P.audioPos = (P.audioPos + 1) % P.audio.length;
    var t = P.audio[P.audioPos];
    try { call('setSelectTrack', 'AUDIO', t.index); } catch (e) { flash('No se pudo cambiar el audio'); return; }
    $('osdAudio').textContent = t.lang + ' (' + (P.audioPos + 1) + '/' + P.audio.length + ')';
    flash('Audio: ' + t.lang);
  }

  function toggleSubs() {
    P.subsOn = !P.subsOn;
    try { call('setSilentSubtitle', !P.subsOn); } catch (_) {}
    if (!P.subsOn) $('subtitle').classList.add('hidden');
    $('osdSubs').textContent = P.subsOn ? 'sí' : 'no';
    var subs = (P.opts.file.subtitles || []).filter(function (s) { return s.text; });
    flash(P.subsOn ? (subs.length ? 'Subtítulos activados' : 'Este archivo no trae subtítulos de texto') : 'Subtítulos desactivados');
  }

  var subTimer = 0;
  function showSub(text, duration) {
    var s = $('subtitle');
    s.textContent = String(text).replace(/<[^>]*>/g, '').replace(/\{[^}]*\}/g, '');
    s.classList.remove('hidden');
    clearTimeout(subTimer);
    subTimer = setTimeout(function () { s.classList.add('hidden'); }, Math.max(800, duration || 2500));
  }

  function jump(deltaMs) {
    if (P.preparing || !P.opened) return;
    var target = YF.seekTarget(queue.base(now()), deltaMs, P.duration);
    queue.request(target);
    showOsd();
  }

  function togglePause() {
    var s = state();
    try {
      if (s === 'PLAYING') { call('pause'); setState('Pausa'); showOsd(true); saveProgress(); }
      else if (s === 'PAUSED' || s === 'READY') { call('play'); setState(''); showOsd(); }
    } catch (_) {}
  }

  function saveProgress() {
    if (!P.opts || !P.opened || P.preparing) return;
    var pos = now() / 1000, dur = P.duration / 1000;
    if (pos > 5) YF.api.progress(P.opts.file.id, pos, dur);
  }

  function startTimers() {
    stopTimers();
    P.timers.tick = setInterval(tick, 500);
    P.timers.save = setInterval(saveProgress, 15000);
    tick();
  }
  function stopTimers() { clearInterval(P.timers.tick); clearInterval(P.timers.save); clearTimeout(P.timers.osd); }

  function tick() {
    var t = now(), d = P.duration;
    $('osdTime').textContent = YF.clock(t / 1000);
    $('osdDuration').textContent = YF.clock(d / 1000);
    $('osdProgress').style.width = d ? Math.min(100, t / d * 100) + '%' : '0';
  }

  function showSeek(ms) {
    var m = $('osdSeek');
    if (ms == null) { m.style.display = 'none'; return; }
    m.style.display = 'block';
    m.style.left = P.duration ? (ms / P.duration * 100) + '%' : '0';
    setState('→ ' + YF.clock(ms / 1000));
  }

  function setState(s) { $('osdState').textContent = s; }

  function showOsd(stay) {
    var o = $('osd');
    o.classList.remove('off');
    clearTimeout(P.timers.osd);
    if (!stay) P.timers.osd = setTimeout(function () { if (state() === 'PLAYING' && !queue.busy) o.classList.add('off'); }, 4000);
  }

  function flash(text) { setState(text); showOsd(); }

  function fail(msg) {
    trace(msg);
    var m = $('playerMsg');
    m.textContent = msg;
    m.classList.remove('hidden');
    setTimeout(function () { if (P.active) exit(false); }, 3500);
  }

  function exit(completed) {
    if (!P.active) return;
    var pos = now() / 1000, dur = P.duration / 1000, opts = P.opts;
    if (completed) pos = dur;
    // Guardar el progreso antes de avisar: la ficha y «Seguir viendo» se recargan en onExit.
    var saved = opts && (pos > 5 || completed) ? YF.api.progress(opts.file.id, pos, dur) : Promise.resolve();
    stopTimers();
    nativeClose();
    P.active = false;
    $('player').classList.add('hidden');
    document.documentElement.classList.remove('video-mode');
    if (YF.app && YF.app.markDirty) YF.app.markDirty();
    saved.then(function () { if (opts && opts.onExit) opts.onExit(pos, dur, !!completed); });
  }
  P.exit = exit;

  // Teclas durante la reproducción. Devuelve true si la ha usado.
  P.key = function (k) {
    if (!P.active) return false;
    switch (k) {
      case 'Back': case 'Stop': exit(false); break;
      case 'Enter': case 'PlayPause': case 'Play': case 'Pause': togglePause(); break;
      case 'Left': jump(-10000); break;
      case 'Right': jump(10000); break;
      case 'Rewind': jump(-30000); break;
      case 'FastForward': jump(30000); break;
      case 'Green': nextAudio(); break;
      case 'Yellow': toggleSubs(); break;
      default: showOsd();
    }
    return true;
  };

  document.addEventListener('visibilitychange', function () { if (document.hidden && P.active) exit(false); });
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
