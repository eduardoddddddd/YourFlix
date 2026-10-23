#!/usr/bin/env python3
"""Resume private/spike-log.jsonl en tablas Markdown para docs/PRUEBAS-TV.md.

Uso: python3 tools/spike-media/summarize.py [registro] [--from LINEA]
Toma el último resultado de cada archivo (y modo 4K) y las peticiones HTTP que
AVPlay hizo para ese archivo durante la prueba.
"""
import json
import sys
from collections import defaultdict

args = sys.argv[1:]
start = 0
if '--from' in args:
    i = args.index('--from')
    start = int(args[i + 1])
    del args[i:i + 2]
path = args[0] if args else 'private/spike-log.jsonl'

events = []
with open(path, encoding='utf-8') as f:
    for n, line in enumerate(f, 1):
        if n >= start and line.strip():
            events.append(json.loads(line))

results = {}
http = defaultdict(list)
errors = defaultdict(list)
subs = defaultdict(list)
current = None
for e in events:
    if e.get('src') == 'http':
        http[e['file']].append(e)
        continue
    if e.get('kind') == 'begin':
        current = e.get('file')
    if e.get('kind') == 'error':
        errors[e.get('file')].append(e['msg'])
    if e.get('msg', '').startswith('subtítulo ('):
        subs[e.get('file')].append(e['msg'])
    if e.get('kind') == 'result':
        r = e['result']
        results[(r['file'], r.get('mode4k', False))] = r

def cell(steps, name):
    for s in steps:
        if s['step'] == name:
            d = s.get('detail')
            if s['ok']:
                if isinstance(d, dict) and 'reanuda' in d:
                    return f"✓ {d['reanuda']/1000:.1f} s"
                if isinstance(d, dict) and 'cortes' in d:
                    return f"✓ {d['cortes']} cortes"
                return f"✓ {s['ms']/1000:.1f} s"
            return '✗ ' + str(d)[:60]
    return '—'

cols = ['prepare', 'arranque', 'continuo 20 s', 'salto 50 %', '+30 s', '−30 s', 'salto 90 %', 'volver al 10 %', 'cambio de audio', 'subtítulos (onsubtitlechange)']
print('| Archivo | 4K | ' + ' | '.join(cols) + ' |')
print('|' + '---|' * (len(cols) + 2))
for (name, m4k), r in sorted(results.items()):
    row = [cell(r['steps'], c) for c in cols]
    sub = [s for s in r['steps'] if s['step'].startswith('subtítulos')]
    if sub and sub[0]['ok']:
        row[-1] = f"✓ {sub[0]['detail'].get('recibidos')} recibidos"
    print(f"| {name} | {'sí' if m4k else 'no'} | " + ' | '.join(row) + ' |')

print('\n| Archivo | Peticiones | Con Range | Offset máximo pedido | Bytes enviados | Códigos |')
print('|---|---|---|---|---|---|')
for name, reqs in sorted(http.items()):
    ranged = [q for q in reqs if q.get('range')]
    offs = []
    for q in ranged:
        try:
            offs.append(int(q['range'].split('=')[1].split('-')[0] or 0))
        except ValueError:
            pass
    codes = sorted({q['status'] for q in reqs})
    sent = sum(q['sent'] for q in reqs)
    print(f"| {name} | {len(reqs)} | {len(ranged)} | {max(offs)/1e9 if offs else 0:.2f} GB | {sent/1e6:.1f} MB | {', '.join(map(str, codes))} |")

if errors:
    print('\nErrores de AVPlay:')
    for name, msgs in errors.items():
        print(f"- {name}: " + '; '.join(dict.fromkeys(msgs)))
if subs:
    print('\nPrimeros subtítulos recibidos:')
    for name, msgs in subs.items():
        print(f"- {name}: " + ' | '.join(msgs[:3]))
ua = next((q['ua'] for reqs in http.values() for q in reqs), '')
if ua:
    print(f'\nUser-Agent de AVPlay: `{ua}`')
