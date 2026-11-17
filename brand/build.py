#!/usr/bin/env python3
"""Builds the LAN Party brand assets: app icon, tray icons, horizontal lockup, banner.

Text is converted to outlines with fontTools (Space Grotesk, OFL) so every SVG
is self-contained and needs no font installed.
"""
import os
import re
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.boundsPen import BoundsPen

HERE = os.path.dirname(os.path.abspath(__file__))
FONT = os.path.join(HERE, "fonts", "SpaceGrotesk.ttf")

# ---------------------------------------------------------------- palette
INK = "#08040F"
INK_2 = "#120A2E"
MAGENTA = "#FF3DBE"
VIOLET = "#8B5CF6"
CYAN = "#22D3EE"
PALE = "#FDF4FF"

PAD_PATH = ("M200 112H312C336 112 350 124 356 144C364 170 374 190 382 210"
            "C392 232 382 252 360 252C342 252 330 242 318 224Q296 194 256 194"
            "Q216 194 194 224C182 242 170 252 152 252C130 252 120 232 130 210"
            "C138 190 148 170 156 144C162 124 176 112 200 112Z")


# ---------------------------------------------------------------- type
class Face:
    def __init__(self, path, wght):
        font = instantiateVariableFont(TTFont(path), {"wght": wght}, inplace=False)
        self.cmap = font.getBestCmap()
        self.gs = font.getGlyphSet()
        self.upm = font["head"].unitsPerEm

    def run(self, text, size, tracking=0.0, kerning=None):
        scale = size / self.upm
        x = 0.0
        parts = []
        minx = miny = 1e9
        maxx = maxy = -1e9
        prev = None
        for ch in text:
            if kerning and prev is not None:
                x += kerning.get(prev + ch, 0.0) * size
            glyph = self.gs[self.cmap[ord(ch)]]
            pen = SVGPathPen(self.gs)
            glyph.draw(TransformPen(pen, (scale, 0, 0, -scale, x, 0)))
            if pen.getCommands():
                parts.append(pen.getCommands())
            bp = BoundsPen(self.gs)
            glyph.draw(TransformPen(bp, (scale, 0, 0, -scale, x, 0)))
            if bp.bounds:
                b = bp.bounds
                minx, miny = min(minx, b[0]), min(miny, b[1])
                maxx, maxy = max(maxx, b[2]), max(maxy, b[3])
            x += glyph.width * scale + tracking
            prev = ch
        return {
            "d": round_d(" ".join(parts)),
            "width": x,
            "x0": minx, "x1": maxx,
            "top": miny, "bottom": maxy,   # y-up: top is negative
        }


def round_d(d, nd=1):
    return re.sub(r"-?\d+\.\d+", lambda m: f"{float(m.group()):.{nd}f}".rstrip("0").rstrip("."), d)


def text_el(run, x, y, fill, extra=""):
    return (f'<path transform="translate({x:g} {y:g})" d="{run["d"]}" '
            f'fill="{fill}"{extra}/>')


# ---------------------------------------------------------------- shared defs
def base_defs(prefix=""):
    p = prefix
    return f'''<defs>
    <radialGradient id="{p}sky" cx="0.5" cy="0.26" r="0.85">
      <stop offset="0" stop-color="#2C1166"/>
      <stop offset="0.55" stop-color="#120A2E"/>
      <stop offset="1" stop-color="#07030F"/>
    </radialGradient>
    <linearGradient id="{p}pad" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#FF3DBE"/>
      <stop offset="0.5" stop-color="#8B5CF6"/>
      <stop offset="1" stop-color="#22D3EE"/>
    </linearGradient>
    <linearGradient id="{p}ring" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#67E8F9"/>
      <stop offset="0.5" stop-color="#C084FC"/>
      <stop offset="1" stop-color="#FB7BD2"/>
    </linearGradient>
    <linearGradient id="{p}ink" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="#FDF4FF"/>
      <stop offset="1" stop-color="#A5F3FC"/>
    </linearGradient>
    <filter id="{p}soft" x="-100%" y="-100%" width="300%" height="300%" color-interpolation-filters="sRGB">
      <feGaussianBlur stdDeviation="22"/>
    </filter>
    <filter id="{p}tight" x="-100%" y="-100%" width="300%" height="300%" color-interpolation-filters="sRGB">
      <feGaussianBlur stdDeviation="10"/>
    </filter>
    <clipPath id="{p}tile"><rect x="0" y="0" width="512" height="512" rx="116"/></clipPath>
    <path id="{p}padShape" d="{PAD_PATH}"/>
    <g id="{p}person">
      <circle cx="0" cy="-14" r="14"/>
      <path d="M-27 28C-27 10-15-1 0-1C15-1 27 10 27 28C27 31 24 33 21 33H-21C-24 33-27 31-27 28Z"/>
    </g>
  </defs>'''


# ---------------------------------------------------------------- icon art
STARS = """    <g fill="#EDE9FE">
      <circle cx="84" cy="118" r="2.2" opacity="0.7"/>
      <circle cx="430" cy="92" r="1.6" opacity="0.5"/>
      <circle cx="448" cy="230" r="2.2" opacity="0.6"/>
      <circle cx="62" cy="268" r="1.5" opacity="0.45"/>
      <circle cx="452" cy="392" r="1.8" opacity="0.45"/>
      <circle cx="58" cy="404" r="2" opacity="0.4"/>
      <circle cx="146" cy="54" r="1.4" opacity="0.4"/>
      <circle cx="372" cy="46" r="2" opacity="0.5"/>
    </g>"""


def icon_art(p="i", background=True):
    """Neon arcade mark, 512x512, optically centred."""
    if background:
        head = f'''
  <rect x="0" y="0" width="512" height="512" rx="116" fill="url(#{p}sky)"/>
  <g clip-path="url(#{p}tile)">
    <rect x="1" y="1" width="510" height="510" rx="115" fill="none" stroke="#FFFFFF" stroke-opacity="0.06" stroke-width="2"/>
    <circle cx="256" cy="212" r="140" fill="#7C3AED" opacity="0.42" filter="url(#{p}soft)"/>
    <circle cx="256" cy="366" r="124" fill="#0891B2" opacity="0.28" filter="url(#{p}soft)"/>
{STARS}
'''
        close = "\n  </g>"
    else:
        head = f'''
{STARS}
'''
        close = ""
    return f'''{head}
  <g fill="none" stroke-linecap="round" stroke-dasharray="10 18">
    <g stroke="#22D3EE" stroke-width="18" opacity="0.55" filter="url(#{p}tight)">
      <path d="M256 292 124 352M256 292v60M256 292l132 60"/>
    </g>
    <g stroke="#CFFAFE" stroke-width="5">
      <path d="M256 292 124 352M256 292v60M256 292l132 60"/>
    </g>
  </g>

  <g transform="matrix(1.066 0 0 1 -16.9 -41)">
    <use href="#{p}padShape" fill="#FF3DBE" opacity="0.5" filter="url(#{p}soft)"/>
    <use href="#{p}padShape" fill="url(#{p}pad)"/>
    <use href="#{p}padShape" fill="none" stroke="#F5D0FE" stroke-width="2" opacity="0.5"/>
    <g fill="#12052B">
      <rect x="166" y="167" width="48" height="14" rx="4"/>
      <rect x="183" y="150" width="14" height="48" rx="4"/>
      <circle cx="316" cy="152" r="8"/>
      <circle cx="294" cy="174" r="8"/>
      <circle cx="338" cy="174" r="8"/>
      <circle cx="316" cy="196" r="8"/>
    </g>
  </g>

  <circle cx="256" cy="251" r="63" fill="#22D3EE" opacity="0.32" filter="url(#{p}soft)"/>
  <circle cx="256" cy="251" r="52" fill="#0A0320"/>
  <circle cx="256" cy="251" r="47" fill="#0A0320" stroke="url(#{p}ring)" stroke-width="6"/>
  <use href="#{p}person" x="256" y="251" fill="url(#{p}ink)"/>

  <g>
    <circle cx="124" cy="397" r="44" fill="#0A0320" stroke="url(#{p}ring)" stroke-width="4"/>
    <circle cx="256" cy="397" r="44" fill="#0A0320" stroke="url(#{p}ring)" stroke-width="4"/>
    <circle cx="388" cy="397" r="44" fill="#0A0320" stroke="url(#{p}ring)" stroke-width="4"/>
  </g>
  <g transform="translate(124 399) scale(0.84)"><use href="#{p}person" fill="#F0ABFC"/></g>
  <g transform="translate(256 399) scale(0.84)"><use href="#{p}person" fill="#C4B5FD"/></g>
  <g transform="translate(388 399) scale(0.84)"><use href="#{p}person" fill="#67E8F9"/></g>{close}
'''


def mark_doc(p="m"):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="30 50 452 412" width="452" height="412" role="img" aria-label="LAN Party">
{base_defs(p)}
{icon_art(p, background=False)}</svg>
'''


def icon_doc(p="i"):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512" role="img" aria-label="LAN Party">
{base_defs(p)}
{icon_art(p)}</svg>
'''


# ---------------------------------------------------------------- tray
TRAY_FAN = '''<path d="M12 8.4 5.3 18M12 8.4 12 18M12 8.4 18.7 18" stroke="{c}" stroke-width="2.3" stroke-linecap="round" fill="none"/>
  <circle cx="12" cy="8.4" r="3.9" fill="{c}"/>
  <circle cx="5.3" cy="18" r="2.9" fill="{c}"/>
  <circle cx="12" cy="18" r="2.9" fill="{c}"/>
  <circle cx="18.7" cy="18" r="2.9" fill="{c}"/>'''


def tray_doc(color, p="t"):
    """24x24 monochrome tray mark: one player node fanning out to three."""
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" role="img" aria-label="LAN Party">
  <g transform="translate(12 12) scale(0.94) translate(-12 -12)">
  {TRAY_FAN.format(c=color)}
  </g>
</svg>
'''


def tray_color_doc():
    """24x24 tray mark: the fan glyph on the brand gradient tile."""
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" role="img" aria-label="LAN Party">
  <defs>
    <linearGradient id="tgrad" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#FF3DBE"/>
      <stop offset="0.55" stop-color="#8B5CF6"/>
      <stop offset="1" stop-color="#22D3EE"/>
    </linearGradient>
  </defs>
  <rect x="0.5" y="0.5" width="23" height="23" rx="6.2" fill="url(#tgrad)"/>
  <g transform="translate(12 12) scale(0.9) translate(-12 -12)">
  {TRAY_FAN.format(c="#FFFFFF")}
  </g>
</svg>
'''


# ---------------------------------------------------------------- lockup
def lockup_dark(light=False):
    bold = Face(FONT, 700)
    reg = Face(FONT, 400)

    tile = 176
    name_size = 112
    tag_size = 30
    gap = 48

    kern = {"LA": -0.018, "AN": -0.004}
    lan = bold.run("LAN ", name_size, tracking=-0.012 * name_size, kerning=kern)
    party = bold.run("Party", name_size, tracking=-0.012 * name_size)
    tag = reg.run("A LAN cable across the internet.", tag_size, tracking=0.004 * tag_size)

    name_w = lan["width"] + party["width"]
    text_x = tile + gap
    width = text_x + max(name_w, tag["width"])
    height = tile

    name_baseline = 96
    tag_baseline = name_baseline + 64
    block_top = name_baseline + min(lan["top"], party["top"])
    block_bottom = tag_baseline + tag["bottom"]
    shift = tile / 2 - (block_top + block_bottom) / 2
    name_baseline += shift
    tag_baseline += shift

    name_fill = "#FFFFFF" if not light else "#150C2B"
    tag_fill = "#A79BC4" if not light else "#5F5678"

    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width:.0f} {height}" width="{width:.0f}" height="{height}" role="img" aria-label="LAN Party — A LAN cable across the internet">
{base_defs("l")}
  <linearGradient id="lword" x1="0" y1="0" x2="{party["width"]:.0f}" y2="0" gradientUnits="userSpaceOnUse">
    <stop offset="0" stop-color="#FF3DBE"/>
    <stop offset="0.55" stop-color="#A855F7"/>
    <stop offset="1" stop-color="#22D3EE"/>
  </linearGradient>

  <g transform="translate(0 0) scale({tile / 512:.5f})">{icon_art("l")}</g>

  {text_el(lan, text_x, name_baseline, name_fill)}
  {text_el(party, text_x + lan["width"], name_baseline, "url(#lword)")}
  {text_el(tag, text_x, tag_baseline, tag_fill)}
</svg>
'''


# ---------------------------------------------------------------- banner
def banner_doc():
    bold = Face(FONT, 700)
    reg = Face(FONT, 400)

    W, H = 1600, 640
    name_size, tag_size = 170, 42
    x = 120
    icon_size = 400
    icon_x = W - 120 - icon_size
    icon_y = (H - icon_size) / 2

    lan = bold.run("LAN ", name_size, tracking=-0.012 * name_size, kerning={"LA": -0.018, "AN": -0.004})
    party = bold.run("Party", name_size, tracking=-0.012 * name_size)
    tag = reg.run("A LAN cable across the internet.", tag_size, tracking=0.004 * tag_size)

    name_baseline = 352
    tag_baseline = 416

    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" aria-label="LAN Party — A LAN cable across the internet">
{base_defs("b")}
  <pattern id="bdots" width="46" height="46" patternUnits="userSpaceOnUse">
    <circle cx="2" cy="2" r="1.1" fill="#FFFFFF" opacity="0.10"/>
  </pattern>
  <radialGradient id="bglowA" cx="0.5" cy="0.5" r="0.5">
    <stop offset="0" stop-color="#8B5CF6" stop-opacity="0.55"/>
    <stop offset="1" stop-color="#8B5CF6" stop-opacity="0"/>
  </radialGradient>
  <radialGradient id="bglowD" cx="0.5" cy="0.5" r="0.5">
    <stop offset="0" stop-color="#6D3BD8" stop-opacity="0.30"/>
    <stop offset="1" stop-color="#6D3BD8" stop-opacity="0"/>
  </radialGradient>
  <radialGradient id="bglowB" cx="0.5" cy="0.5" r="0.5">
    <stop offset="0" stop-color="#FF3DBE" stop-opacity="0.45"/>
    <stop offset="1" stop-color="#FF3DBE" stop-opacity="0"/>
  </radialGradient>
  <radialGradient id="bglowC" cx="0.5" cy="0.5" r="0.5">
    <stop offset="0" stop-color="#22D3EE" stop-opacity="0.35"/>
    <stop offset="1" stop-color="#22D3EE" stop-opacity="0"/>
  </radialGradient>
  <linearGradient id="bword" x1="0" y1="0" x2="{party["width"]:.0f}" y2="0" gradientUnits="userSpaceOnUse">
    <stop offset="0" stop-color="#FF3DBE"/>
    <stop offset="0.55" stop-color="#A855F7"/>
    <stop offset="1" stop-color="#22D3EE"/>
  </linearGradient>
  <filter id="bblur" x="-50%" y="-50%" width="200%" height="200%">
    <feGaussianBlur stdDeviation="55"/>
  </filter>
  <radialGradient id="bvign" cx="0.5" cy="0.5" r="0.75">
    <stop offset="0.5" stop-color="#000000" stop-opacity="0"/>
    <stop offset="1" stop-color="#000000" stop-opacity="0.45"/>
  </radialGradient>

  <rect width="{W}" height="{H}" fill="{INK}"/>
  <circle cx="1300" cy="60" r="620" fill="url(#bglowB)"/>
  <circle cx="1500" cy="600" r="520" fill="url(#bglowC)"/>
  <circle cx="820" cy="150" r="600" fill="url(#bglowA)"/>
  <circle cx="360" cy="330" r="520" fill="url(#bglowD)"/>
  <circle cx="20" cy="660" r="460" fill="url(#bglowC)"/>
  <rect width="{W}" height="{H}" fill="url(#bdots)"/>
  <rect width="{W}" height="{H}" fill="url(#bvign)"/>


  <!-- soft ring behind the mark -->
  <circle cx="{icon_x + icon_size / 2:.0f}" cy="{icon_y + icon_size / 2:.0f}" r="268" fill="none" stroke="#A78BFA" stroke-width="1.5" opacity="0.35"/>
  <circle cx="{icon_x + icon_size / 2:.0f}" cy="{icon_y + icon_size / 2:.0f}" r="316" fill="none" stroke="#22D3EE" stroke-width="1.5" opacity="0.20"/>

  <rect x="{icon_x - 10:.0f}" y="{icon_y - 10:.0f}" width="{icon_size + 20}" height="{icon_size + 20}" rx="124" fill="#8B5CF6" opacity="0.42" filter="url(#bblur)"/>
  <g transform="translate({icon_x:.0f} {icon_y:.0f}) scale({icon_size / 512:.5f})">{icon_art("b")}</g>

  {text_el(lan, x, name_baseline, "#FFFFFF")}
  {text_el(party, x + lan["width"], name_baseline, "url(#bword)")}
  {text_el(tag, x, tag_baseline, "#A79BC4")}
</svg>
'''


# ---------------------------------------------------------------- write
def main():
    os.makedirs(HERE, exist_ok=True)
    files = {
        "icon-app.svg": icon_doc("i"),
        "mark-transparent.svg": mark_doc("m"),
        "tray-icon-white.svg": tray_doc("#FFFFFF", "tw"),
        "tray-icon-black.svg": tray_doc("#141018", "tb"),
        "tray-icon-color.svg": tray_color_doc(),
        "lockup-horizontal-dark.svg": lockup_dark(False),
        "lockup-horizontal-light.svg": lockup_dark(True),
        "banner.svg": banner_doc(),
    }
    for name, data in files.items():
        with open(os.path.join(HERE, name), "w") as fh:
            fh.write(data)
        print("wrote", name, len(data), "bytes")


if __name__ == "__main__":
    main()
