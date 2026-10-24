// Copia la base de datos de YourFlix a otra carpeta de datos y corrige las rutas absolutas
// de las carátulas de la semilla (`titles.poster` / `titles.backdrop`), que apuntan a la carpeta vieja.
// Las imágenes y los subtítulos cacheados se copian aparte (robocopy). Necesita Node 22.5+ (node:sqlite).
//
//   node tools/ally/migrar-datos.mjs <datos-viejos> <datos-nuevos>
//
// VACUUM INTO da una copia coherente aunque el servidor viejo siga abierto; para no perder
// progreso, lanzarlo con el servidor viejo ya parado.
import { DatabaseSync } from 'node:sqlite';
import fs from 'node:fs';
import path from 'node:path';

const [from, to] = process.argv.slice(2).map((p) => p && path.resolve(p));
if (!from || !to) { console.error('uso: node migrar-datos.mjs <datos-viejos> <datos-nuevos>'); process.exit(2); }
const src = path.join(from, 'yourflix.db'), dst = path.join(to, 'yourflix.db');
if (!fs.existsSync(src)) { console.error('no existe ' + src); process.exit(1); }
fs.mkdirSync(to, { recursive: true });
for (const f of [dst, dst + '-wal', dst + '-shm']) fs.rmSync(f, { force: true });

const old = new DatabaseSync(src, { readOnly: true });
old.prepare('VACUUM INTO ?').run(dst);
old.close();

const db = new DatabaseSync(dst);
const fix = db.prepare('UPDATE titles SET poster = replace(poster, ?, ?), backdrop = replace(backdrop, ?, ?)');
const r = fix.run(from + path.sep, to + path.sep, from + path.sep, to + path.sep);
const left = db.prepare('SELECT count(*) n FROM titles WHERE poster LIKE ? OR backdrop LIKE ?').get(from + '%', from + '%').n;
const counts = ['sources', 'titles', 'files', 'progress'].map((t) => t + ' ' + db.prepare(`SELECT count(*) n FROM ${t}`).get().n).join(' · ');
db.close();
console.log(`Copiada ${src} → ${dst}\nRutas de carátulas corregidas: ${r.changes} fichas (quedan ${left} con la ruta vieja)\n${counts}`);
