// Cliente mínimo del inspector remoto de la TV (Chrome DevTools Protocol).
// Requisitos: app lanzada con `sdb shell 0 debug <appId>` y `sdb forward tcp:9333 tcp:<puerto>`.
//   node tools/tv-monitor/cdp.mjs eval '<expresión JS>'   evalúa (espera promesas) e imprime el resultado
//   node tools/tv-monitor/cdp.mjs monitor [segundos]      consola + latido cada 2 s; avisa si la app no responde
// El latido detecta bloqueos del hilo JS; si además Debugger.pause no pausa, el bloqueo es nativo (AVPlay).
const [mode = 'monitor', arg] = process.argv.slice(2);
const base = process.env.CDP || 'http://127.0.0.1:9333';

const pages = await (await fetch(base + '/json')).json();
const ws = new WebSocket(pages[0].webSocketDebuggerUrl);
let seq = 0;
const waiting = new Map();
const send = (method, params = {}) => new Promise((resolve) => {
  const id = ++seq;
  waiting.set(id, resolve);
  ws.send(JSON.stringify({ id, method, params }));
});
const stamp = () => new Date().toTimeString().slice(0, 8);

ws.onmessage = (m) => {
  const msg = JSON.parse(m.data);
  if (msg.id && waiting.has(msg.id)) { waiting.get(msg.id)(msg); waiting.delete(msg.id); return; }
  if (mode !== 'monitor') return;
  if (msg.method === 'Runtime.consoleAPICalled') {
    console.log(stamp(), '[console]', msg.params.args.map((a) => a.value ?? a.description).join(' '));
  } else if (msg.method === 'Runtime.exceptionThrown') {
    console.log(stamp(), '[excepción]', msg.params.exceptionDetails.exception?.description || msg.params.exceptionDetails.text);
  }
};
await new Promise((r) => { ws.onopen = r; });
await send('Runtime.enable');
await send('Runtime.runIfWaitingForDebugger');

if (mode === 'eval') {
  const r = await send('Runtime.evaluate', { expression: arg, awaitPromise: true, returnByValue: true });
  console.log(JSON.stringify(r.result?.result?.value ?? r.result?.exceptionDetails ?? r.result, null, 2));
  process.exit(0);
}

const limit = Number(arg || 0) * 1000;
const started = Date.now();
let misses = 0;
setInterval(async () => {
  const t = Date.now();
  const r = await Promise.race([send('Runtime.evaluate', { expression: 'Date.now()', returnByValue: true }), new Promise((res) => setTimeout(() => res(null), 3000))]);
  if (!r) {
    misses++;
    console.log(stamp(), `[latido] SIN RESPUESTA (${misses})`);
    if (misses === 3) {
      const p = await Promise.race([send('Debugger.enable').then(() => send('Debugger.pause')), new Promise((res) => setTimeout(() => res(null), 3000))]);
      console.log(stamp(), p ? '[latido] Debugger.pause respondió: bloqueo en JS' : '[latido] Debugger.pause no responde: bloqueo nativo');
    }
  } else {
    if (misses) console.log(stamp(), `[latido] responde de nuevo (${Date.now() - t} ms)`);
    misses = 0;
  }
  if (limit && Date.now() - started > limit) process.exit(0);
}, 2000);
