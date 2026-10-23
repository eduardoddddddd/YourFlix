/* YourFlix · app de prueba de la fase 0 (docs/PRUEBAS-TV.md).
 * Reglas heredadas de AstroIPTV: una sola apertura a la vez, nunca stop/close
 * con un prepareAsync pendiente, nunca setSelectTrack('TEXT'), token por apertura.
 * Cada llamada nativa se traza y se envía al servidor antes de ejecutarse:
 * si AVPlay se cuelga, el registro del servidor dice dónde. */
(function () {
  'use strict';

  var params = new URLSearchParams(location.search);
  var SERVER = params.get('server') || (location.protocol.indexOf('http') === 0 ? location.origin : 'http://192.168.1.3:8787');
  var av = window.webapis && window.webapis.avplay;
  var $ = function (id) { return document.getElementById(id); };

  var state = {
    files: [], sel: 0, results: {}, mode4k: false,
    token: 0, opened: false, preparing: false, closeAfterPrepare: false,
    file: null, duration: 0, playing: false, manual: false, auto: false,
    subsOn: false, subCount: 0, bufferings: 0, audioTracks: [], audioPos: 0, timer: 0, backAt: 0
  };

  // ---------- registro ----------
  var queue = [], sending = false, lines = [];
  function flush() {
    if (sending || !queue.length) return;
    sending = true;
    var batch = queue.splice(0, queue.length);
    var xhr = new XMLHttpRequest();
    xhr.open('POST', SERVER + '/api/log');
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.onloadend = function () { sending = false; flush(); };
    xhr.send(JSON.stringify(batch));
  }
  function send(ev) { ev.ms = Date.now(); queue.push(ev); flush(); }
  function trace(msg, extra) {
    var ev = extra || {};
    ev.kind = ev.kind || 'trace'; ev.msg = msg;
    if (state.file) ev.file = state.file.name;
    send(ev);
    lines.push(new Date().toTimeString().slice(0, 8) + ' ' + msg);
    if (lines.length > 26) lines.shift();
    $('log').textContent = lines.join('\n');
  }
  function call(name) {
    var args = Array.prototype.slice.call(arguments, 1);
    trace('[av] ' + name + ' ' + args.filter(function (a) { return typeof a !== 'function'; }).join(' '));
    var r = av[name].apply(av, args);
    trace('[av] ' + name + ' done');
    return r;
  }

  // ---------- utilidades ----------
  function sleep(ms) { return new Promise(function (r) { setTimeout(r, ms); }); }
  function withTimeout(p, ms, what) {
    return new Promise(function (resolve, reject) {
      var t = setTimeout(function () { reject(new Error('tiempo agotado: ' + what)); }, ms);
      p.then(function (v) { clearTimeout(t); resolve(v); }, function (e) { clearTimeout(t); reject(e); });
    });
  }
  function now() { try { return av.getCurrentTime(); } catch (_) { return -1; } }
  function avState() { try { return av.getState(); } catch (_) { return '?'; } }
  function fmt(ms) {
    if (!(ms >= 0)) return '--:--';
    var s = Math.floor(ms / 1000), h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60;
    return (h ? h + ':' : '') + (m < 10 ? '0' : '') + m + ':' + (s % 60 < 10 ? '0' : '') + (s % 60);
  }
  function mb(n) { return n >= 1e9 ? (n / 1e9).toFixed(2) + ' GB' : (n / 1e6).toFixed(1) + ' MB'; }
  function el(tag, cls, text) { var e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; }
  async function waitFor(pred, timeout, step) {
    var end = Date.now() + timeout;
    while (Date.now() < end) { if (pred()) return true; await sleep(step || 250); }
    return pred();
  }

  // ---------- AVPlay serializado ----------
  function listener(token) {
    var mine = function () { return token === state.token; };
    return {
      onbufferingstart: function () { if (mine()) { state.bufferings++; trace('buffering start'); setAction('Cargando…'); } },
      onbufferingprogress: function (p) { if (mine() && (p === 0 || p === 100)) trace('buffering ' + p + '%'); },
      onbufferingcomplete: function () { if (mine()) { trace('buffering complete'); setAction(''); } },
      onstreamcompleted: function () { if (mine()) { trace('stream completed'); if (state.manual) stopPlayback(); } },
      onerror: function (e) { if (mine()) trace('onerror ' + e, { kind: 'error' }); },
      onevent: function (type, data) { if (mine()) trace('onevent ' + type + ' ' + data); },
      oncurrentplaytime: function () {},
      onsubtitlechange: function (duration, text) {
        if (!mine()) return;
        state.subCount++;
        if (state.subCount <= 5) trace('subtítulo (' + duration + ' ms): ' + String(text).slice(0, 80));
        if (state.subsOn) showSub(text, duration);
      }
    };
  }

  function nativeClose() {
    if (!state.opened) return;
    if (state.preparing) { state.closeAfterPrepare = true; trace('cierre aplazado: prepare pendiente'); return; }
    try { call('stop'); } catch (e) { trace('stop: ' + e.message); }
    try { call('close'); } catch (e) { trace('close: ' + e.message); }
    state.opened = false; state.playing = false;
  }

  function open(file) {
    if (state.preparing) return Promise.reject(new Error('hay un prepare pendiente'));
    nativeClose();
    var token = ++state.token;
    state.file = file; state.subCount = 0; state.duration = 0;
    var url = SERVER + file.url;
    call('open', url);
    state.opened = true;
    call('setDisplayRect', 0, 0, 1920, 1080);
    try { call('setDisplayMethod', 'PLAYER_DISPLAY_MODE_LETTER_BOX'); } catch (e) { trace('setDisplayMethod: ' + e.message); }
    if (state.mode4k) { try { call('setStreamingProperty', 'SET_MODE_4K', 'TRUE'); } catch (e) { trace('SET_MODE_4K: ' + e.message, { kind: 'error' }); } }
    av.setListener(listener(token));
    state.preparing = true;
    var t0 = Date.now();
    return new Promise(function (resolve, reject) {
      trace('[av] prepareAsync');
      av.prepareAsync(function () {
        state.preparing = false;
        trace('[av] prepareAsync done ' + (Date.now() - t0) + ' ms');
        if (state.closeAfterPrepare || token !== state.token) { state.closeAfterPrepare = false; nativeClose(); reject(new Error('cancelado')); return; }
        try { state.duration = av.getDuration(); } catch (_) {}
        resolve(Date.now() - t0);
      }, function (e) {
        state.preparing = false;
        trace('[av] prepareAsync error ' + e, { kind: 'error' });
        if (state.closeAfterPrepare) { state.closeAfterPrepare = false; }
        nativeClose();
        reject(new Error('prepare: ' + e));
      });
    });
  }

  function avAsync(name, arg, what) {
    return withTimeout(new Promise(function (resolve, reject) {
      trace('[av] ' + name + ' ' + arg);
      av[name](arg, function () { trace('[av] ' + name + ' done'); resolve(); }, function (e) { trace('[av] ' + name + ' error ' + e, { kind: 'error' }); reject(new Error(name + ': ' + e)); });
    }), 15000, what || name);
  }

  function readTracks() {
    var total = [], current = [];
    try { total = av.getTotalTrackInfo() || []; } catch (e) { trace('getTotalTrackInfo: ' + e.message); }
    try { current = av.getCurrentStreamInfo() || []; } catch (e) { trace('getCurrentStreamInfo: ' + e.message); }
    var plain = function (arr) { var out = []; for (var i = 0; i < arr.length; i++) out.push({ index: arr[i].index, type: arr[i].type, extra: arr[i].extra_info }); return out; };
    var info = { total: plain(total), current: plain(current) };
    trace('pistas: ' + info.total.map(function (t) { return t.type + '#' + t.index; }).join(', '), { kind: 'tracks', tracks: info });
    state.audioTracks = info.total.filter(function (t) { return t.type === 'AUDIO'; });
    return info;
  }

  // ---------- reproducción manual ----------
  async function playManual(file) {
    state.manual = true;
    showHud(file);
    try {
      await withTimeout(open(file), 30000, 'prepare');
      readTracks();
      call('setSilentSubtitle', !state.subsOn);
      call('play'); state.playing = true;
      startTicker();
    } catch (e) {
      trace('error al abrir: ' + e.message, { kind: 'error' });
      setAction('Error: ' + e.message);
    }
  }

  function stopPlayback() {
    stopTicker();
    nativeClose();
    state.manual = false;
    $('hud').classList.add('hidden');
    document.documentElement.classList.remove('video-mode');
    renderList();
  }

  async function seekPercent(p) {
    if (!state.duration) return;
    var target = Math.floor(state.duration * p);
    setAction('Ir a ' + fmt(target));
    try { await avAsync('seekTo', target, 'seekTo'); } catch (e) { setAction(e.message); }
  }
  async function jump(ms) {
    setAction((ms > 0 ? '+' : '') + ms / 1000 + ' s');
    try { await avAsync(ms > 0 ? 'jumpForward' : 'jumpBackward', Math.abs(ms)); } catch (e) { setAction(e.message); }
  }
  function nextAudio() {
    if (state.audioTracks.length < 2) { setAction('Solo una pista de audio'); return; }
    state.audioPos = (state.audioPos + 1) % state.audioTracks.length;
    var t = state.audioTracks[state.audioPos], t0 = Date.now();
    try { call('setSelectTrack', 'AUDIO', t.index); setAction('Audio #' + t.index + ' (' + (Date.now() - t0) + ' ms)'); }
    catch (e) { trace('setSelectTrack AUDIO: ' + e.message, { kind: 'error' }); }
  }
  function toggleSubs() {
    state.subsOn = !state.subsOn;
    try { call('setSilentSubtitle', !state.subsOn); } catch (e) { trace('setSilentSubtitle: ' + e.message); }
    if (!state.subsOn) $('sub').classList.add('hidden');
    setAction('Subtítulos ' + (state.subsOn ? 'sí' : 'no'));
  }
  var subTimer = 0;
  function showSub(text, duration) {
    var s = $('sub');
    s.textContent = String(text).replace(/<[^>]*>/g, '');
    s.classList.remove('hidden');
    clearTimeout(subTimer);
    subTimer = setTimeout(function () { s.classList.add('hidden'); }, Math.max(500, duration || 2000));
  }
  function togglePause() {
    var st = avState();
    try { if (st === 'PLAYING') call('pause'); else if (st === 'PAUSED' || st === 'READY') call('play'); } catch (e) { trace(e.message); }
  }

  // ---------- HUD ----------
  function showHud(file) {
    document.documentElement.classList.add('video-mode');
    $('hud').classList.remove('hidden', 'quiet');
    $('sub').classList.add('hidden');
    $('hudTitle').textContent = file.name;
    setAction('Abriendo…');
  }
  function setAction(text) { $('hudAction').textContent = text; }
  function tick() {
    var t = now();
    $('hudTime').textContent = fmt(t) + ' / ' + fmt(state.duration);
    $('hudState').textContent = avState() + (state.mode4k ? ' · 4K' : '');
    $('hudBar').style.width = state.duration ? Math.min(100, t / state.duration * 100) + '%' : '0';
  }
  function startTicker() { stopTicker(); state.timer = setInterval(tick, 500); tick(); }
  function stopTicker() { clearInterval(state.timer); state.timer = 0; }

  // ---------- prueba automática ----------
  async function autoTest(file) {
    state.auto = true; state.manual = false;
    var steps = [], res = { file: file.name, mode4k: state.mode4k, steps: steps };
    var step = async function (name, fn) {
      var t0 = Date.now(), r = { step: name };
      try { var detail = await fn(); r.ok = true; if (detail !== undefined) r.detail = detail; }
      catch (e) { r.ok = false; r.detail = e.message; }
      r.ms = Date.now() - t0; steps.push(r);
      trace((r.ok ? '✓ ' : '✗ ') + name + ' ' + r.ms + ' ms' + (r.detail !== undefined ? ' · ' + JSON.stringify(r.detail) : ''), { kind: 'step', step: r });
      setAction(name + (r.ok ? ' ✓' : ' ✗'));
      return r.ok;
    };
    var advancing = async function () {
      var a = now(); await sleep(2500); var b = now();
      if (!(b > a)) throw new Error('no avanza (' + a + ' → ' + b + ')');
      return { at: fmt(b), t: b };
    };
    // Tras un salto, AVPlay vuelve a llenar el búfer: con archivos de mucha tasa (4K por Wi-Fi)
    // tarda varios segundos. Se espera a que el tiempo vuelva a avanzar y se mide cuánto tardó;
    // saltar otra vez antes de eso devuelve PLAYER_ERROR_SEEK_FAILED.
    var near = async function (target, slack) {
      var t0 = Date.now();
      var ok = await waitFor(function () { var t = now(); return t >= target - 3000 && t <= target + (slack || 10000); }, 12000);
      if (!ok) throw new Error('posición ' + fmt(now()) + ', esperada ' + fmt(target));
      var p = now();
      if (!(await waitFor(function () { return now() > p + 500; }, 20000))) throw new Error('no reanuda en 20 s (' + fmt(p) + ')');
      return { at: fmt(now()), reanuda: Date.now() - t0 };
    };

    showHud(file);
    startTicker();
    trace('=== prueba automática: ' + file.name + (state.mode4k ? ' (SET_MODE_4K)' : ''), { kind: 'begin' });
    var opened = await step('prepare', function () { return withTimeout(open(file), 30000, 'prepare').then(function (ms) { return { ms: ms, duration: state.duration }; }); });
    if (opened) {
      await step('pistas', function () { return Promise.resolve(readTracks().total.length); });
      try { call('setSilentSubtitle', true); } catch (_) {}
      var played = await step('arranque', async function () {
        call('play'); state.playing = true;
        if (!(await waitFor(function () { return now() >= 2000; }, 20000))) throw new Error('no llega a 2 s (t=' + now() + ')');
        return advancing();
      });
      if (played) {
        // Reproducción continua: detecta si el caudal (Wi-Fi) no da para la tasa del archivo.
        await step('continuo 20 s', async function () {
          var a = now(), b0 = state.bufferings;
          await sleep(20000);
          var adv = now() - a, cuts = state.bufferings - b0;
          if (adv < 18000) throw new Error('solo avanzó ' + (adv / 1000).toFixed(1) + ' s, ' + cuts + ' cortes');
          return { avanzado: Math.round(adv / 100) / 10, cortes: cuts };
        });
      }
      if (played && state.duration > 30000) {
        var mid = Math.floor(state.duration * 0.5);
        await step('salto 50 %', async function () { await avAsync('seekTo', mid); return near(mid); });
        var before = now();
        await step('+30 s', async function () { await avAsync('jumpForward', 30000); return near(before + 30000); });
        before = now();
        await step('−30 s', async function () { await avAsync('jumpBackward', 30000); return near(before - 30000); });
        var far = Math.floor(state.duration * 0.9);
        await step('salto 90 %', async function () { await avAsync('seekTo', far); return near(far); });
        await step('volver al 10 %', async function () { var t = Math.floor(state.duration * 0.1); await avAsync('seekTo', t); return near(t); });
      }
      if (played && state.audioTracks.length > 1) {
        await step('cambio de audio', async function () {
          var t = state.audioTracks[1], t0 = Date.now();
          call('setSelectTrack', 'AUDIO', t.index);
          var ms = Date.now() - t0;
          var adv = await advancing();
          return { track: t.index, callMs: ms, at: adv.at };
        });
      }
      var hasText = (file.tracks || []).some(function (t) { return t.type === 'subtitle'; });
      if (played && hasText) {
        await step('subtítulos (onsubtitlechange)', async function () {
          state.subCount = 0; state.subsOn = true;
          call('setSilentSubtitle', false);
          await sleep(12000);
          call('setSilentSubtitle', true); state.subsOn = false;
          return { recibidos: state.subCount };
        });
      }
    }
    stopTicker();
    nativeClose();
    if (state.preparing) await waitFor(function () { return !state.preparing; }, 30000);
    res.ok = steps.every(function (s) { return s.ok; });
    state.results[file.id + (state.mode4k ? ':4k' : '')] = res;
    trace('=== resultado ' + file.name + ': ' + (res.ok ? 'OK' : 'FALLOS'), { kind: 'result', result: res });
    state.auto = false;
    $('hud').classList.add('hidden');
    document.documentElement.classList.remove('video-mode');
    renderList(); renderDetail();
    return res;
  }

  async function autoAll() {
    // El DTS al final: es el que más probablemente deja AVPlay en mal estado.
    var order = state.files.slice().sort(function (a, b) { return (/dts/i.test(a.name) ? 1 : 0) - (/dts/i.test(b.name) ? 1 : 0); });
    for (var i = 0; i < order.length; i++) {
      state.sel = state.files.indexOf(order[i]);
      await autoTest(order[i]);
      await sleep(1500);
    }
    trace('=== matriz completa', { kind: 'done' });
  }

  // ---------- catálogo ----------
  function renderList() {
    var list = $('list');
    while (list.firstChild) list.removeChild(list.firstChild);
    state.files.forEach(function (f, i) {
      var row = el('div', 'item' + (i === state.sel ? ' sel' : ''));
      row.appendChild(document.createTextNode(f.name));
      row.appendChild(el('small', '', mb(f.size)));
      var r = state.results[f.id] || state.results[f.id + ':4k'];
      if (r) row.appendChild(el('span', 'res', r.ok ? '✓' : '✗ ' + r.steps.filter(function (s) { return !s.ok; }).length));
      list.appendChild(row);
    });
  }
  function renderDetail() {
    var d = $('detail');
    while (d.firstChild) d.removeChild(d.firstChild);
    var f = state.files[state.sel];
    if (!f) return;
    var line = function (k, v) { var r = el('div'); r.appendChild(el('b', '', k + ' ')); r.appendChild(document.createTextNode(v)); d.appendChild(r); };
    line('Archivo', f.name);
    line('Tamaño', mb(f.size) + ' · ' + Math.round(f.bitrate / 1e6) + ' Mb/s · ' + fmt(f.duration * 1000));
    (f.tracks || []).forEach(function (t) {
      var desc = t.codec + (t.profile ? ' ' + t.profile : '') + (t.pixFmt ? ' ' + t.pixFmt : '') + (t.width ? ' ' + t.width + '×' + t.height : '') + (t.channels ? ' ' + t.channels + ' canales' : '') + (t.language ? ' [' + t.language + ']' : '') + (t.title ? ' «' + t.title + '»' : '');
      line(t.type + ' #' + t.index, desc);
    });
    var r = state.results[f.id] || state.results[f.id + ':4k'];
    if (r) r.steps.forEach(function (s) { line((s.ok ? '✓ ' : '✗ ') + s.step, s.ms + ' ms' + (s.detail !== undefined ? ' · ' + JSON.stringify(s.detail) : '')); });
  }
  function loadFiles() {
    $('server').textContent = SERVER;
    var xhr = new XMLHttpRequest();
    xhr.open('GET', SERVER + '/api/files');
    xhr.timeout = 8000;
    xhr.onload = function () {
      try { state.files = JSON.parse(xhr.responseText) || []; } catch (_) { state.files = []; }
      trace('servidor OK: ' + state.files.length + ' archivos · ' + navigator.userAgent, { kind: 'hello' });
      renderList(); renderDetail();
    };
    xhr.onerror = xhr.ontimeout = function () { $('detail').textContent = 'No responde el servidor ' + SERVER; setTimeout(loadFiles, 5000); };
    xhr.send();
  }

  // ---------- mando ----------
  var KEYS = ['MediaPlayPause', 'MediaPlay', 'MediaPause', 'MediaStop', 'MediaFastForward', 'MediaRewind',
    'ColorF0Red', 'ColorF1Green', 'ColorF2Yellow', 'ColorF3Blue', 'Info', '0', '1', '5', '9'];
  function registerKeys() {
    if (!window.tizen || !tizen.tvinputdevice) return;
    KEYS.forEach(function (k) { try { tizen.tvinputdevice.registerKey(k); } catch (_) {} });
  }
  document.addEventListener('keydown', function (e) {
    var k = e.keyCode;
    if (state.auto) { if (k === 10009) e.preventDefault(); return; }
    if (state.manual) {
      e.preventDefault();
      if (k === 10009 || k === 413) stopPlayback();
      else if (k === 37 || k === 412) jump(-30000);
      else if (k === 39 || k === 417) jump(30000);
      else if (k === 13 || k === 10252 || k === 415 || k === 19) togglePause();
      else if (k === 49) seekPercent(0.1);
      else if (k === 53) seekPercent(0.5);
      else if (k === 57) seekPercent(0.9);
      else if (k === 48) seekPercent(0);
      else if (k === 404) nextAudio();
      else if (k === 405) toggleSubs();
      else if (k === 457 || k === 38 || k === 40) $('hud').classList.toggle('quiet');
      return;
    }
    if (k === 38 || k === 40) {
      state.sel = Math.max(0, Math.min(state.files.length - 1, state.sel + (k === 40 ? 1 : -1)));
      renderList(); renderDetail(); e.preventDefault();
    } else if (k === 13 && state.files[state.sel]) playManual(state.files[state.sel]);
    else if (k === 403 && state.files[state.sel]) autoTest(state.files[state.sel]);
    else if (k === 406) autoAll();
    else if (k === 405) { state.mode4k = !state.mode4k; $('mode4k').textContent = state.mode4k ? 'sí' : 'no'; }
    else if (k === 10009) {
      e.preventDefault();
      if (Date.now() - state.backAt < 2000) { try { tizen.application.getCurrentApplication().exit(); } catch (_) {} }
      state.backAt = Date.now();
    }
  });

  // Ganchos para conducir la prueba desde el inspector remoto (CDP).
  window.spike = { state: state, autoTest: function (i) { return autoTest(state.files[i]); }, autoAll: autoAll, setMode4k: function (v) { state.mode4k = !!v; $('mode4k').textContent = v ? 'sí' : 'no'; } };

  document.addEventListener('visibilitychange', function () { if (document.hidden && state.manual) stopPlayback(); });
  registerKeys();
  loadFiles();
})();
