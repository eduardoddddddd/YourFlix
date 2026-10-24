/* YourFlix · web de administración. JavaScript sin dependencias ni CDN.
   Datos siempre con textContent (nunca innerHTML). Todo lo que cambia algo va por JSON
   con la cabecera X-CSRF-Token de la sesión (docs/API.md, «Administración»). */
(function () {
  'use strict';

  var S = { session: null, csrf: '', view: '', jobs: [], seenDone: {}, es: null, poll: null };
  var main = document.getElementById('main');
  var KINDS = { auto: 'Automático', movies: 'Películas', series: 'Series' };
  var STATES = { active: 'activa', paused: 'pausada', offline: 'desconectada' };
  var COMPAT = { direct: 'directo', warn: 'con avisos', optimize: 'optimizar' };
  var JOBSTATE = { queued: 'en cola', running: 'en marcha', done: 'terminado', failed: 'falló' };

  // ---------- utilidades ----------

  // el('div', {class: 'x', onclick: fn}, 'texto', otroNodo…)
  function el(tag, attrs) {
    var n = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        var v = attrs[k];
        if (v === undefined || v === null || v === false) return;
        if (k === 'class') n.className = v;
        else if (k.indexOf('on') === 0 && typeof v === 'function') n.addEventListener(k.slice(2), v);
        else if (k === 'text') n.textContent = v;
        else if (v === true) n.setAttribute(k, '');
        else n.setAttribute(k, v);
      });
    }
    for (var i = 2; i < arguments.length; i++) add(n, arguments[i]);
    return n;
  }
  function add(n, c) {
    if (c === undefined || c === null || c === false) return;
    if (Array.isArray(c)) { c.forEach(function (x) { add(n, x); }); return; }
    n.appendChild(typeof c === 'object' ? c : document.createTextNode(String(c)));
  }
  function clear(n) { while (n.firstChild) n.removeChild(n.firstChild); return n; }
  function setMain() { clear(main); for (var i = 0; i < arguments.length; i++) add(main, arguments[i]); }

  function api(method, url, body) {
    var opt = { method: method, credentials: 'same-origin', headers: {} };
    if (method !== 'GET') {
      opt.headers['Content-Type'] = 'application/json';
      opt.headers['X-CSRF-Token'] = S.csrf;
      opt.body = JSON.stringify(body || {});
    }
    return fetch('/admin/api/' + url, opt).then(function (r) {
      if (r.status === 204) return null;
      return r.json().catch(function () { return {}; }).then(function (data) {
        if (!r.ok) {
          var e = new Error((data && data.error) || ('Error ' + r.status));
          e.status = r.status;
          if (r.status === 401 && url !== 'login' && url !== 'password') onLoggedOut();
          throw e;
        }
        return data;
      });
    });
  }

  var toastTimer = 0;
  function toast(msg, bad) {
    var t = document.getElementById('toast');
    t.textContent = msg;
    t.className = bad ? 'bad' : '';
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { t.className = 'hidden'; }, bad ? 7000 : 3500);
  }
  function fail(e) { toast(e.message || String(e), true); }

  function fmtBytes(n) {
    if (!n && n !== 0) return '—';
    var u = ['B', 'KB', 'MB', 'GB', 'TB'], i = 0;
    while (n >= 1000 && i < u.length - 1) { n /= 1000; i++; }
    return n.toLocaleString('es-ES', { maximumFractionDigits: n < 10 ? 1 : 0 }) + ' ' + u[i];
  }
  function fmtDate(s) {
    if (!s) return '—';
    var d = new Date(s);
    if (isNaN(d)) return s;
    return d.toLocaleString('es-ES', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
  }
  function ago(s) {
    if (!s) return 'nunca';
    var sec = Math.round((Date.now() - new Date(s).getTime()) / 1000);
    if (isNaN(sec)) return s;
    if (sec < 60) return 'hace un momento';
    if (sec < 3600) return 'hace ' + Math.round(sec / 60) + ' min';
    if (sec < 86400) return 'hace ' + Math.round(sec / 3600) + ' h';
    return 'hace ' + Math.round(sec / 86400) + ' días';
  }
  function fmtUptime(sec) {
    var d = Math.floor(sec / 86400), h = Math.floor(sec % 86400 / 3600), m = Math.floor(sec % 3600 / 60);
    return (d ? d + ' d ' : '') + (d || h ? h + ' h ' : '') + m + ' min';
  }
  // bar: barra de uso; el ancho va por CSSOM (la CSP no deja atributos style).
  function bar(frac, low) {
    var s = el('span', { class: low ? 'low' : '' });
    s.style.width = Math.round(frac * 100) + '%';
    return el('div', { class: 'bar' }, s);
  }
  function badge(cls, text) { return el('span', { class: 'badge ' + cls, text: text }); }
  function epLabel(f) {
    if (f.season == null || f.episode == null) return '';
    return f.season + '×' + (f.episode < 10 ? '0' : '') + f.episode + ' ';
  }

  // ---------- diálogo ----------

  function modal(title, body, buttons) {
    var card = clear(document.getElementById('modalCard'));
    var foot = el('div', { class: 'modal-foot' });
    add(card, [el('h2', { text: title }), body, foot]);
    (buttons || [{ text: 'Cerrar' }]).forEach(function (b) {
      add(foot, el('button', { class: 'btn ' + (b.cls || ''), type: 'button', text: b.text, onclick: function () {
        if (b.click && b.click() === false) return;
        closeModal();
      } }));
    });
    document.getElementById('modal').className = '';
    var f = card.querySelector('input,select,textarea,button');
    if (f) f.focus();
  }
  function closeModal() { document.getElementById('modal').className = 'hidden'; }
  document.getElementById('modal').addEventListener('click', function (e) { if (e.target.id === 'modal') closeModal(); });
  document.addEventListener('keydown', function (e) { if (e.key === 'Escape') closeModal(); });

  function confirmBox(title, text, okText, danger) {
    return new Promise(function (resolve) {
      var done = false;
      modal(title, el('p', { text: text }), [
        { text: 'Cancelar', click: function () { done = true; resolve(false); } },
        { text: okText, cls: danger ? 'danger' : 'primary', click: function () { done = true; resolve(true); } }
      ]);
      var obs = setInterval(function () {
        if (document.getElementById('modal').className === 'hidden') { clearInterval(obs); if (!done) resolve(false); }
      }, 200);
    });
  }

  // ---------- sesión ----------

  function loadSession() {
    return api('GET', 'session').then(function (s) {
      S.session = s;
      S.csrf = s.csrf || '';
      renderWho();
      if (!s.authenticated) { stopJobs(); renderLogin(); return; }
      document.getElementById('tabs').className = '';
      startJobs();
      route();
      refreshPendingCount();
    }).catch(function (e) { setMain(el('p', { class: 'err', text: 'No se pudo hablar con el servidor: ' + e.message })); });
  }

  function onLoggedOut() {
    if (S.session && S.session.authenticated) { S.session.authenticated = false; loadSession(); }
  }

  function renderWho() {
    var w = clear(document.getElementById('who'));
    var s = S.session;
    if (!s || !s.authenticated) return;
    add(w, s.local ? badge('gold', 'en este equipo') : badge('active', 'sesión iniciada'));
    if (!s.local) add(w, el('button', { class: 'btn small', type: 'button', text: 'Salir', onclick: logout }));
  }

  function logout() {
    api('POST', 'logout').then(function () { S.csrf = ''; stopJobs(); loadSession(); }).catch(fail);
  }

  function renderLogin() {
    document.getElementById('tabs').className = 'hidden';
    var s = S.session;
    if (!s.passwordSet) {
      setMain(el('div', { class: 'card login gold' },
        el('h1', { text: 'Administración' }),
        el('p', { text: 'Aún no hay contraseña de administración. Por seguridad, la primera se define desde el propio servidor:' }),
        el('ul', null,
          el('li', null, 'abre ', el('code', { text: 'http://localhost:8787/admin/' }), ' en el equipo del servidor (el ROG Ally), o'),
          el('li', null, 'ejecuta ', el('code', { text: 'yourflix admin password' }), '.')),
        el('p', { class: 'muted', text: 'Después podrás entrar desde cualquier equipo de casa con esa contraseña.' })));
      return;
    }
    var pw = el('input', { type: 'password', autocomplete: 'current-password', required: true });
    var err = el('p', { class: 'err hidden' });
    var form = el('form', { class: 'card login', onsubmit: function (e) {
      e.preventDefault();
      api('POST', 'login', { password: pw.value }).then(function () { pw.value = ''; loadSession(); })
        .catch(function (x) { err.textContent = x.message; err.className = 'err'; pw.select(); });
    } },
      el('span', { class: 'eyebrow', text: 'YourFlix' }),
      el('h1', { text: 'Administración' }),
      el('p', { class: 'muted', text: 'Escribe la contraseña de administración del servidor.' }),
      el('label', null, 'Contraseña', pw), err,
      el('button', { class: 'btn primary', type: 'submit', text: 'Entrar' }));
    setMain(form);
    pw.focus();
  }

  // ---------- navegación ----------

  var VIEWS = { fuentes: viewSources, catalogo: viewCatalog, compatibilidad: viewCompat, estado: viewStatus };
  function route() {
    if (!S.session || !S.session.authenticated) return;
    var v = (location.hash || '#fuentes').slice(1);
    if (!VIEWS[v]) v = 'fuentes';
    S.view = v;
    Array.prototype.forEach.call(document.querySelectorAll('#tabs a'), function (a) {
      a.className = a.getAttribute('data-view') === v ? 'active' : '';
    });
    VIEWS[v]();
  }
  window.addEventListener('hashchange', route);

  function refreshPendingCount() {
    api('GET', 'pending').then(function (d) {
      var c = document.getElementById('pendingCount');
      c.textContent = d.items.length;
      c.className = d.items.length ? 'count' : 'count hidden';
    }).catch(function () {});
  }

  // ---------- trabajos en vivo (SSE, con sondeo de respaldo) ----------

  function startJobs() {
    stopJobs();
    if (window.EventSource) {
      S.es = new EventSource('/admin/api/jobs/events');
      S.es.addEventListener('jobs', function (e) { try { onJobs(JSON.parse(e.data).jobs); } catch (x) { /* nada */ } });
      S.es.onerror = function () {
        if (S.es && S.es.readyState === 2) { S.es = null; startPolling(); }
      };
    } else {
      startPolling();
    }
  }
  function startPolling() {
    if (S.poll) return;
    S.poll = setInterval(function () { api('GET', 'jobs').then(function (d) { onJobs(d.jobs); }).catch(function () {}); }, 1500);
  }
  function stopJobs() {
    if (S.es) { S.es.close(); S.es = null; }
    if (S.poll) { clearInterval(S.poll); S.poll = null; }
    document.getElementById('jobs').className = 'hidden';
  }

  var jobsClosedAt = '';
  function onJobs(jobs) {
    var first = !S.jobsLoaded;
    S.jobsLoaded = true;
    S.jobs = jobs || [];
    var finished = false;
    S.jobs.forEach(function (j) {
      if ((j.state === 'done' || j.state === 'failed') && !S.seenDone[j.id]) {
        S.seenDone[j.id] = true;
        if (!first) {
          finished = true;
          if (j.state === 'failed') toast(j.title + ': ' + j.error, true);
          else toast(j.title + ': terminado');
        }
      }
    });
    renderJobs();
    if (finished) {
      refreshPendingCount();
      if (S.view === 'fuentes' || S.view === 'estado' || S.view === 'catalogo') route();
    }
  }

  function renderJobs() {
    var box = clear(document.getElementById('jobs'));
    var active = S.jobs.filter(function (j) { return j.state === 'queued' || j.state === 'running'; });
    var recent = S.jobs.filter(function (j) { return (j.state === 'done' || j.state === 'failed') && j.finishedAt > jobsClosedAt && Date.now() - new Date(j.finishedAt) < 120000; });
    var show = active.concat(recent.slice(0, 2));
    if (!show.length) { box.className = 'hidden'; return; }
    box.className = '';
    add(box, el('button', { class: 'btn small close', type: 'button', text: 'Ocultar', onclick: function () {
      jobsClosedAt = new Date().toISOString(); box.className = 'hidden';
    } }));
    add(box, el('span', { class: 'eyebrow', text: 'Trabajos' }));
    show.forEach(function (j) {
      var head = el('div', { class: 'jt' }, el('b', null, j.state === 'running' ? el('span', { class: 'spin' }) : null, ' ', j.title), badge(j.state, JOBSTATE[j.state] || j.state));
      var item = el('div', { class: 'job' }, head);
      if (j.state === 'running') {
        add(item, el('div', { class: 'small muted', text: j.processed + ' archivos procesados · desde ' + ago(j.startedAt) }));
        if (j.current) add(item, el('div', { class: 'cur mono', text: j.current, title: j.current }));
      }
      if (j.summary && j.summary.length) add(item, el('ul', null, j.summary.map(function (s) { return el('li', { text: s }); })));
      if (j.error) add(item, el('p', { class: 'err', text: j.error }));
      add(box, item);
    });
  }

  // ---------- Fuentes ----------

  function viewSources() {
    api('GET', 'sources').then(function (list) {
      var addBtn = el('button', { class: 'btn primary', type: 'button', text: '＋ Añadir carpeta', onclick: function () { addForm.classList.toggle('hidden'); } });
      var scanAll = el('button', { class: 'btn', type: 'button', text: '⟳ Escanear todo', disabled: !list.length, onclick: function () {
        api('POST', 'scan').then(function () { toast('Escaneo de todas las fuentes en marcha'); }).catch(fail);
      } });
      var addForm = sourceForm(null, function (body) {
        return api('POST', 'sources', body).then(function (r) {
          toast('Fuente «' + r.source.label + '» añadida' + (r.job ? ': escaneando…' : ''));
          if (r.warning) toast(r.warning, true);
          viewSources();
        });
      });
      addForm.classList.add('hidden');
      if (!list.length) addForm.classList.remove('hidden');
      setMain(
        el('div', { class: 'head' },
          el('div', null, el('h1', { text: 'Fuentes' }), el('p', { text: 'Carpetas de la biblioteca. Quitar una fuente no borra nada del disco: solo la saca del catálogo.' })),
          el('div', { class: 'actions' }, addBtn, scanAll)),
        addForm,
        list.length ? list.map(sourceCard) : el('p', { class: 'empty', text: 'Todavía no hay fuentes. Añade la primera carpeta.' }));
    }).catch(fail);
  }

  function sourceCard(s) {
    var acts = el('div', { class: 'acts' },
      el('button', { class: 'btn small primary', type: 'button', text: 'Escanear', disabled: s.state === 'paused', onclick: function () {
        api('POST', 'sources/' + encodeURIComponent(s.id) + '/scan').then(function () { toast('Escaneando «' + s.label + '»…'); }).catch(fail);
      } }),
      el('button', { class: 'btn small', type: 'button', text: 'Editar', onclick: function () { editSource(s); } }),
      s.state === 'paused'
        ? el('button', { class: 'btn small', type: 'button', text: 'Reanudar', onclick: function () { setState(s, 'resume'); } })
        : el('button', { class: 'btn small', type: 'button', text: 'Pausar', onclick: function () { setState(s, 'pause'); } }),
      el('button', { class: 'btn small danger', type: 'button', text: 'Quitar', onclick: function () { removeSource(s); } }));
    return el('div', { class: 'card source' },
      el('div', null,
        el('h3', null, s.label, badge(s.state, STATES[s.state] || s.state)),
        el('div', { class: 'mono muted', text: s.path }),
        el('div', { class: 'meta' },
          el('span', { text: KINDS[s.kind] || s.kind }),
          el('span', { text: s.files + ' archivos' }),
          el('span', { text: 'Último escaneo: ' + ago(s.lastScanAt), title: fmtDate(s.lastScanAt) }),
          s.excludes && s.excludes.length ? el('span', { text: 'Excluye: ' + s.excludes.join(', ') }) : null)),
      acts);
  }

  function setState(s, what) {
    api('POST', 'sources/' + encodeURIComponent(s.id) + '/' + what).then(function () {
      toast('«' + s.label + '» ' + (what === 'pause' ? 'en pausa: no se escaneará' : 'reanudada'));
      viewSources();
    }).catch(fail);
  }

  function removeSource(s) {
    confirmBox('Quitar «' + s.label + '»',
      'Se saca del catálogo con sus ' + s.files + ' archivos (y el progreso visto de esos archivos). No se borra NADA del disco: ' + s.path + '. Puedes volver a añadirla cuando quieras.',
      'Quitar del catálogo', true).then(function (ok) {
      if (!ok) return;
      api('DELETE', 'sources/' + encodeURIComponent(s.id)).then(function () { toast('«' + s.label + '» quitada del catálogo'); viewSources(); }).catch(fail);
    });
  }

  // sourceForm: alta (s = null) o edición. submit(body) devuelve una promesa.
  function sourceForm(s, submit) {
    var path = el('input', { type: 'text', placeholder: 'C:\\YourFlix\\Nueva', value: s ? s.path : '', readonly: !!s, spellcheck: 'false' });
    var label = el('input', { type: 'text', placeholder: 'p. ej. Samsung T7 (se ve en la TV)', value: s ? s.label : '' });
    var kind = el('select', null, Object.keys(KINDS).map(function (k) { return el('option', { value: k, text: KINDS[k], selected: s && s.kind === k }); }));
    var ex = el('textarea', { placeholder: '*sample*\nExtras/', spellcheck: 'false' });
    ex.value = s && s.excludes ? s.excludes.join('\n') : '';
    var scanNow = el('input', { type: 'checkbox', checked: true });
    var err = el('p', { class: 'err hidden' });
    var btn = el('button', { class: 'btn primary', type: 'submit', text: s ? 'Guardar' : 'Añadir fuente' });
    var form = el('form', { class: 'card gold', onsubmit: function (e) {
      e.preventDefault();
      var body = { label: label.value, kind: kind.value, excludes: ex.value.split(/\n|,/).map(function (x) { return x.trim(); }).filter(Boolean) };
      if (!s) { body.path = path.value.trim(); body.scan = scanNow.checked; }
      btn.disabled = true;
      submit(body).catch(function (x) { err.textContent = x.message; err.className = 'err'; }).then(function () { btn.disabled = false; });
    } },
      s ? null : el('h3', { text: 'Añadir carpeta' }),
      el('div', { class: 'row' },
        el('label', null, 'Carpeta', path),
        s ? null : el('button', { class: 'btn', type: 'button', text: '📁 Explorar…', onclick: function () {
          browseDialog(path.value.trim(), function (p, name) { path.value = p; if (!label.value) label.value = name; });
        } })),
      el('div', { class: 'row' }, el('label', null, 'Etiqueta', label), el('label', { class: 'narrow' }, 'Tipo', kind)),
      el('label', null, 'Exclusiones (una por línea; *patrón* para nombres, Carpeta/ para carpetas)', ex),
      s ? el('p', { class: 'small muted', text: 'El tipo y las exclusiones se aplican en el próximo escaneo. Un cambio de tipo solo reinterpreta los archivos nuevos, cambiados o pendientes.' })
        : el('label', { class: 'check' }, scanNow, 'Escanear al añadir'),
      err, btn);
    return form;
  }

  function editSource(s) {
    var form = sourceForm(s, function (body) {
      return api('PATCH', 'sources/' + encodeURIComponent(s.id), body).then(function () { closeModal(); toast('Fuente guardada'); viewSources(); });
    });
    form.className = '';
    modal('Editar «' + s.label + '»', form, [{ text: 'Cancelar' }]);
  }

  // Explorador de carpetas servido por el propio servidor.
  function browseDialog(start, pick) {
    var body = el('div');
    var cur = { path: '' };
    function load(p) {
      clear(body);
      add(body, el('p', { class: 'loading', text: 'Abriendo…' }));
      api('GET', 'browse?path=' + encodeURIComponent(p || '')).then(function (d) {
        cur = d;
        clear(body);
        add(body, el('div', { class: 'chips' }, d.roots.map(function (r) {
          return el('button', { class: 'chip' + (d.path && d.path.indexOf(r.path) === 0 ? ' on' : ''), type: 'button', text: r.name, onclick: function () { load(r.path); } });
        })));
        if (d.path) {
          var crumbs = el('div', { class: 'crumbs' });
          add(crumbs, el('button', { type: 'button', text: '⌂', title: 'Unidades', onclick: function () { load(''); } }));
          var parts = d.path.split(d.sep).filter(Boolean), acc = '';
          parts.forEach(function (part, i) {
            acc = i === 0 ? (d.sep === '/' ? '/' + part : part + d.sep) : (acc.slice(-1) === d.sep ? acc : acc + d.sep) + part;
            var target = acc;
            add(crumbs, [el('span', { class: 'muted', text: '›' }), el('button', { type: 'button', text: part, onclick: function () { load(target); } })]);
          });
          add(body, crumbs);
          add(body, el('div', { class: 'mono', text: d.path }));
          add(body, el('p', { class: 'small muted', text: d.videos + ' vídeos directamente en esta carpeta · ' + d.dirs.length + ' subcarpetas' + (d.truncated ? ' (lista recortada)' : '') }));
        } else {
          add(body, el('p', { class: 'muted', text: 'Elige una unidad o un punto de partida.' }));
        }
        var ul = el('ul', { class: 'dirlist' });
        if (d.path) add(ul, el('li', null, el('button', { type: 'button', text: '.. (subir)', onclick: function () { load(d.parent); } })));
        d.dirs.forEach(function (x) { add(ul, el('li', null, el('button', { type: 'button', text: x.name, onclick: function () { load(x.path); } }))); });
        if (!d.dirs.length && !d.path) add(ul, el('li', { class: 'empty', text: 'Sin unidades visibles' }));
        add(body, ul);
      }).catch(function (e) {
        clear(body);
        add(body, el('p', { class: 'err', text: e.message }), el('button', { class: 'btn', type: 'button', text: 'Volver a las unidades', onclick: function () { load(''); } }));
      });
    }
    modal('Elegir carpeta del servidor', body, [
      { text: 'Cancelar' },
      { text: 'Elegir esta carpeta', cls: 'primary', click: function () {
        if (!cur.path) { toast('Entra primero en una carpeta', true); return false; }
        var name = cur.path.split(cur.sep).filter(Boolean).pop() || cur.path;
        pick(cur.path, name);
      } }
    ]);
    load(start);
  }

  // ---------- Catálogo ----------

  function viewCatalog() {
    Promise.all([api('GET', 'pending')]).then(function (r) {
      var d = r[0];
      var retry = el('input', { type: 'checkbox' });
      var idBtn = el('button', { class: 'btn primary', type: 'button', text: 'Identificar ahora', disabled: !d.tmdb, onclick: function () {
        api('POST', 'identify', { retry: retry.checked }).then(function () { toast('Identificación con TMDB en marcha'); }).catch(fail);
      } });
      var findQ = el('input', { type: 'search', placeholder: 'Título, director o actor' });
      var found = el('div');
      var findForm = el('form', { class: 'row', onsubmit: function (e) {
        e.preventDefault();
        clear(found);
        api('GET', 'titles?q=' + encodeURIComponent(findQ.value)).then(function (t) {
          if (!t.items.length) { add(found, el('p', { class: 'empty', text: 'Nada con ese texto.' })); return; }
          add(found, t.items.map(titleCard));
        }).catch(fail);
      } }, el('label', null, 'Buscar en el catálogo', findQ), el('button', { class: 'btn', type: 'submit', text: 'Buscar' }));
      setMain(
        el('div', { class: 'head' },
          el('div', null, el('h1', { text: 'Catálogo' }),
            el('p', { text: d.items.length ? d.items.length + ' fichas pendientes: TMDB no las identificó con seguridad. Elige la buena con un clic (queda bloqueada, como «yourflix match»).' : 'No hay fichas pendientes.' })),
          el('div', { class: 'actions' }, idBtn, el('label', { class: 'check small' }, retry, 'reintentar también las ya probadas'))),
        d.tmdb ? null : el('p', { class: 'card err', text: 'TMDB no está configurado en el servidor (--tmdb-token-file o TMDB_ACCESS_TOKEN): no se puede buscar ni emparejar.' }),
        el('div', { class: 'pending' }, d.items.map(function (p) { return pendingCard(p, d.tmdb); })),
        el('h2', { text: 'Corregir una ficha' }),
        el('p', { class: 'muted', text: 'Si una película quedó con la ficha equivocada, búscala y asígnale la correcta de TMDB.' }),
        findForm, found);
      var c = document.getElementById('pendingCount');
      c.textContent = d.items.length;
      c.className = d.items.length ? 'count' : 'count hidden';
    }).catch(fail);
  }

  function filesList(files) {
    return el('ul', { class: 'files' }, files.map(function (f) {
      return el('li', null, el('b', { text: f.source + ': ' }), el('span', { class: 'mono', text: epLabel(f) + f.path }), ' · ' + fmtBytes(f.size));
    }));
  }

  function tmdbLink(kind, id) {
    return el('a', { href: 'https://www.themoviedb.org/' + (kind === 'series' ? 'tv' : 'movie') + '/' + id, target: '_blank', rel: 'noopener noreferrer', text: 'TMDB ↗' });
  }

  function assign(titleId, kind, cand, after) {
    var name = cand.title + (cand.year ? ' (' + cand.year + ')' : '');
    confirmBox('Asignar ficha', 'Esta ficha pasará a ser «' + name + '» (TMDB ' + cand.tmdb + ') y quedará bloqueada: ni la semilla ni TMDB la cambiarán.', 'Asignar', false).then(function (ok) {
      if (!ok) return;
      api('POST', 'titles/' + encodeURIComponent(titleId) + '/match', { tmdb: cand.tmdb, kind: kind }).then(function (r) {
        toast(r.merged ? 'Unida a la ficha que ya existía: ' + r.title : 'Ficha asignada: ' + r.title + (r.year ? ' (' + r.year + ')' : ''));
        refreshPendingCount();
        if (after) after(); else viewCatalog();
      }).catch(fail);
    });
  }

  function pendingCard(p, tmdbOn) {
    var cands = p.candidates.length
      ? el('div', { class: 'tablewrap' }, el('table', null,
          el('thead', null, el('tr', null, ['Candidato', 'Año', 'Parecido', 'Votos', ''].map(function (h) { return el('th', { text: h }); }))),
          el('tbody', null, p.candidates.map(function (c) {
            return el('tr', null,
              el('td', null, el('b', { text: c.title }), c.original && c.original !== c.title ? el('div', { class: 'small muted', text: c.original }) : null),
              el('td', { text: c.year || '—' }),
              el('td', { text: (c.score || 0).toFixed(2) }),
              el('td', { text: c.votes }),
              el('td', null, el('button', { class: 'btn small primary', type: 'button', text: 'Elegir', disabled: !tmdbOn, onclick: function () { assign(p.id, p.kind, c); } }), ' ', tmdbLink(p.kind, c.tmdb)));
          }))))
      : el('p', { class: 'small muted', text: p.identifyTried ? 'TMDB no devolvió candidatos. Prueba a buscar a mano.' : 'Aún sin buscar en TMDB: pulsa «Identificar ahora» o busca a mano.' });
    return el('div', { class: 'card' },
      el('h3', null, p.title + (p.year ? ' (' + p.year + ')' : ''), ' ', badge('warn', p.kind === 'series' ? 'serie' : 'película')),
      el('div', { class: 'small muted', text: 'Deducido del nombre del archivo' + (p.identifyTried ? ' · último intento con TMDB ' + ago(p.identifyTried) : '') }),
      filesList(p.files), cands,
      tmdbOn ? searchBox(p.id, p.kind, p.title, p.year) : null);
  }

  function titleCard(t) {
    return el('div', { class: 'card' },
      el('div', { class: 'row' },
        t.poster ? el('img', { class: 'poster', src: t.poster, alt: '', loading: 'lazy' }) : null,
        el('div', { class: 'grow' },
          el('h3', null, t.title + (t.year ? ' (' + t.year + ')' : ''), ' ', badge(t.locked ? 'gold' : '', t.locked ? 'bloqueada' : ({ seed: 'semilla', tmdb: 'TMDB', manual: 'manual', filename: 'pendiente' }[t.matchSource] || t.matchSource))),
          t.tmdb ? el('div', { class: 'small' }, tmdbLink(t.kind, t.tmdb)) : null,
          filesList(t.files))),
      searchBox(t.id, t.kind, t.title, t.year, viewCatalog));
  }

  // searchBox: búsqueda manual en TMDB (título + año) para asignar a la ficha titleId.
  function searchBox(titleId, kind, title, year, after) {
    var q = el('input', { type: 'text', value: title || '' });
    var y = el('input', { type: 'text', inputmode: 'numeric', maxlength: '4', value: year || '', placeholder: 'año' });
    var k = el('select', null, el('option', { value: 'movie', text: 'Película', selected: kind !== 'series' }), el('option', { value: 'series', text: 'Serie', selected: kind === 'series' }));
    var out = el('div', { class: 'results' });
    var box = el('details', null, el('summary', { class: 'small', text: 'Buscar a mano en TMDB' }),
      el('form', { class: 'row search', onsubmit: function (e) {
        e.preventDefault();
        clear(out);
        add(out, el('p', { class: 'muted', text: 'Buscando…' }));
        api('GET', 'tmdb/search?q=' + encodeURIComponent(q.value) + '&year=' + encodeURIComponent(y.value.trim()) + '&kind=' + k.value).then(function (r) {
          clear(out);
          if (!r.items.length) { add(out, el('p', { class: 'muted', text: 'TMDB no encuentra nada con esos datos.' })); return; }
          r.items.forEach(function (c) {
            add(out, el('div', { class: 'result' },
              c.poster ? el('img', { src: c.poster, alt: '', loading: 'lazy', referrerpolicy: 'no-referrer' }) : el('div', { class: 'ph' }),
              el('div', { class: 'info' },
                el('div', { class: 't', text: c.title + (c.year ? ' (' + c.year + ')' : '') }),
                c.original && c.original !== c.title ? el('div', { class: 'small muted', text: c.original }) : null,
                el('div', { class: 'small muted', text: c.votes + ' votos · ' }, tmdbLink(r.kind, c.tmdb)),
                el('button', { class: 'btn small primary', type: 'button', text: 'Asignar', onclick: function () { assign(titleId, r.kind, c, after); } }))));
          });
        }).catch(function (x) { clear(out); add(out, el('p', { class: 'err', text: x.message })); });
      } }, el('label', null, 'Título', q), el('label', { class: 'narrow' }, 'Año', y), el('label', { class: 'narrow' }, 'Tipo', k), el('button', { class: 'btn', type: 'submit', text: 'Buscar' })),
      out);
    return box;
  }

  // ---------- Compatibilidad ----------

  function viewCompat() {
    api('GET', 'compat').then(function (d) {
      var filter = { status: '', reason: '', q: '' };
      var list = el('div');
      function draw() {
        clear(list);
        var items = d.items.filter(function (it) {
          if (filter.status && it.status !== filter.status) return false;
          if (filter.reason && !it.reasons.some(function (r) { return reasonKind(r) === filter.reason; })) return false;
          if (filter.q && (it.title + ' ' + it.path + ' ' + it.source).toLowerCase().indexOf(filter.q) < 0) return false;
          return true;
        });
        if (!items.length) { add(list, el('p', { class: 'empty', text: 'Nada que mostrar con este filtro.' })); return; }
        add(list, el('div', { class: 'card tablewrap' }, el('table', null,
          el('thead', null, el('tr', null, ['Título', 'Archivo', 'Estado', 'Motivos'].map(function (h) { return el('th', { text: h }); }))),
          el('tbody', null, items.map(function (it) {
            return el('tr', null,
              el('td', null, el('b', { text: it.title }), it.year ? ' (' + it.year + ')' : '', it.season != null ? el('div', { class: 'small muted', text: epLabel(it) }) : null),
              el('td', null, el('div', { class: 'small', text: it.source }), el('div', { class: 'mono muted', text: it.path })),
              el('td', null, badge(it.status, COMPAT[it.status])),
              el('td', null, el('ul', { class: 'reasons small' }, it.reasons.map(function (r) { return el('li', { text: r }); }))));
          })))));
      }
      var chips = el('div', { class: 'chips' });
      function drawChips() {
        clear(chips);
        add(chips, el('button', { class: 'chip' + (!filter.reason ? ' on' : ''), type: 'button', text: 'Todos los motivos', onclick: function () { filter.reason = ''; drawChips(); draw(); } }));
        d.reasons.forEach(function (r) {
          add(chips, el('button', { class: 'chip' + (filter.reason === r.reason ? ' on' : ''), type: 'button', text: r.reason + ' · ' + r.count, onclick: function () { filter.reason = r.reason; drawChips(); draw(); } }));
        });
      }
      var st = el('select', { onchange: function () { filter.status = st.value; draw(); } },
        el('option', { value: '', text: 'Avisos y optimizar' }), el('option', { value: 'optimize', text: 'Solo optimizar' }), el('option', { value: 'warn', text: 'Solo con avisos' }));
      var q = el('input', { type: 'search', placeholder: 'Filtrar por título o archivo', oninput: function () { filter.q = q.value.toLowerCase(); draw(); } });
      setMain(
        el('div', { class: 'head' }, el('div', null, el('h1', { text: 'Compatibilidad' }),
          el('p', { text: 'Lo que el perfil de la TV (' + d.profile + ') marca como problemático, de ' + d.total + ' archivos disponibles. «Optimizar» = la TV no lo reproduce bien tal cual (p. ej. solo DTS: sin sonido).' }))),
        el('div', { class: 'grid' }, ['direct', 'warn', 'optimize'].map(function (k) {
          return el('div', { class: 'card stat ' + k }, el('div', { class: 'n', text: d.counts[k] || 0 }), el('div', { class: 'muted', text: COMPAT[k] }));
        })),
        chips, el('div', { class: 'row filters' }, el('label', null, 'Estado', st), el('label', null, 'Buscar', q)),
        list);
      drawChips();
      draw();
    }).catch(fail);
  }

  // Igual que reasonKind en el servidor (internal/admin/status.go).
  function reasonKind(r) {
    if (r.indexOf('subtítulos en imagen') >= 0) return 'subtítulos en imagen (PGS/DVD)';
    if (r.indexOf('Mb/s') >= 0) return 'caudal alto (Wi-Fi)';
    if (r.indexOf('resolución') === 0) return 'resolución mayor que la TV';
    if (r.indexOf('pistas de audio no disponibles') === 0) return 'alguna pista de audio no suena';
    if (r.indexOf('audio ') === 0 && /sin sonido en la TV$/.test(r)) return 'sin audio compatible (sin sonido)';
    return r;
  }

  // ---------- Estado ----------

  function viewStatus() {
    api('GET', 'status').then(function (d) {
      function fact(k, v, cls) { return el('div', { class: 'card fact' }, el('div', { class: 'k', text: k }), el('div', { class: 'v ' + (cls || '') }, v)); }
      var facts = el('div', { class: 'grid' },
        fact('Versión', 'YourFlix ' + d.version),
        fact('Encendido', fmtUptime(d.uptimeSeconds), null),
        fact('Carpeta de datos', el('span', { class: 'mono', text: d.dataDir })),
        fact('Sistema', d.os + ' · ' + d.go),
        fact('Catálogo', d.titles + ' títulos · ' + d.pending + ' pendientes'),
        fact('TMDB', d.tmdb ? 'configurado' : 'sin credenciales', d.tmdb ? 'ok' : 'bad'),
        d.tools.map(function (t) {
          return fact(t.name, t.found ? el('span', null, '✓ ', el('span', { class: 'small', text: t.version }), el('div', { class: 'mono muted', text: t.path })) : '✗ ' + (t.error || 'no encontrado') + (t.path ? ' (' + t.path + ')' : ''), t.found ? 'ok' : 'bad');
        }));
      var disks = el('div', { class: 'grid' }, d.disks.map(function (k) {
        if (k.error) return el('div', { class: 'card' }, el('h3', { text: k.usedBy.join(', ') }), el('div', { class: 'mono muted', text: k.path }), el('p', { class: 'err', text: k.error }));
        var used = k.total ? (k.total - k.free) / k.total : 0;
        return el('div', { class: 'card' },
          el('h3', { class: 'mono', text: k.path }),
          bar(used, k.free / k.total < 0.1),
          el('div', null, el('b', { text: fmtBytes(k.free) + ' libres' }), el('span', { class: 'muted', text: ' de ' + fmtBytes(k.total) })),
          el('div', { class: 'small muted', text: k.usedBy.join(' · ') }));
      }));
      var srcTable = el('div', { class: 'card tablewrap' }, el('table', null,
        el('thead', null, el('tr', null, ['Fuente', 'Estado', 'Archivos', 'Último escaneo'].map(function (h) { return el('th', { text: h }); }))),
        el('tbody', null, d.sources.map(function (s) {
          return el('tr', null, el('td', { text: s.label }), el('td', null, badge(s.state, STATES[s.state] || s.state)), el('td', { text: s.files }), el('td', { text: fmtDate(s.lastScanAt) + ' (' + ago(s.lastScanAt) + ')' }));
        }))));
      var scans = d.scans.length ? el('div', { class: 'card tablewrap' }, el('table', null,
        el('thead', null, el('tr', null, ['Cuándo', 'Fuente', 'Resultado', 'Duración'].map(function (h) { return el('th', { text: h }); }))),
        el('tbody', null, d.scans.map(function (r) {
          var x = r.report || {};
          var res = r.error ? el('span', { class: 'err', text: r.error })
            : x.offline ? 'desconectada: ' + x.missing + ' ausentes'
            : x.seen + ' vídeos · ' + x.new + ' nuevos · ' + x.changed + ' cambiados · ' + x.moved + ' movidos · ' + x.missing + ' ausentes · ' + x.probeErrors + ' errores de ffprobe';
          return el('tr', null, el('td', { text: fmtDate(r.startedAt) }), el('td', { text: r.label }), el('td', null, res), el('td', { text: r.seconds.toFixed(1) + ' s' }));
        })))) : el('p', { class: 'muted', text: 'Aún no se ha escaneado desde la administración. Los re-escaneos periódicos de serve se ven en «Último escaneo» de cada fuente y en el registro.' });
      var logBox = el('pre', { class: 'log', text: d.log.join('\n') || '(registro vacío)' });
      var more = el('button', { class: 'btn small', type: 'button', text: 'Ver 300 líneas', onclick: function () {
        api('GET', 'log?n=300').then(function (r) { logBox.textContent = r.lines.join('\n'); logBox.scrollTop = logBox.scrollHeight; }).catch(fail);
      } });
      setMain(
        el('div', { class: 'head' }, el('div', null, el('h1', { text: 'Estado del servidor' }), el('p', { text: 'Encendido desde ' + fmtDate(d.startedAt) })),
          el('div', { class: 'actions' }, el('button', { class: 'btn', type: 'button', text: '⟳ Actualizar', onclick: viewStatus }))),
        facts,
        el('h2', { text: 'Discos' }), disks,
        el('h2', { text: 'Fuentes' }), srcTable,
        el('h2', { text: 'Últimos escaneos desde la administración' }), scans,
        el('h2', null, 'Registro ', more), logBox,
        el('h2', { text: 'Contraseña de administración' }), passwordForm(d.passwordSet));
      logBox.scrollTop = logBox.scrollHeight;
    }).catch(fail);
  }

  function passwordForm(isSet) {
    var local = S.session && S.session.local;
    var cur = el('input', { type: 'password', autocomplete: 'current-password' });
    var p1 = el('input', { type: 'password', autocomplete: 'new-password', minlength: '8' });
    var p2 = el('input', { type: 'password', autocomplete: 'new-password' });
    var err = el('p', { class: 'err hidden' });
    return el('form', { class: 'card', onsubmit: function (e) {
      e.preventDefault();
      if (p1.value !== p2.value) { err.textContent = 'Las dos contraseñas no coinciden'; err.className = 'err'; return; }
      api('POST', 'password', { current: cur.value, password: p1.value }).then(function () {
        cur.value = p1.value = p2.value = '';
        err.className = 'err hidden';
        toast('Contraseña guardada. Las demás sesiones se han cerrado.');
        loadSession();
      }).catch(function (x) { err.textContent = x.message; err.className = 'err'; });
    } },
      el('p', { class: 'muted', text: isSet ? 'Cambiarla cierra las sesiones abiertas en otros equipos.' : 'Aún no hay contraseña: sin ella solo se entra desde este equipo. Defínela para administrar desde el Mac o el móvil.' }),
      el('div', { class: 'row' },
        isSet && !local ? el('label', null, 'Contraseña actual', cur) : null,
        el('label', null, 'Nueva (mínimo 8 caracteres)', p1),
        el('label', null, 'Repítela', p2)),
      err,
      el('button', { class: 'btn primary', type: 'submit', text: isSet ? 'Cambiar contraseña' : 'Definir contraseña' }));
  }

  loadSession();
})();
