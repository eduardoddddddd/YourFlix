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

  // cueAt: línea visible en t (s) en cues ordenadas [[inicio, fin, texto], …]; búsqueda binaria.
  YF.cueAt = function (cues, t) {
    var lo = 0, hi = cues.length - 1, found = -1;
    while (lo <= hi) {
      var mid = (lo + hi) >> 1;
      if (cues[mid][0] <= t) { found = mid; lo = mid + 1; } else hi = mid - 1;
    }
    // Puede haber solapes: mirar unas pocas anteriores que sigan vivas.
    for (var i = found, n = 0; i >= 0 && n < 4; i--, n++) if (cues[i][1] > t) return cues[i];
    return null;
  };

  // pickSubtitle: pista a activar al empezar. pref: null (nunca eligió), 'off' o idioma («ES»).
  // Los forzados en el idioma del audio (diálogos en otro idioma) se activan aunque esté en 'off'.
  YF.pickSubtitle = function (tracks, pref, audioLang) {
    var L = function (t) { return YF.lang(t.language); };
    if (pref && pref !== 'off') {
      var full = tracks.filter(function (t) { return L(t) === pref && !t.forced; });
      if (full.length) return (full.filter(function (t) { return t.default; })[0] || full[0]).id;
    }
    var forced = tracks.filter(function (t) { return t.forced && L(t) === audioLang; })[0];
    return forced ? forced.id : null;
  };

  // audioStart: posición (en las pistas que lista AVPlay) que hay que elegir a mano al empezar, o -1.
  // En la TV real AVPlay descarta las pistas DTS y suena la primera compatible: no hay nada que hacer.
  // Solo si AVPlay listara la DTS por defecto habría que pasar a otra (después de play: en READY,
  // setSelectTrack da PLAYER_ERROR_INVALID_STATE).
  YF.audioStart = function (fileAudio, avTracks) {
    var dts = function (c) { return /dts|dca/i.test(c || ''); };
    fileAudio = fileAudio || [];
    var def = fileAudio.filter(function (a) { return a.default; })[0] || fileAudio[0];
    if (!def || !dts(def.codec) || !avTracks.some(function (t) { return dts(t.codec); })) return -1;
    for (var i = 0; i < avTracks.length; i++) if (!dts(avTracks[i].codec)) return i;
    return -1;
  };

  // codecName: fourCC de AVPlay a nombre corto («audio/x-ac3» → «AC3», «audio/mpeg» → «MPEG»).
  YF.codecName = function (fourCC) { return String(fourCC || '').replace(/^audio\//i, '').replace(/^x-/i, '').toUpperCase(); };

  if (typeof document === 'undefined') return; // Node: solo la parte pura

  var av = function () { return window.webapis && window.webapis.avplay; };
  var $ = YF.$;
  var P = YF.player = {
    active: false, token: 0, opened: false, preparing: false, wanted: null, closeAfterPrepare: false,
    opts: null, duration: 0, audio: [], audioPos: 0, btn: -1, timers: {}
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
      onstreamcompleted: function () { if (mine()) completed(); },
      onerror: function (e) { if (mine()) fail('Error de reproducción: ' + e); },
      onevent: function () {},
      oncurrentplaytime: function () {},
      onsubtitlechange: function () {} // silenciado: los subtítulos los dibuja la app
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
    P.subTracks = null; P.subId = null; P.cues = []; P.menu = null; P.nextUp = null;
    $('subMenu').classList.add('hidden');
    $('nextUp').classList.add('hidden');
    $('osdSubs').textContent = 'no';
    $('osdAudio').textContent = '—';
    $('osdNextKey').classList.toggle('hidden', !(opts.next && opts.next()));
    markBtn(-1);
    P.notice = '';
    loadSubTracks(opts.file.id, token);
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
    var fix = YF.audioStart(P.opts.file.audio, P.audio);
    if (!P.audio.length && P.opts.file.audio && P.opts.file.audio.length) P.notice = noAudioText();
    try { call('setSilentSubtitle', true); } catch (_) {} // AVPlay nunca dibuja: lo hace la app
    P.audioReady = true;
    autoPick(token);
    var start = (P.opts.startAt || 0) * 1000;
    var go = function () {
      var release = queue.block(); // ⏩ durante el arranque se acumula y se aplica al empezar
      try { call('play'); } catch (e) { release(); return fail(e.message); }
      waitResume(function () { release(); if (fix >= 0 && token === P.token) selectAudio(fix, true); });
      setState(''); startTimers(); showOsd();
      if (P.notice) setTimeout(function () { if (token === P.token) flash(P.notice); }, 1500);
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
      var x = {};
      try { x = JSON.parse(all[i].extra_info) || {}; } catch (_) {}
      var ch = parseInt(x.channels, 10), codec = YF.codecName(x.fourCC);
      var label = YF.lang(x.language) + (ch === 8 ? ' 7.1' : ch === 6 ? ' 5.1' : ch === 2 ? ' estéreo' : ch === 1 ? ' mono' : '') + (codec ? ' · ' + codec : '');
      P.audio.push({ index: all[i].index, lang: YF.lang(x.language), codec: codec, label: label });
    }
    showAudio();
  }

  function showAudio() {
    var n = P.audio.length;
    $('osdAudio').textContent = n ? P.audio[P.audioPos].label + (n > 1 ? ' (' + (P.audioPos + 1) + '/' + n + ')' : '') : '—';
  }

  function noAudioText() {
    var a = (P.opts && P.opts.file.audio) || [];
    if (!a.length) return 'Este archivo no tiene audio';
    return a.every(function (x) { return /dts|dca/i.test(x.codec); }) ? 'Sin sonido: el audio es DTS y la tele no lo reproduce' : 'La tele no encuentra un audio que pueda reproducir';
  }

  function selectAudio(i, quiet) {
    var t = P.audio[i];
    if (!t) return;
    if (P.preparing || queue.busy) { if (!quiet) flash('Un momento, está cargando'); return; } // AVPlay: una operación cada vez
    try { call('setSelectTrack', 'AUDIO', t.index); } catch (e) { flash('No se pudo cambiar el audio'); return; }
    P.audioPos = i;
    showAudio();
    if (!quiet) flash('Audio: ' + t.label);
  }

  function nextAudio() {
    if (P.audio.length < 2) { flash(P.audio.length ? 'Solo hay una pista de audio' : noAudioText()); return; }
    selectAudio((P.audioPos + 1) % P.audio.length);
  }

  function openAudioMenu() {
    if (!P.audio.length) { flash(noAudioText()); return; }
    P.menu = { kind: 'audio', items: P.audio, sel: P.audioPos };
    $('menuTitle').textContent = 'Audio';
    drawMenu();
    $('subMenu').classList.remove('hidden');
    showOsd(true);
  }

  // ---------- Botones del reproductor (▼ desde el vídeo): el mando de The Serif no tiene botones de color ----------
  var BTNS = ['btnAudio', 'btnSubs', 'osdNextKey'];
  function osdButtons() { return BTNS.map(function (id) { return $(id); }).filter(function (b) { return !b.classList.contains('hidden'); }); }

  function markBtn(i) {
    var bs = osdButtons();
    P.btn = i < 0 || !bs.length ? -1 : Math.max(0, Math.min(bs.length - 1, i));
    BTNS.forEach(function (id) { $(id).classList.remove('focused'); });
    if (P.btn >= 0) bs[P.btn].classList.add('focused');
  }

  function focusBtn(i) { markBtn(i); showOsd(P.btn >= 0); }

  function btnKey(k) {
    if (k === 'Left') focusBtn(P.btn - 1);
    else if (k === 'Right') focusBtn(P.btn + 1);
    else if (k === 'Up' || k === 'Back') focusBtn(-1);
    else if (k === 'Down') showOsd(true);
    else if (k === 'Enter') {
      var id = osdButtons()[P.btn].id;
      if (id === 'btnAudio') openAudioMenu();
      else if (id === 'btnSubs') openSubMenu();
      else playNext();
    } else return false;
    return true;
  }

  // ---------- Subtítulos: los extrae el servidor y los dibuja la app según el tiempo ----------
  function loadSubTracks(fileId, token) {
    P.audioReady = false;
    YF.api.get('/files/' + encodeURIComponent(fileId) + '/subtitles').then(function (r) {
      if (token !== P.token) return;
      P.subTracks = r.tracks || [];
      P.subImages = r.imageTracks || 0;
      autoPick(token);
    }, function () { if (token === P.token) P.subTracks = []; });
  }

  // Cuando se conocen el audio y las pistas, aplicar la preferencia guardada.
  function autoPick(token) {
    if (token !== P.token || !P.subTracks || !P.audioReady || P.subId) return;
    var audio = P.audio.length ? P.audio[P.audioPos].lang : '';
    var id = YF.pickSubtitle(P.subTracks, YF.storage.get('subPref', null), audio);
    if (id) selectSub(id, true);
  }

  function trackById(id) { return (P.subTracks || []).filter(function (t) { return t.id === id; })[0]; }

  function selectSub(id, quiet) {
    var token = P.token;
    P.subId = id; P.cues = [];
    $('subtitle').classList.add('hidden');
    clearInterval(P.timers.sub);
    var t = trackById(id);
    $('osdSubs').textContent = t ? t.label : 'no';
    if (!t) return;
    if (!quiet) flash('Cargando subtítulos…');
    // La primera vez el servidor lee el vídeo entero con ffmpeg: margen amplio.
    YF.api.get('/files/' + encodeURIComponent(P.opts.file.id) + '/subtitles/' + encodeURIComponent(id), 180000).then(function (cues) {
      if (token !== P.token || P.subId !== id) return;
      P.cues = cues || [];
      P.timers.sub = setInterval(drawSub, 200);
      if (!quiet) flash('Subtítulos: ' + t.label);
    }, function () { if (token === P.token && P.subId === id) { flash('No se pudieron cargar los subtítulos'); P.subId = null; $('osdSubs').textContent = 'no'; } });
  }

  var lastCue = null;
  function drawSub() {
    var c = YF.cueAt(P.cues, now() / 1000), s = $('subtitle');
    if (c === lastCue) return;
    lastCue = c;
    if (!c) { s.classList.add('hidden'); return; }
    s.textContent = c[2];
    s.classList.remove('hidden');
  }

  function openSubMenu() {
    if (!P.subTracks) { flash('Buscando subtítulos…'); return; }
    var items = [{ id: null, label: 'Desactivados' }].concat(P.subTracks);
    var sel = 0;
    items.forEach(function (it, i) { if (it.id === P.subId) sel = i; });
    P.menu = { kind: 'subs', items: items, sel: sel };
    $('menuTitle').textContent = 'Subtítulos';
    drawMenu();
    $('subMenu').classList.remove('hidden');
    showOsd(true);
  }

  function drawMenu() {
    var m = P.menu, list = YF.clear($('subMenuList'));
    m.items.forEach(function (it, i) {
      var on = m.kind === 'audio' ? i === P.audioPos : it.id === P.subId;
      list.appendChild(YF.el('div', 'sub-item' + (i === m.sel ? ' sel' : '') + (on ? ' on' : ''), it.label));
    });
    if (m.kind === 'subs' && !P.subTracks.length) list.appendChild(YF.el('p', '', P.subImages ? 'Este archivo solo trae subtítulos en imagen (PGS), que la tele no puede mostrar.' : 'Este archivo no trae subtítulos.'));
  }

  function menuKey(k) {
    var m = P.menu;
    if (k === 'Up') m.sel = Math.max(0, m.sel - 1);
    else if (k === 'Down') m.sel = Math.min(m.items.length - 1, m.sel + 1);
    else if (k === 'Enter') {
      var it = m.items[m.sel];
      closeMenu();
      if (m.kind === 'audio') { selectAudio(m.sel); return; }
      if (!it.id) { selectSub(null); YF.storage.set('subPref', 'off'); flash('Subtítulos desactivados'); }
      else { if (!it.forced) YF.storage.set('subPref', YF.lang(it.language)); selectSub(it.id); }
      return;
    } else if (k === 'Back' || k === 'Yellow' || k === 'Green') { closeMenu(); return; }
    drawMenu();
  }

  function closeMenu() { P.menu = null; $('subMenu').classList.add('hidden'); showOsd(P.btn >= 0); }

  // ---------- Siguiente capítulo ----------
  function completed() {
    var next = P.opts.next && P.opts.next();
    if (!next) return exit(true);
    YF.api.progress(P.opts.file.id, P.duration / 1000, P.duration / 1000); // visto
    stopTimers();
    $('subtitle').classList.add('hidden');
    var left = 10;
    P.nextUp = next;
    $('nextUpTitle').textContent = next.label;
    $('nextUpCount').textContent = 'Empieza en ' + left + ' s';
    $('nextUp').classList.remove('hidden');
    showOsd(true);
    P.timers.next = setInterval(function () {
      left--;
      $('nextUpCount').textContent = 'Empieza en ' + left + ' s';
      if (left <= 0) playNext();
    }, 1000);
  }

  function playNext() {
    clearInterval(P.timers.next);
    var next = P.nextUp || (P.opts.next && P.opts.next());
    if (!next) return;
    if (!P.nextUp) saveProgress(); // salto manual con el azul: guardar por dónde iba
    P.nextUp = null;
    $('nextUp').classList.add('hidden');
    P.play(next);
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
  function stopTimers() { clearInterval(P.timers.tick); clearInterval(P.timers.save); clearTimeout(P.timers.osd); clearInterval(P.timers.sub); clearInterval(P.timers.next); }

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
    if (!stay && P.btn < 0 && !P.menu) P.timers.osd = setTimeout(function () { if (state() === 'PLAYING' && !queue.busy) o.classList.add('off'); }, 4000);
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
    P.active = false; P.menu = null; P.nextUp = null;
    markBtn(-1);
    $('subMenu').classList.add('hidden');
    $('nextUp').classList.add('hidden');
    $('player').classList.add('hidden');
    document.documentElement.classList.remove('video-mode');
    if (YF.app && YF.app.markDirty) YF.app.markDirty();
    saved.then(function () { if (opts && opts.onExit) opts.onExit(pos, dur, !!completed); });
  }
  P.exit = exit;

  // Teclas durante la reproducción. Devuelve true si la ha usado.
  P.key = function (k) {
    if (!P.active) return false;
    if (P.menu) { menuKey(k); return true; }
    if (P.nextUp) {
      if (k === 'Enter' || k === 'Play' || k === 'PlayPause') playNext();
      else if (k === 'Back' || k === 'Stop') { clearInterval(P.timers.next); P.nextUp = null; exit(true); }
      return true;
    }
    if (P.btn >= 0 && btnKey(k)) return true;
    switch (k) {
      case 'Down': focusBtn(0); break;
      case 'Back': case 'Stop': exit(false); break;
      case 'Enter': case 'PlayPause': case 'Play': case 'Pause': togglePause(); break;
      case 'Left': jump(-10000); break;
      case 'Right': jump(10000); break;
      case 'Rewind': jump(-30000); break;
      case 'FastForward': jump(30000); break;
      case 'Green': nextAudio(); break;
      case 'Yellow': openSubMenu(); break;
      case 'Blue': if (P.opts.next && P.opts.next()) playNext(); else flash('No hay capítulo siguiente'); break;
      default: showOsd();
    }
    return true;
  };

  document.addEventListener('visibilitychange', function () { if (document.hidden && P.active) exit(false); });
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
