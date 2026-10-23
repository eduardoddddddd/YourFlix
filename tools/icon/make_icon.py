#!/usr/bin/env python3
"""Icono de YourFlix (tv/icon.png, 512×512) dibujado con Pillow a 4× y reducido.

Fondo azul noche con brillo cálido, botón de reproducción dorado con aro, y el
nombre en versalitas. Uso: python3 tools/icon/make_icon.py [salida.png] · --mark: solo el botón (tv/logo.png)
"""
import math
import sys

from PIL import Image, ImageChops, ImageDraw, ImageFilter, ImageFont

args = [a for a in sys.argv[1:] if not a.startswith('--')]
OUT = args[0] if args else 'tv/icon.png'
MARK = '--mark' in sys.argv  # solo el botón, sin nombre (logo del menú)
S = 2048  # lienzo de trabajo
GOLD_HI, GOLD_LO = (250, 222, 150), (196, 140, 52)


def lerp(a, b, t):
    return tuple(int(a[i] + (b[i] - a[i]) * t) for i in range(3))


def gradient(size, c1, c2, angle_deg):
    """Degradado lineal c1→c2 en la dirección dada."""
    w, h = size
    a = math.radians(angle_deg)
    dx, dy = math.cos(a), math.sin(a)
    small = Image.new('RGB', (256, 256))
    px = small.load()
    for y in range(256):
        for x in range(256):
            t = ((x / 255 - .5) * dx + (y / 255 - .5) * dy) + .5
            px[x, y] = lerp(c1, c2, min(1, max(0, t)))
    return small.resize(size, Image.BICUBIC)


def radial(size, center, radius, color, strength):
    """Capa de brillo radial (RGBA)."""
    w, h = size
    layer = Image.new('L', (256, 256))
    px = layer.load()
    cx, cy = center[0] / w * 255, center[1] / h * 255
    r = radius / w * 255
    for y in range(256):
        for x in range(256):
            d = math.hypot(x - cx, y - cy) / r
            px[x, y] = int(255 * strength * max(0, 1 - d) ** 2)
    mask = layer.resize(size, Image.BICUBIC)
    glow = Image.new('RGBA', size, color + (0,))
    glow.putalpha(mask)
    return glow


def rounded_triangle(cx, cy, r, rounding, rotate=0):
    """Puntos de un triángulo equilátero (apunta a la derecha) con esquinas redondeadas."""
    pts = []
    for k in range(3):
        a = math.radians(rotate + k * 120)
        pts.append((cx + r * math.cos(a), cy + r * math.sin(a)))
    out = []
    for i in range(3):
        p0, p1, p2 = pts[i - 1], pts[i], pts[(i + 1) % 3]
        # arco aproximado: varios puntos entre las dos aristas que llegan a p1
        v1 = (p0[0] - p1[0], p0[1] - p1[1])
        v2 = (p2[0] - p1[0], p2[1] - p1[1])
        l1, l2 = math.hypot(*v1), math.hypot(*v2)
        a1 = (p1[0] + v1[0] / l1 * rounding, p1[1] + v1[1] / l1 * rounding)
        a2 = (p1[0] + v2[0] / l2 * rounding, p1[1] + v2[1] / l2 * rounding)
        for t in [i / 12 for i in range(13)]:
            # Bézier cuadrática a1 → p1 → a2
            x = (1 - t) ** 2 * a1[0] + 2 * (1 - t) * t * p1[0] + t ** 2 * a2[0]
            y = (1 - t) ** 2 * a1[1] + 2 * (1 - t) * t * p1[1] + t ** 2 * a2[1]
            out.append((x, y))
    return out


def gold_fill(mask, angle=60):
    g = gradient(mask.size, GOLD_HI, GOLD_LO, angle).convert('RGBA')
    g.putalpha(mask)
    return g


img = gradient((S, S), (22, 27, 42), (5, 6, 10), 90).convert('RGBA')
img = Image.alpha_composite(img, radial((S, S), (S * .5, S * .39), S * .55, (229, 185, 92), .42))
img = Image.alpha_composite(img, radial((S, S), (S * .82, S * .08), S * .45, (90, 120, 200), .22))

cx, cy = S * .5, (S * .5 if MARK else S * .39)
ring_r, ring_w = S * .265, S * .026

# Sombra del botón
shadow = Image.new('L', (S, S), 0)
ImageDraw.Draw(shadow).ellipse((cx - ring_r, cy - ring_r + S * .03, cx + ring_r, cy + ring_r + S * .03), fill=150)
shadow = shadow.filter(ImageFilter.GaussianBlur(S * .03))
img = Image.alpha_composite(img, Image.merge('RGBA', (Image.new('L', (S, S), 0),) * 3 + (shadow,)))

# Disco interior oscuro, aro dorado y triángulo
disc = Image.new('L', (S, S), 0)
ImageDraw.Draw(disc).ellipse((cx - ring_r, cy - ring_r, cx + ring_r, cy + ring_r), fill=255)
inner = gradient((S, S), (34, 40, 58), (12, 14, 22), 100).convert('RGBA')
inner.putalpha(disc)
img = Image.alpha_composite(img, inner)

ring = Image.new('L', (S, S), 0)
d = ImageDraw.Draw(ring)
d.ellipse((cx - ring_r, cy - ring_r, cx + ring_r, cy + ring_r), fill=255)
d.ellipse((cx - ring_r + ring_w, cy - ring_r + ring_w, cx + ring_r - ring_w, cy + ring_r - ring_w), fill=0)
img = Image.alpha_composite(img, gold_fill(ring, 50))

tri = Image.new('L', (S, S), 0)
ImageDraw.Draw(tri).polygon(rounded_triangle(cx + S * .026, cy, S * .142, S * .04), fill=255)
img = Image.alpha_composite(img, gold_fill(tri, 65))

# Brillo superior del triángulo (luz suave)
shine = Image.new('L', (S, S), 0)
ImageDraw.Draw(shine).ellipse((cx - S * .25, cy - S * .42, cx + S * .25, cy - S * .02), fill=70)
shine = ImageChops.multiply(shine.filter(ImageFilter.GaussianBlur(S * .02)), tri)
img = Image.alpha_composite(img, Image.merge('RGBA', (Image.new('L', (S, S), 255),) * 3 + (shine,)))

if MARK:
    img.convert('RGB').resize((256, 256), Image.LANCZOS).save(OUT, optimize=True)
    print('logo:', OUT)
    sys.exit(0)

# Nombre: «YOUR» claro + «FLIX» dorado, versalitas espaciadas
font = ImageFont.truetype('/System/Library/Fonts/Avenir Next.ttc', int(S * .098), index=2)  # Demi Bold
spacing = S * .012
word = [('YOUR', (244, 241, 232)), ('FLIX', None)]
widths = [sum(font.getlength(ch) + spacing for ch in w) - spacing for w, _ in word]
gap = S * .006
x = (S - (sum(widths) + gap)) / 2
y = S * .845
text_gold = Image.new('L', (S, S), 0)
td = ImageDraw.Draw(text_gold)
dd = ImageDraw.Draw(img)
for (w, color), wd in zip(word, widths):
    for ch in w:
        if color:
            dd.text((x, y), ch, font=font, fill=color + (255,), anchor='ls')
        else:
            td.text((x, y), ch, font=font, fill=255, anchor='ls')
        x += font.getlength(ch) + spacing
    x += gap - spacing
img = Image.alpha_composite(img, gold_fill(text_gold, 70))

img.convert('RGB').resize((512, 512), Image.LANCZOS).save(OUT, optimize=True)
print('icono:', OUT)
