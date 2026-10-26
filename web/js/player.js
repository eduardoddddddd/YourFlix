/* Reproductor web: <video> con el archivo tal cual o HLS (hls.js, o nativo en Safari), audio y
 * subtítulos elegibles, barra con saltos, pantalla completa y progreso compartido con la TV. */
(function (YF) {
  'use strict';
  var el = YF.el, api = YF.api, P = YF.playback;
  var PL = YF.player = { active: false };
  var ui = null, st = null, token = 0;

  // ---------- Construcción (una vez) ----------
  function ibtn(name, label, onClick, extra) {
    var b = el('button', 'ibtn' + (extra ? ' ' + extra : ''));
    b.type = 'button';
    b.setAttribute('aria-label', label);
    b.title = label;
    b.appendChild(YF.icon(name));
    if (onClick) b.addEventListener('click', function (e) { e.stopPropagation(); onClick(); });
    return b;
  }

  function setIcon(b, name) { b.replaceChild(YF.icon(name), b.firstChild); }

  function build() {
    var root = el('div', 'hidden');
    root.id = 'player';
    root.setAttribute('role', 'dialog');
    root.setAttribute('aria-label', 'Reproductor');
    root.tabIndex = -1;
    var video = el('video');
    video.setAttribute('playsinline', '');
    video.setAttribute('webkit-playsinline', '');
    video.preload = 'auto';
    if (api.base) video.crossOrigin = 'anonymous';
    root.appendChild(video);
    root.appendChild(el('div', 'pl-shade'));
    var wait = el('div', 'pl-wait hidden');
    root.appendChild(wait);

    var center = el('div', 'pl-center');
    var c10 = ibtn('replay', 'Atrás 10 segundos', function () { skip(-10); });
    c10.appendChild(el('span', 'n', '10'));
    var cplay = ibtn('play', 'Reproducir', togglePlay, 'big');
    var cf10 = ibtn('forward', 'Adelante 10 segundos', function () { skip(10); });
    cf10.appendChild(el('span', 'n', '10'));
    center.appendChild(c10); center.appendChild(cplay); center.appendChild(cf10);
    root.appendChild(center);

    var top = el('div', 'pl-top');
    top.appendChild(ibtn('back', 'Volver', function () { YF.app.leavePlayer(); }));
    var title = el('div', 'pl-title');
    var h2 = el('h2'), sub = el('p');
    title.appendChild(h2); title.appendChild(sub);
    top.appendChild(title);
    root.appendChild(top);

    var bottom = el('div', 'pl-bottom');
    var seek = el('div', 'seek');
    seek.tabIndex = 0;
    seek.setAttribute('role', 'slider');
    seek.setAttribute('aria-label', 'Posición');
    var rail = el('div', 'rail'), buf = el('div', 'buf'), done = el('div', 'done'), knob = el('div', 'knob'), tip = el('div', 'tip hidden');
    rail.appendChild(buf); rail.appendChild(done); rail.appendChild(knob);
    seek.appendChild(rail); seek.appendChild(tip);
    bottom.appendChild(seek);
    var rowEl = el('div', 'pl-row');
    var bplay = ibtn('play', 'Reproducir (espacio)', togglePlay);
    var b10 = ibtn('replay', 'Atrás 10 s (←)', function () { skip(-10); }, 'skip-btn');
    b10.appendChild(el('span', 'n', '10'));
    var bf10 = ibtn('forward', 'Adelante 10 s (→)', function () { skip(10); }, 'skip-btn');
    bf10.appendChild(el('span', 'n', '10'));
    var bvol = ibtn('vol', 'Silenciar (M)', toggleMute);
    var vol = el('input', 'vol');
    vol.type = 'range'; vol.min = 0; vol.max = 1; vol.step = 0.05;
    vol.setAttribute('aria-label', 'Volumen');
    vol.addEventListener('input', function () { video.volume = +vol.value; video.muted = false; YF.storage.set('volume', video.volume); });
    var time = el('span', 'pl-time', '0:00 / 0:00');
    var bnext = ibtn('next', 'Siguiente capítulo (N)', playNext, 'hidden');
    var baudio = ibtn('audio', 'Audio', function () { toggleMenu('audio'); });
    var bsubs = ibtn('subs', 'Subtítulos (C)', function () { toggleMenu('subs'); });
    var bfull = ibtn('full', 'Pantalla completa (F)', toggleFull);
    [bplay, b10, bf10, bvol, vol, time, el('span', 'grow'), bnext, baudio, bsubs, bfull].forEach(function (x) { rowEl.appendChild(x); });
    bottom.appendChild(rowEl);
    root.appendChild(bottom);

    var menu = el('div', 'menu hidden');
    menu.setAttribute('role', 'menu');
    root.appendChild(menu);
    var msg = el('div', 'pl-msg hidden');
    root.appendChild(msg);
    var flash = el('div', 'pl-flash hidden');
    root.appendChild(flash);
    var nextUp = el('div', 'next-up hidden');
    root.appendChild(nextUp);
    document.body.appendChild(root);

    ui = { root: root, video: video, wait: wait, cplay: cplay, h2: h2, sub: sub, seek: seek, buf: buf, done: done, knob: knob, tip: tip, bplay: bplay,
      bvol: bvol, vol: vol, time: time, bnext: bnext, baudio: baudio, bsubs: bsubs, bfull: bfull, menu: menu, msg: msg, flash: flash, nextUp: nextUp };
    wireVideo();
    wireSeek();
    wireSurface();
    document.addEventListener('keydown', onKey);
    document.addEventListener('fullscreenchange', onFullChange);
    document.addEventListener('webkitfullscreenchange', onFullChange);
    window.addEventListener('pagehide', function () { if (PL.active) { saveProgress(true); stopRemux(); } });
  }

  // ---------- Abrir / cerrar ----------
  // start: segundos, o null para reanudar donde se dejó.
  PL.open = function (titleId, fileId, start) {
    if (!ui) build();
    if (PL.active && st && st.f && st.f.id === fileId && st.d.title.id === titleId) return;
    if (PL.active) teardown(true);
    PL.active = true;
    var my = ++token;
    st = { token: my, session: P.session(), lastSave: 0, started: false };
    document.body.classList.add('playing');
    ui.root.classList.remove('hidden');
    hideMsg(); closeMenu(); ui.nextUp.classList.add('hidden');
    ui.h2.textContent = ''; ui.sub.textContent = '';
    showWait(true);
    ui.root.focus && ui.root.focus();
    Promise.all([api.title(titleId), YF.app.caps()]).then(function (r) {
      if (my !== token) return;
      var d = r[0], f = (d.files || []).filter(function (x) { return x.id === fileId; })[0];
      if (!f) { showMsg('Este archivo ya no está disponible.', [{ label: 'Volver', primary: true, action: YF.app.leavePlayer }]); return; }
      st.d = d; st.caps = r[1];
      setFile(f, start == null ? P.resumeAt(f) : start);
    }).catch(function (e) {
      if (my === token) showMsg('No se pudo abrir: ' + e.message, [{ label: 'Volver', primary: true, action: YF.app.leavePlayer }]);
    });
  };

  // Estado para depurar y para la prueba de extremo a extremo (tools/web-demo/e2e.mjs).
  PL.info = function () {
    return st && st.f ? { file: st.f.id, method: st.method, mode: st.mode, variant: st.variant, audio: st.audio, sub: st.sub, hls: st.hls ? 'hlsjs' : st.method === 'hls' ? 'native' : '' } : null;
  };

  PL.close = function () {
    if (!PL.active) return;
    teardown(true);
    PL.active = false;
    token++;
    ui.root.classList.add('hidden');
    document.body.classList.remove('playing');
    if (document.fullscreenElement && document.exitFullscreen) document.exitFullscreen().catch(function () {});
    else if (document.webkitFullscreenElement && document.webkitExitFullscreen) document.webkitExitFullscreen();
    if ('mediaSession' in navigator) navigator.mediaSession.metadata = null;
    document.title = 'YourFlix';
  };

  function setFile(f, start) {
    st.f = f;
    st.duration = f.duration || 0;
    st.audio = P.pickAudio(f, YF.storage.get('audioLang', ''));
    st.sub = null; st.subTracks = []; st.subImages = 0;
    var t = st.d.title;
    ui.h2.textContent = t.title;
    ui.sub.textContent = f.season != null ? YF.epLabel(f) + (f.episodeInfo && f.episodeInfo.name ? ' · ' + f.episodeInfo.name : '') : (t.year || '');
    document.title = t.title + (f.season != null ? ' ' + YF.epLabel(f) : '') + ' · YourFlix';
    ui.bnext.classList.toggle('hidden', !P.nextFile(st.d.files, f));
    ui.baudio.classList.toggle('hidden', (f.audio || []).length < 2);
    mediaSession();
    var my = st.token;
    api.subtitles(f.id).then(function (r) {
      if (my !== token || st.f !== f) return;
      st.subTracks = r.tracks || [];
      st.subImages = r.imageTracks || 0;
      var a = P.audioByIndex(f, st.audio);
      var id = P.pickSubtitle(st.subTracks, YF.storage.get('subPref', null), a ? YF.lang(a.language) : '');
      if (id) setSub(id, true);
    }).catch(function () {});
    load(start);
  }

  // ---------- Fuente: directa o HLS ----------
  function load(start) {
    teardownSource(false); // la conversión anterior la para el servidor al pedir la nueva
    hideMsg();
    var f = st.f, v = ui.video;
    var dec = P.decide(f, st.audio, st.caps);
    st.method = dec.method; st.mode = dec.mode; st.started = false; st.recovered = 0; st.frameTime = 0;
    if (dec.method === 'none') { showWait(false); showMsg(dec.why, [{ label: 'Volver', primary: true, action: YF.app.leavePlayer }]); return; }
    showWait(true);
    st.pendingStart = start > 1 ? start : 0;
    if (dec.method === 'direct') {
      v.src = api.streamURL(f.id) + (st.pendingStart ? '#t=' + Math.floor(st.pendingStart) : '');
    } else {
      st.variant = P.variant(st.audio, dec.mode);
      st.base = P.hlsBase(f.id, st.variant);
      var url = api.url(P.masterURL(f.id, st.variant, st.session, st.pendingStart));
      if (st.caps.hls === 'hlsjs') {
        var Hls = window.Hls;
        var h = st.hls = new Hls(P.hlsConfig(st.pendingStart));
        var my = st.token;
        h.on(Hls.Events.ERROR, function (ev, data) { if (my === token) onHlsError(data); });
        h.on(Hls.Events.MANIFEST_PARSED, function () { if (my === token) tryPlay(); });
        h.loadSource(url);
        h.attachMedia(v);
        return;
      }
      v.src = url + (st.pendingStart ? '#t=' + Math.floor(st.pendingStart) : '');
    }
    v.load();
    tryPlay();
  }

  function tryPlay() {
    var p = ui.video.play();
    if (p && p.catch) p.catch(function (e) {
      if (e && e.name === 'NotAllowedError') { showWait(false); showControls(); updatePlayIcon(); }
    });
  }

  function onHlsError(data) {
    if (!data.fatal) return;
    var Hls = window.Hls, code = data.response && data.response.code;
    if (code === 503) return busy();
    if (code === 409 || code === 501) return showMsg('El servidor no puede convertir este vídeo para el navegador (recodificación desactivada).', [{ label: 'Volver', primary: true, action: YF.app.leavePlayer }]);
    if (data.type === Hls.ErrorTypes.MEDIA_ERROR && st.recovered < 3) return recover();
    failed('No se pudo reproducir (' + (code ? 'HTTP ' + code : data.details) + ').');
  }

  // Rehace el MediaSource y sigue reproduciendo donde iba. recoverMediaError solo vuelve a
  // enganchar el <video>: hls.js arranca entonces en config.startPosition (el punto de inicio
  // original, no donde se iba) y el vídeo se queda en pausa sin que la interfaz lo sepa.
  function recover() {
    var v = ui.video, h = st.hls, t = P.recoverAt(v.currentTime, st.frameTime, st.pendingStart);
    st.recovered++;
    st.recoverPos = t;
    st.frameTime = 0;
    st.pendingStart = t > 1 ? t : 0;
    h.config.startPosition = t > 0 ? t : -1;
    h.recoverMediaError();
    tryPlay();
  }

  function busy() {
    showWait(false);
    YF.api.remux().then(function (s) {
      showMsg('El servidor ya está convirtiendo vídeos para otros reproductores (máximo ' + (s.maxJobs || 2) + ' a la vez). Prueba de nuevo en un momento.',
        [{ label: 'Reintentar', primary: true, action: function () { load(currentTime()); } }, { label: 'Volver', action: YF.app.leavePlayer }]);
    });
  }

  function failed(text) {
    showWait(false);
    showMsg(text, [{ label: 'Reintentar', primary: true, action: function () { load(currentTime()); } }, { label: 'Volver', action: YF.app.leavePlayer }]);
  }

  function teardownSource(stop) {
    var v = ui.video;
    if (st && st.hls) { try { st.hls.destroy(); } catch (_) {} st.hls = null; }
    if (stop) stopRemux();
    if (st) st.base = null;
    v.removeAttribute('src');
    try { v.load(); } catch (_) {}
  }

  function teardown(save) {
    if (!st) return;
    clearTimeout(st.nextTimer);
    if (save) saveProgress(true);
    teardownSource(true);
    removeTracks();
  }

  function stopRemux() { if (st && st.base) api.stopHLS(st.base, st.session); }

  // ---------- Eventos del vídeo ----------
  function currentTime() { return ui.video.currentTime || st.pendingStart || 0; }

  function wireVideo() {
    var v = ui.video;
    v.volume = YF.storage.get('volume', 1);
    v.addEventListener('loadedmetadata', function () {
      if (!st) return;
      if (!st.duration && isFinite(v.duration)) st.duration = v.duration;
      if (st.pendingStart && Math.abs(v.currentTime - st.pendingStart) > 3) v.currentTime = st.pendingStart;
      render();
    });
    v.addEventListener('playing', function () { if (!st) return; st.started = true; st.pendingStart = 0; showWait(false); updatePlayIcon(); scheduleIdle(); });
    v.addEventListener('pause', function () { updatePlayIcon(); showControls(); if (st && st.started) saveProgress(true); });
    v.addEventListener('play', updatePlayIcon);
    v.addEventListener('waiting', function () { showWait(true); });
    v.addEventListener('canplay', function () { showWait(false); });
    v.addEventListener('seeked', function () { showWait(false); if (st && st.started) saveProgress(true); });
    // Tiempo del último fotograma pintado: en el atasco de arranque de Chrome la imagen avanza
    // aunque currentTime no se mueva, y es donde hay que seguir al recuperar (recover).
    if (v.requestVideoFrameCallback) {
      var onFrame = function (now, md) { if (st && !v.seeking) st.frameTime = md.mediaTime; v.requestVideoFrameCallback(onFrame); };
      v.requestVideoFrameCallback(onFrame);
    }
    v.addEventListener('seeking', function () { if (st) st.frameTime = 0; });
    v.addEventListener('timeupdate', function () {
      render();
      if (st && st.recovered && v.currentTime - st.recoverPos > 30) st.recovered = 0; // otra vez 3 recuperaciones
      if (st && st.started && !v.paused && P.shouldSave(st.lastSave, Date.now(), false)) saveProgress(false);
    });
    v.addEventListener('progress', render);
    v.addEventListener('volumechange', function () { setIcon(ui.bvol, v.muted || v.volume === 0 ? 'mute' : 'vol'); ui.vol.value = v.muted ? 0 : v.volume; });
    v.addEventListener('ended', onEnded);
    v.addEventListener('error', function () {
      if (!st || !PL.active || st.hls || !v.getAttribute('src')) return;
      if (st.method === 'direct' && !st.started) {
        // El navegador decía que podía y no puede: probar con el servidor convirtiendo.
        st.caps = Object.assign({}, st.caps, { canPlay: function () { return false; } });
        return load(st.pendingStart);
      }
      failed(st.method === 'hls' ? 'No se pudo reproducir. Puede que el servidor esté ocupado con otras conversiones.' : 'No se pudo reproducir este archivo.');
    });
  }

  function saveProgress(force) {
    if (!st || !st.f || !st.started) return;
    var now = Date.now();
    if (!P.shouldSave(st.lastSave, now, force)) return;
    st.lastSave = now;
    var pos = ui.video.ended ? st.duration : ui.video.currentTime;
    if (pos > 0) api.progress(st.f.id, pos, st.duration || pos, force);
  }

  function onEnded() {
    saveProgress(true);
    var next = P.nextFile(st.d.files, st.f);
    if (!next) { showControls(); return; }
    var n = 10, box = ui.nextUp, my = st.token;
    YF.clear(box);
    box.appendChild(el('div', 'eyebrow', 'A continuación'));
    box.appendChild(el('h3', '', YF.epLabel(next) + (next.episodeInfo && next.episodeInfo.name ? ' · ' + next.episodeInfo.name : '')));
    var p = el('p', '', '');
    box.appendChild(p);
    var btns = el('div', 'btns');
    btns.appendChild(button('Ver ahora', 'primary', playNext));
    btns.appendChild(button('Cancelar', '', function () { clearTimeout(st.nextTimer); box.classList.add('hidden'); }));
    box.appendChild(btns);
    box.classList.remove('hidden');
    (function tick() {
      if (my !== token) return;
      p.textContent = 'Empieza en ' + n + ' s';
      if (n-- <= 0) return playNext();
      st.nextTimer = setTimeout(tick, 1000);
    })();
  }

  function playNext() {
    var next = st && P.nextFile(st.d.files, st.f);
    if (!next) return;
    clearTimeout(st.nextTimer);
    ui.nextUp.classList.add('hidden');
    saveProgress(true);
    removeTracks();
    history.replaceState(null, '', YF.route(['ver', st.d.title.id, next.id]));
    setFile(next, P.resumeAt(next));
  }

  // ---------- Controles ----------
  function togglePlay() {
    var v = ui.video;
    if (!st || st.method === 'none') return;
    if (v.paused || v.ended) tryPlay(); else v.pause();
  }

  function skip(d) {
    var t = Math.max(0, Math.min((st && st.duration || ui.video.duration || 0) - 1, ui.video.currentTime + d));
    ui.video.currentTime = t;
    flash((d > 0 ? '+' : '−') + Math.abs(d) + ' s · ' + YF.clock(t));
    showControls();
  }

  function seekTo(t) {
    if (!st) return;
    ui.video.currentTime = Math.max(0, Math.min((st.duration || 0) - 1, t));
    render();
  }

  function toggleMute() { ui.video.muted = !ui.video.muted; }

  function isFull() { return !!(document.fullscreenElement || document.webkitFullscreenElement); }

  function toggleFull() {
    var r = ui.root, v = ui.video;
    if (isFull()) {
      if (document.exitFullscreen) document.exitFullscreen().catch(function () {}); else if (document.webkitExitFullscreen) document.webkitExitFullscreen();
    } else if (r.requestFullscreen) {
      r.requestFullscreen().then(function () { try { screen.orientation.lock('landscape').catch(function () {}); } catch (_) {} }).catch(function () {});
    } else if (r.webkitRequestFullscreen) r.webkitRequestFullscreen();
    else if (v.webkitEnterFullscreen) v.webkitEnterFullscreen(); // iPhone: pantalla completa nativa (los subtítulos <track> se ven)
  }

  function onFullChange() { if (ui) setIcon(ui.bfull, isFull() ? 'unfull' : 'full'); }

  function updatePlayIcon() {
    var paused = ui.video.paused || ui.video.ended;
    setIcon(ui.bplay, paused ? 'play' : 'pause');
    setIcon(ui.cplay, paused ? 'play' : 'pause');
    ui.bplay.setAttribute('aria-label', paused ? 'Reproducir' : 'Pausa');
    ui.cplay.setAttribute('aria-label', paused ? 'Reproducir' : 'Pausa');
    if ('mediaSession' in navigator) navigator.mediaSession.playbackState = paused ? 'paused' : 'playing';
  }

  function showWait(on) { ui.wait.classList.toggle('hidden', !on); }

  function render() {
    if (!st) return;
    var v = ui.video, d = st.duration || v.duration || 0, t = st.dragging != null ? st.dragging : (v.currentTime || st.pendingStart || 0);
    var pct = d > 0 ? Math.min(100, t / d * 100) : 0;
    ui.done.style.width = pct + '%';
    ui.knob.style.left = pct + '%';
    var bufEnd = 0;
    for (var i = 0; i < v.buffered.length; i++) if (v.buffered.start(i) <= v.currentTime + 1 && v.buffered.end(i) > bufEnd) bufEnd = v.buffered.end(i);
    ui.buf.style.width = (d > 0 ? Math.min(100, bufEnd / d * 100) : 0) + '%';
    ui.time.textContent = YF.clock(t) + ' / ' + YF.clock(d);
    ui.seek.setAttribute('aria-valuemin', '0');
    ui.seek.setAttribute('aria-valuemax', String(Math.floor(d)));
    ui.seek.setAttribute('aria-valuenow', String(Math.floor(t)));
    ui.seek.setAttribute('aria-valuetext', YF.clock(t) + ' de ' + YF.clock(d));
  }

  function wireSeek() {
    var s = ui.seek;
    function frac(e) { var r = s.getBoundingClientRect(); return Math.max(0, Math.min(1, (e.clientX - r.left) / r.width)); }
    function tipAt(e) {
      var f = frac(e);
      ui.tip.textContent = YF.clock(f * (st && st.duration || 0));
      ui.tip.style.left = (f * 100) + '%';
      ui.tip.classList.remove('hidden');
      return f;
    }
    s.addEventListener('pointerdown', function (e) {
      if (!st || !st.duration) return;
      s.setPointerCapture(e.pointerId);
      s.classList.add('drag');
      st.dragging = tipAt(e) * st.duration;
      render();
      showControls();
    });
    s.addEventListener('pointermove', function (e) {
      if (!st || !st.duration) return;
      var f = tipAt(e);
      if (st.dragging != null) { st.dragging = f * st.duration; render(); showControls(); }
    });
    function end(e) {
      if (!st || st.dragging == null) return;
      var t = frac(e) * st.duration;
      st.dragging = null;
      s.classList.remove('drag');
      ui.tip.classList.add('hidden');
      seekTo(t);
    }
    s.addEventListener('pointerup', end);
    s.addEventListener('pointercancel', function () { if (st) st.dragging = null; s.classList.remove('drag'); ui.tip.classList.add('hidden'); render(); });
    s.addEventListener('pointerleave', function () { if (!st || st.dragging == null) ui.tip.classList.add('hidden'); });
    s.addEventListener('keydown', function (e) {
      var k = e.key;
      if (k === 'ArrowLeft' || k === 'ArrowRight') { skip(k === 'ArrowLeft' ? -10 : 10); e.preventDefault(); e.stopPropagation(); }
      else if (k === 'Home') { seekTo(0); e.preventDefault(); }
      else if (k === 'End') { seekTo((st.duration || 0) - 5); e.preventDefault(); }
    });
  }

  // Ratón: clic pausa, doble clic pantalla completa. Táctil: un toque muestra u oculta los
  // controles; dos toques en un lateral saltan 10 s.
  function wireSurface() {
    var r = ui.root, lastTap = 0, lastSide = 0, clickTimer = 0;
    var surface = function (e) { return e.target === ui.video || e.target === r || e.target.classList.contains('pl-shade'); };
    r.addEventListener('pointermove', function (e) { if (e.pointerType === 'mouse') showControls(); });
    r.addEventListener('click', function (e) {
      if (!surface(e) || r.dataset.pointer !== 'mouse') return;
      if (!ui.menu.classList.contains('hidden')) { closeMenu(); return; }
      clearTimeout(clickTimer);
      clickTimer = setTimeout(togglePlay, 230);
    });
    r.addEventListener('dblclick', function (e) { if (surface(e) && r.dataset.pointer === 'mouse') { clearTimeout(clickTimer); toggleFull(); } });
    r.addEventListener('pointerup', function (e) {
      r.dataset.pointer = e.pointerType;
      if (e.pointerType === 'mouse' || !surface(e)) return;
      if (!ui.menu.classList.contains('hidden')) { closeMenu(); return; }
      var now = Date.now(), side = e.clientX < r.clientWidth / 3 ? -1 : e.clientX > r.clientWidth * 2 / 3 ? 1 : 0;
      if (now - lastTap < 320 && side && side === lastSide) { skip(side * 10); lastTap = 0; return; }
      lastTap = now; lastSide = side;
      if (r.classList.contains('idle')) showControls(); else hideControls();
    });
  }

  function showControls() {
    if (!ui) return;
    ui.root.classList.remove('idle');
    scheduleIdle();
  }

  function hideControls() {
    if (ui.video.paused || !ui.menu.classList.contains('hidden') || !ui.msg.classList.contains('hidden')) return;
    ui.root.classList.add('idle');
  }

  function scheduleIdle() {
    clearTimeout(PL.idleTimer);
    PL.idleTimer = setTimeout(hideControls, 3000);
  }

  function onKey(e) {
    if (!PL.active || e.ctrlKey || e.metaKey || e.altKey) return;
    var tag = (e.target && e.target.tagName) || '';
    if (tag === 'INPUT' && e.target.type !== 'range') return;
    var k = e.key, onBtn = tag === 'BUTTON' || tag === 'A';
    if ((k === ' ' || k === 'Enter') && onBtn) return; // el botón enfocado ya lo hace
    var handled = true;
    switch (k) {
      case ' ': case 'k': case 'K': togglePlay(); break;
      case 'ArrowLeft': case 'j': case 'J': if (tag === 'INPUT') return; skip(-10); break;
      case 'ArrowRight': case 'l': case 'L': if (tag === 'INPUT') return; skip(10); break;
      case 'ArrowUp': ui.video.volume = Math.min(1, ui.video.volume + 0.1); ui.video.muted = false; flash('Volumen ' + Math.round(ui.video.volume * 100) + ' %'); break;
      case 'ArrowDown': ui.video.volume = Math.max(0, ui.video.volume - 0.1); flash('Volumen ' + Math.round(ui.video.volume * 100) + ' %'); break;
      case 'f': case 'F': toggleFull(); break;
      case 'm': case 'M': toggleMute(); break;
      case 'c': case 'C': toggleSubs(); break;
      case 'n': case 'N': playNext(); break;
      case 'Escape':
        if (!ui.menu.classList.contains('hidden')) closeMenu();
        else if (!isFull()) YF.app.leavePlayer();
        break;
      default: handled = false;
    }
    if (handled) { e.preventDefault(); showControls(); }
  }

  function flash(text) {
    ui.flash.textContent = text;
    ui.flash.classList.remove('hidden');
    clearTimeout(PL.flashTimer);
    PL.flashTimer = setTimeout(function () { ui.flash.classList.add('hidden'); }, 1300);
  }

  function button(label, cls, action) {
    var b = el('button', 'btn' + (cls ? ' ' + cls : ''), label);
    b.type = 'button';
    b.addEventListener('click', function (e) { e.stopPropagation(); action(); });
    return b;
  }

  function showMsg(text, actions) {
    var m = ui.msg;
    YF.clear(m);
    m.appendChild(el('p', '', text));
    var btns = el('div', 'btns');
    (actions || []).forEach(function (a) { btns.appendChild(button(a.label, a.primary ? 'primary' : '', a.action)); });
    m.appendChild(btns);
    m.classList.remove('hidden');
    ui.root.classList.remove('idle');
    var first = m.querySelector('button');
    if (first) first.focus();
  }

  function hideMsg() { if (ui) ui.msg.classList.add('hidden'); }

  // ---------- Menús de audio y subtítulos ----------
  function closeMenu() { if (ui) { ui.menu.classList.add('hidden'); ui.menu.dataset.kind = ''; } }

  function toggleMenu(kind) {
    if (ui.menu.dataset.kind === kind) return closeMenu();
    var m = ui.menu;
    YF.clear(m);
    m.dataset.kind = kind;
    var item = function (label, on, action) {
      var b = el('button');
      b.type = 'button';
      b.setAttribute('role', 'menuitemradio');
      b.setAttribute('aria-checked', on ? 'true' : 'false');
      b.appendChild(el('span', 'ck', on ? '✓' : ''));
      b.appendChild(el('span', '', label));
      b.addEventListener('click', function (e) { e.stopPropagation(); closeMenu(); action(); });
      m.appendChild(b);
    };
    if (kind === 'audio') {
      m.appendChild(el('h3', '', 'Audio'));
      (st.f.audio || []).forEach(function (a) { item(P.audioLabel(a), a.index === st.audio, function () { setAudio(a.index); }); });
      if (st.method === 'hls') m.appendChild(el('p', '', 'El servidor convierte la pista elegida a AAC estéreo.'));
    } else {
      m.appendChild(el('h3', '', 'Subtítulos'));
      item('Desactivados', !st.sub, function () { setSub(null); YF.storage.set('subPref', 'off'); });
      st.subTracks.forEach(function (t) {
        item(t.label, t.id === st.sub, function () { setSub(t.id); if (!t.forced) YF.storage.set('subPref', YF.lang(t.language)); });
      });
      if (!st.subTracks.length) m.appendChild(el('p', '', 'Este archivo no tiene subtítulos de texto.'));
      if (st.subImages) m.appendChild(el('p', '', st.subImages + ' en imagen (PGS/DVD) no se pueden mostrar.'));
    }
    m.classList.remove('hidden');
    ui.root.classList.remove('idle');
    var first = m.querySelector('button[aria-checked="true"]') || m.querySelector('button');
    if (first) first.focus();
  }

  function setAudio(index) {
    if (index === st.audio) return;
    var a = P.audioByIndex(st.f, index);
    if (a && a.language) YF.storage.set('audioLang', YF.lang(a.language));
    var t = currentTime();
    st.audio = index;
    flash('Audio: ' + P.audioLabel(a));
    load(t);
  }

  function removeTracks() {
    if (!ui) return;
    Array.prototype.slice.call(ui.video.querySelectorAll('track')).forEach(function (t) { t.parentNode.removeChild(t); });
    for (var i = 0; i < ui.video.textTracks.length; i++) ui.video.textTracks[i].mode = 'disabled';
  }

  function setSub(id, quiet) {
    removeTracks();
    st.sub = id;
    ui.bsubs.classList.toggle('on', !!id);
    if (!id) { if (!quiet) flash('Subtítulos desactivados'); return; }
    var info = st.subTracks.filter(function (t) { return t.id === id; })[0] || { label: 'Subtítulos', language: '' };
    var tr = el('track');
    tr.kind = 'subtitles';
    tr.label = info.label;
    tr.srclang = YF.lang(info.language).toLowerCase();
    tr.src = api.vttURL(st.f.id, id);
    tr.default = true;
    var my = st.token;
    tr.addEventListener('load', function () { if (my === token && !quiet) flash('Subtítulos: ' + info.label); });
    tr.addEventListener('error', function () { if (my === token) YF.toast('No se pudieron cargar los subtítulos'); });
    ui.video.appendChild(tr);
    tr.track.mode = 'showing';
    if (!quiet) flash('Cargando subtítulos…');
  }

  function toggleSubs() {
    if (st.sub) { setSub(null); return; }
    var pref = YF.storage.get('subPref', null);
    var t = st.subTracks.filter(function (x) { return !x.forced && YF.lang(x.language) === pref; })[0] || st.subTracks.filter(function (x) { return !x.forced; })[0] || st.subTracks[0];
    if (t) setSub(t.id); else flash('Sin subtítulos de texto');
  }

  function mediaSession() {
    if (!('mediaSession' in navigator) || !window.MediaMetadata) return;
    var d = st.d, f = st.f;
    try {
      navigator.mediaSession.metadata = new MediaMetadata({ title: d.title.title, artist: f.season != null ? YF.epLabel(f) : (d.title.year || ''), album: 'YourFlix',
        artwork: d.poster ? [{ src: api.url(YF.imgWidth(d.poster, 600)), sizes: '600x900', type: 'image/jpeg' }] : [] });
      var ms = navigator.mediaSession;
      ms.setActionHandler('play', function () { tryPlay(); });
      ms.setActionHandler('pause', function () { ui.video.pause(); });
      ms.setActionHandler('seekbackward', function () { skip(-10); });
      ms.setActionHandler('seekforward', function () { skip(10); });
      ms.setActionHandler('seekto', function (e) { seekTo(e.seekTime); });
    } catch (_) {}
  }
})(window.YF = window.YF || {});
