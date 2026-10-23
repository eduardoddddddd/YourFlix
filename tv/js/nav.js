/* Navegación espacial con el mando: los elementos con clase .f son enfocables; cada flecha
 * elige el más cercano en esa dirección por su posición en pantalla. El foco es la clase
 * .focused (no el foco del DOM) y cada elemento puede tener ._onFocus / ._onEnter. */
(function (YF) {
  'use strict';

  var nav = YF.nav = { current: null, roots: [] };

  function center(r) { return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; }

  // pick: puro, testeable. from y cands son rectángulos {left, top, width, height, right, bottom}.
  nav.pick = function (from, cands, dir) {
    var fc = center(from), best = -1, bestScore = Infinity;
    for (var i = 0; i < cands.length; i++) {
      var c = cands[i], cc = center(c), primary, secondary;
      switch (dir) {
        case 'right': if (cc.x <= fc.x + 1 || c.left < from.left + 1) continue; primary = Math.max(0, c.left - from.right); secondary = Math.abs(cc.y - fc.y) * 3; break;
        case 'left': if (cc.x >= fc.x - 1 || c.right > from.right - 1) continue; primary = Math.max(0, from.left - c.right); secondary = Math.abs(cc.y - fc.y) * 3; break;
        case 'down': if (cc.y <= fc.y + 1 || c.top < from.top + 1) continue; primary = Math.max(0, c.top - from.bottom); secondary = Math.abs(cc.x - fc.x) * 0.8; break;
        case 'up': if (cc.y >= fc.y - 1 || c.bottom > from.bottom - 1) continue; primary = Math.max(0, from.top - c.bottom); secondary = Math.abs(cc.x - fc.x) * 0.8; break;
        default: continue;
      }
      var score = primary + secondary;
      if (score < bestScore) { bestScore = score; best = i; }
    }
    return best;
  };

  function visible(e) { return e.offsetWidth > 0 && e.offsetHeight > 0; }

  // Raíces activas: el diálogo, si está abierto; si no, el menú y la pantalla visible.
  nav.setRoots = function (roots) { nav.roots = roots.filter(Boolean); };

  nav.candidates = function () {
    var out = [];
    nav.roots.forEach(function (r) {
      var list = r.querySelectorAll('.f');
      for (var i = 0; i < list.length; i++) if (visible(list[i])) out.push(list[i]);
    });
    return out;
  };

  nav.focus = function (e, noScroll) {
    if (!e) return;
    if (nav.current && nav.current !== e) nav.current.classList.remove('focused');
    nav.current = e;
    e.classList.add('focused');
    if (!noScroll) scrollIntoView(e);
    if (e._onFocus) e._onFocus(e);
  };

  // Desplaza el contenedor horizontal (fila) y el vertical (pantalla) lo justo para verlo.
  function scrollIntoView(e) {
    try { e.scrollIntoView({ block: 'nearest', inline: 'nearest' }); } catch (_) { e.scrollIntoView(false); }
    // scrollIntoView no deja margen bajo el menú: se corrige a mano en el desplazador vertical.
    var sc = e.closest ? e.closest('.scroller') : null;
    if (sc) {
      var r = e.getBoundingClientRect();
      if (r.top < 130) sc.scrollTop -= 130 - r.top;
      if (r.bottom > 1040) sc.scrollTop += r.bottom - 1040;
    }
  }

  nav.move = function (dir) {
    var cands = nav.candidates();
    if (!cands.length) return false;
    if (!nav.current || cands.indexOf(nav.current) < 0) { nav.focus(cands[0]); return true; }
    var rects = cands.map(function (c) { return c.getBoundingClientRect(); });
    var i = nav.pick(nav.current.getBoundingClientRect(), rects, dir);
    if (i < 0) return false;
    nav.focus(cands[i]);
    return true;
  };

  nav.enter = function () {
    if (nav.current && nav.current._onEnter) { nav.current._onEnter(nav.current); return true; }
    return false;
  };

  // f: hace enfocable un elemento con sus acciones.
  YF.f = function (e, onEnter, onFocus) {
    e.classList.add('f');
    if (onEnter) e._onEnter = onEnter;
    if (onFocus) e._onFocus = onFocus;
    return e;
  };
})(typeof module !== 'undefined' && module.exports ? module.exports : (window.YF = window.YF || {}));
