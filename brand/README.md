# LAN Party — brand assets

Neon arcade mark: a controller feeding a player node that fans out to three
players. Everything here is hand-built SVG; no rasters, no embedded fonts.

## Files

| File | Use |
|---|---|
| `icon-app.svg` | Master app icon, 512×512 rounded tile. |
| `mark-transparent.svg` | The mark alone, no tile — for dark backgrounds. |
| `tray-icon-color.svg` | 24×24 tray mark: the player fan on the brand gradient tile. |
| `tray-icon-white.svg` / `tray-icon-black.svg` | 24×24 monochrome version for panels that need a solid glyph. |
| `lockup-horizontal-dark.svg` | Mark left, "LAN Party" + tagline right, for dark UI. |
| `lockup-horizontal-light.svg` | Same lockup for light UI. |
| `banner.svg` | 1600×640 hero banner. |
| `png/` | Rendered icons (16–512), lockups @2x, banner 1600×640. |
| `ico/` | `app-icon.ico` (16–256), `tray-icon-color.ico`, `tray-icon-white.ico`, `tray-icon-black.ico`. |
| `brand-sheet.png` | One-page overview of the whole set. |
| `build.py`, `fonts/` | Generator for all SVGs (Space Grotesk, SIL OFL). |

## Icons in practice

**Windows** — use `ico/app-icon.ico` for the app shortcut and
`ico/tray-icon-color.ico` for the notification area. If you prefer a flat
taskbar look, `tray-icon-white.ico` and `tray-icon-black.ico` are the same mark
as a solid glyph. All three contain 16/20/24/32/48 px frames.

**Linux** — install `png/icon-app-{16..512}.png` into
`~/.local/share/icons/hicolor/<size>/apps/lan-party.png`. For trays that follow
the panel colour, use `tray-icon-white.png` (dark panels) or
`tray-icon-black.png` (light panels) at 22 or 24 px; otherwise use
`tray-icon-color.png`.

**Web / macOS** — use `icon-app.svg` or `png/icon-app-512.png`.

The art sits inside a 14% safe margin, so platform masks (squircles, circles,
Windows 11 rounding) clip padding, not the mark. Every glow is clipped to the
tile, so the area outside the rounded square stays fully transparent — the
corner pixels are alpha 0 in all PNGs.

The tray mark is the app icon's bottom half: one player node fanning out to
three, on the brand gradient tile. It drops the controller on purpose — at
16–24 px a controller silhouette is indistinguishable from every other game
launcher, and stacking a controller above the fan turns into a hat. The colour
version keeps the tile; the white and black versions are the bare glyph for
panels that tint their own icons.

## Colour

| Token | Hex |
|---|---|
| Ink | `#08040F` |
| Deep violet | `#2C1166` |
| Magenta | `#FF3DBE` |
| Violet | `#8B5CF6` |
| Cyan | `#22D3EE` |
| Pale | `#FDF4FF` |

Wordmark: "LAN" in white (ink in the light lockup), "Party" in the
magenta→violet→cyan gradient. Tagline: "A LAN cable across the internet."

## Rebuilding

```
python3 build.py
```

Text is converted to outlines, so the SVGs render identically without Space
Grotesk installed. Re-render PNGs with `rsvg-convert`, e.g.
`rsvg-convert -w 512 -h 512 icon-app.svg -o png/icon-app-512.png`.
