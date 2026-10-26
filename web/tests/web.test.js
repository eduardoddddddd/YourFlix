// node --test web/tests/*.test.js
const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');

const root = path.join(__dirname, '..');
const U = require('../js/util.js');
const P = require('../js/playback.js').playback;

test('formatos y rutas', () => {
  assert.equal(U.clock(6863), '1:54:23');
  assert.equal(U.clock(300), '5:00');
  assert.equal(U.clock(Infinity), '--:--');
  assert.equal(U.runtime(116), '1 h 56 min');
  assert.equal(U.bytes(4.2e9), '4,2 GB');
  assert.equal(U.epLabel({ season: 1, episode: 3 }), '1×03');
  const r = U.parseRoute('#/titulo/t7%2Fx?temporada=2&q=el+padrino');
  assert.deepEqual(r.parts, ['titulo', 't7/x']);
  assert.deepEqual(r.query, { temporada: '2', q: 'el padrino' });
  assert.deepEqual(U.parseRoute('').parts, []);
  assert.equal(U.route(['ver', 'a b', 'f1'], { t: 0, x: '' }), '#/ver/a%20b/f1?t=0');
  assert.deepEqual(U.parseRoute(U.route(['buscar'], { q: 'ñu & co' })).query, { q: 'ñu & co' });
  assert.equal(U.imgWidth('/api/v1/titles/x/poster?w=300', 600), '/api/v1/titles/x/poster?w=600');
});

test('cadenas de códec para preguntar al navegador', () => {
  assert.equal(P.videoCodec({ codec: 'h264', profile: 'High', width: 1920, height: 1080 }), 'avc1.640029');
  assert.equal(P.videoCodec({ codec: 'h264', profile: 'Main', width: 1280, height: 720 }), 'avc1.4d401f');
  assert.equal(P.videoCodec({ codec: 'h264', profile: 'High 10', bitDepth: 10 }), '', 'H.264 de 10 bits: ningún navegador');
  assert.equal(P.videoCodec({ codec: 'hevc', profile: 'Main 10', bitDepth: 10, width: 3840, height: 2160 }), 'hvc1.2.4.L153.B0');
  assert.equal(P.videoCodec({ codec: 'mpeg4' }), '');
  assert.equal(P.directType({ container: 'mkv', video: { codec: 'h264', profile: 'High', height: 1080 } }, { codec: 'ac3' }), 'video/x-matroska; codecs="avc1.640029, ac-3"');
  assert.equal(P.directType({ container: 'avi', video: { codec: 'h264', profile: 'High' } }, { codec: 'mp3' }), '', 'AVI nunca directo');
});

// Navegadores simulados. canPlay responde como canPlayType(...) === 'probably'.
const chrome = { hls: 'hlsjs', h264: true, hevc: false, hevc10: false, transcode: true,
  canPlay: (t) => /video\/(mp4|x-matroska)/.test(t) && /avc1/.test(t) && !/ac-3|ec-3/.test(t) };
const safari = { hls: 'native', h264: true, hevc: true, hevc10: true, transcode: true,
  canPlay: (t) => /video\/mp4/.test(t) && /avc1|hvc1/.test(t) };
const firefoxNoEncoder = Object.assign({}, chrome, { transcode: false });

const H264_AAC = { container: 'mkv', video: { codec: 'h264', profile: 'High', width: 1920, height: 1080, bitDepth: 8 },
  audio: [{ index: 1, codec: 'aac', channels: 2, language: 'spa', default: true }, { index: 2, codec: 'ac3', channels: 6, language: 'eng' }] };
const H264_AC3 = { container: 'mkv', video: { codec: 'h264', profile: 'High', width: 1920, height: 800 },
  audio: [{ index: 1, codec: 'ac3', channels: 6, language: 'spa', default: true }] };
const HEVC10 = { container: 'mkv', video: { codec: 'hevc', profile: 'Main 10', width: 3840, height: 2160, bitDepth: 10 },
  audio: [{ index: 1, codec: 'eac3', channels: 6, language: 'spa' }] };
const XVID = { container: 'avi', video: { codec: 'mpeg4', profile: 'Advanced Simple Profile', width: 640, height: 352 },
  audio: [{ index: 1, codec: 'mp3', channels: 2 }] };

test('decide: directo, remux o recodificación según navegador y archivo', () => {
  const d = (f, a, c) => { const r = P.decide(f, a, c); return r.method + (r.mode ? ':' + r.mode : ''); };
  assert.equal(d(H264_AAC, 1, chrome), 'direct', 'MKV H.264 + AAC en Chrome: tal cual');
  assert.equal(d(H264_AAC, 2, chrome), 'hls:copy', 'otra pista de audio: remux (el navegador no deja elegir pista)');
  assert.equal(d(H264_AC3, 1, chrome), 'hls:copy', 'AC3: vídeo copiado y audio a AAC');
  assert.equal(d(H264_AC3, 1, safari), 'hls:copy', 'Safari no lee MKV');
  assert.equal(d(HEVC10, 1, safari), 'hls:copy', 'HEVC 10 bits en Safari: copiado');
  assert.equal(d(HEVC10, 1, chrome), 'hls:h264', 'HEVC sin soporte: recodificado');
  assert.equal(d(XVID, 1, safari), 'hls:h264', 'XviD: recodificado');
  assert.equal(d({ container: 'mkv', video: { codec: 'h264', profile: 'High 10', bitDepth: 10 }, audio: [] }, -1, chrome), 'hls:h264', 'H.264 de 10 bits');
  const none = P.decide(HEVC10, 1, firefoxNoEncoder);
  assert.equal(none.method, 'none');
  assert.match(none.why, /HEVC de 10 bits/);
  assert.match(P.decide(XVID, 1, firefoxNoEncoder).why, /XviD/);
  assert.equal(d(H264_AC3, 1, Object.assign({}, chrome, { hls: '' })), 'none', 'sin HLS no hay remux');
  assert.match(P.decide(H264_AC3, 1, Object.assign({}, chrome, { hls: '', remuxOff: true })).why, /--remux-jobs 0/);
  assert.equal(d(H264_AAC, 1, Object.assign({}, chrome, { hls: '', remuxOff: true })), 'direct', 'sin remux, lo directo sigue funcionando');
  assert.match(P.decide(H264_AC3, 1, chrome).why, /AC3 → AAC/);
});

test('pistas de audio y subtítulos', () => {
  assert.equal(P.pickAudio(H264_AAC, ''), 1, 'la de por defecto');
  assert.equal(P.pickAudio(H264_AAC, 'EN'), 2, 'el último idioma elegido');
  assert.equal(P.pickAudio(HEVC10, 'EN'), 1, 'sin ese idioma: la primera');
  assert.equal(P.pickAudio({ audio: [] }, ''), -1);
  assert.equal(P.audioLabel({ index: 2, codec: 'eac3', channels: 6, language: 'eng' }), 'EN 5.1 · E-AC3');
  assert.equal(P.audioLabel({ index: 3, codec: 'aac', channels: 2, title: 'Comentarios' }), 'Pista 3 estéreo · AAC · Comentarios');
  const tracks = [{ id: 'e3', language: 'spa', forced: true }, { id: 'e4', language: 'spa' }, { id: 'x0', language: 'eng', default: true }];
  assert.equal(P.pickSubtitle(tracks, null, 'ES'), 'e3', 'forzados en el idioma del audio');
  assert.equal(P.pickSubtitle(tracks, 'off', 'ES'), 'e3', 'los forzados salen aunque estén desactivados');
  assert.equal(P.pickSubtitle(tracks, 'off', 'EN'), null);
  assert.equal(P.pickSubtitle(tracks, 'EN', 'ES'), 'x0', 'preferencia guardada');
});

test('progreso, siguiente capítulo y URL de HLS', () => {
  const eps = [{ id: 'a', season: 1, episode: 1, progress: { position: 1500, watched: true } }, { id: 'b', season: 1, episode: 2, progress: { position: 300 } }, { id: 'c', season: 1, episode: 3 }];
  assert.equal(P.resumeAt(eps[1]), 300);
  assert.equal(P.resumeAt({ progress: { position: 30 } }), 0, 'menos de un minuto: desde el principio');
  assert.equal(P.chooseFile(eps).id, 'b', 'el que está a medias');
  assert.equal(P.chooseFile([eps[0], eps[2]]).id, 'c', 'si no, el primero sin ver');
  assert.equal(P.nextFile(eps, eps[1]).id, 'c');
  assert.equal(P.nextFile(eps, eps[2]), null);
  assert.equal(P.nextFile([{ id: 'peli' }, { id: 'copia' }], { id: 'peli' }), null, 'las copias de una película no son capítulos');
  assert.equal(P.variant(2, 'copy'), 'a2-copy');
  assert.equal(P.variant(-1, 'h264'), 'an-h264');
  assert.equal(P.masterURL('f1', 'a1-copy', 'wabc', 125.7), '/api/v1/files/f1/hls/a1-copy/master.m3u8?s=wabc&t=125');
  assert.equal(P.masterURL('f1', 'a1-copy', 'wabc', 0), '/api/v1/files/f1/hls/a1-copy/master.m3u8?s=wabc');
  assert.match(P.session(), /^w[a-z0-9]{4,}$/);
  assert.ok(!P.shouldSave(1000, 10000, false) && P.shouldSave(1000, 16000, false) && P.shouldSave(1000, 1001, true));
});

test('hls.js sin empujones y recuperación donde va la imagen', () => {
  const c = P.hlsConfig(0);
  assert.equal(c.startPosition, -1);
  assert.equal(P.hlsConfig(2260).startPosition, 2260);
  assert.equal(c.nudgeMaxRetry, 0, 'los empujones de 0,1 s devuelven la imagen al principio (minibucle de 2 s)');
  const retry = c.fragLoadPolicy.default.errorRetry;
  assert.ok(retry.shouldRetry(retry, 0, false, { code: 503 }) && !retry.shouldRetry(retry, 0, false, { code: 501 }) && !retry.shouldRetry(retry, 3, false, { code: 500 }));
  // Atasco de arranque: currentTime parado en 0,6 mientras la imagen va por 2,4 s.
  assert.equal(P.recoverAt(0.6, 2.4, 0), 2.4);
  assert.equal(P.recoverAt(0, 1.7, 0), 1.7);
  assert.equal(P.recoverAt(0, 0, 2260), 2260, 'antes del primer fotograma: el punto de reanudar');
  assert.equal(P.recoverAt(2300, 0, 2260), 2300, 'a mitad de película: donde se iba, no el punto de inicio');
  assert.equal(P.recoverAt(2300, 2290, 0), 2300, 'un fotograma anterior no hace retroceder');
  assert.equal(P.recoverAt(100, 400, 0), 100, 'un fotograma muy lejano (de antes de un salto) no cuenta');
});

const jsFiles = fs.readdirSync(path.join(root, 'js')).map((f) => path.join(root, 'js', f));

test('seguridad y reglas del repo', () => {
  for (const f of jsFiles) {
    const src = fs.readFileSync(f, 'utf8');
    assert.ok(!/innerHTML|outerHTML|insertAdjacentHTML|document\.write|eval\(/.test(src), `${path.basename(f)}: datos siempre por textContent`);
    // Nada de CDN en tiempo de ejecución: la única URL es el espacio de nombres de SVG.
    const urls = (src.match(/https?:\/\/[^\s'"]+/g) || []).filter((u) => u !== 'http://www.w3.org/2000/svg');
    assert.deepEqual(urls, [], `${path.basename(f)}: sin URL externas`);
  }
  const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8');
  assert.ok(!/https?:\/\/(?!www\.w3\.org)/.test(html), 'index.html sin recursos externos');
  for (const ref of html.match(/(?:src|href)="([^"#][^"]*)"/g).map((m) => m.replace(/^(src|href)="|"$/g, ''))) {
    assert.ok(fs.existsSync(path.join(root, ref)), `index.html enlaza ${ref}, que no existe`);
  }
  for (const f of jsFiles.concat([path.join(root, 'index.html'), path.join(root, 'css', 'app.css')])) {
    assert.ok(!/Samsung T7|USB Patriot/.test(fs.readFileSync(f, 'utf8')), `${path.basename(f)}: no se muestra el soporte de origen`);
  }
});

test('hls.js copiado con su licencia y el hash anotado', () => {
  const lib = fs.readFileSync(path.join(root, 'vendor', 'hls.min.js'));
  const readme = fs.readFileSync(path.join(root, 'vendor', 'README.md'), 'utf8');
  const sha = crypto.createHash('sha256').update(lib).digest('hex');
  assert.ok(readme.includes(sha), 'el SHA-256 de vendor/README.md coincide con hls.min.js');
  assert.match(fs.readFileSync(path.join(root, 'vendor', 'hls.js-LICENSE.txt'), 'utf8'), /Apache License/);
});
