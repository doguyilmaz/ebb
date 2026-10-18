"""ebb's icon: a sunburst whose outer part ebbs away, around an e drawn as a
crescent moon with a wave for its crossbar (the moon and the tide it moves).

    python3 assets/brand/brand.py     # the SVG sources here, and ui/public/favicon.svg
    node assets/brand/export.mjs      # PNGs, the macOS .icns files and ui/public/apple-touch-icon.png

Variants: spectrum (the default), tide (one hue), age (the Explore age colours).
Each comes as icon-*.svg (macOS app icon: 824 px body on a 1024 canvas, with
shadow) and mark-*.svg (the same artwork filling the canvas, for the web and UI).
"""
import math
import os

OUT = os.path.dirname(os.path.abspath(__file__))

PALETTES = {
    "spectrum": ["#38d9c6", "#33bfe9", "#4b8ff2", "#7a7cf1", "#a07cf0"],
    "tide": ["#5fe3e6", "#36c9d6", "#22a9bf", "#1f88a8", "#2a6a95"],
    "age": ["#174a52", "#1f6a74", "#2a8f99", "#3db6bf", "#7ae6ea"],
}

# Each segment: start and end angle (clockwise from the top is -90), colour index, opacity.
INNER = [(-90, -4, 0, 1), (-4, 70, 1, 1), (70, 140, 2, 1), (140, 200, 3, 1), (200, 236, 4, 0.75)]
OUTER = [(-90, -46, 0, 1), (-46, -4, 0, 1), (-4, 36, 1, 1), (36, 70, 1, 1), (70, 108, 2, 1), (108, 140, 2, 0.95),
         (140, 172, 3, 0.8), (172, 200, 3, 0.6), (200, 220, 4, 0.4), (220, 230, 4, 0.22)]
# Age mode colours by how old each folder is, not by where it sits.
AGE_INNER = [3, 1, 4, 2, 0]
AGE_OUTER = [2, 4, 1, 3, 4, 2, 0, 3, 1, 0]

CX = CY = 512
R0, R1, R2, R3 = 160, 234, 245, 300
GAP = 4.2
E_R = 106


def pt(r, a):
    t = math.radians(a)
    return CX + r * math.cos(t), CY + r * math.sin(t)


def sector(r0, r1, a0, a1):
    large = 1 if (a1 - a0) % 360 > 180 else 0
    x0, y0 = pt(r1, a0)
    x1, y1 = pt(r1, a1)
    x2, y2 = pt(r0, a1)
    x3, y3 = pt(r0, a0)
    return (f"M{x0:.1f},{y0:.1f} A{r1},{r1} 0 {large} 1 {x1:.1f},{y1:.1f} "
            f"L{x2:.1f},{y2:.1f} A{r0},{r0} 0 {large} 0 {x3:.1f},{y3:.1f} Z")


def arc(r, a0, a1):
    large = 1 if (a1 - a0) % 360 > 180 else 0
    x0, y0 = pt(r, a0)
    x1, y1 = pt(r, a1)
    return f"M{x0:.1f},{y0:.1f} A{r},{r} 0 {large} 1 {x1:.1f},{y1:.1f}"


def wave(x0, x1, y, amp):
    length = x1 - x0
    return (f"M{x0:.1f},{y:.1f} C{x0 + length * 0.17:.1f},{y - amp:.1f} {x0 + length * 0.33:.1f},{y - amp:.1f} "
            f"{x0 + length * 0.5:.1f},{y:.1f} S{x0 + length * 0.83:.1f},{y + amp:.1f} {x1:.1f},{y:.1f}")


def crescent_e():
    """Two circles make the crescent (heavy left, horns to the lower right); a
    wave from its inside edge to the upper horn closes the e."""
    r = E_R
    ix, iy, rr = CX + 0.25 * r, CY + 0.25 * r, 0.80 * r
    w = wave(ix - rr + 0.02 * r, CX + r + 4, CY - 0.03 * r, 0.105 * r)
    defs = ('<mask id="e"><rect width="1024" height="1024" fill="#000"/>'
            f'<circle cx="{CX}" cy="{CY}" r="{r}" fill="#fff"/><circle cx="{ix:.1f}" cy="{iy:.1f}" r="{rr:.1f}" fill="#000"/>'
            f'<path d="{w}" fill="none" stroke="#fff" stroke-width="{0.185 * r:.1f}" stroke-linecap="round"/></mask>'
            f'<clipPath id="eo"><circle cx="{CX}" cy="{CY}" r="{r}"/></clipPath>')
    body = (f'<g filter="url(#esh)"><g clip-path="url(#eo)"><g mask="url(#e)">'
            f'<rect x="{CX - r - 6}" y="{CY - r - 6}" width="{2 * r + 12}" height="{2 * r + 12}" fill="url(#ef)"/></g></g></g>')
    return defs, body


def artwork(variant):
    pal = PALETTES[variant]
    inner = [(a0, a1, AGE_INNER[i] if variant == "age" else c, op) for i, (a0, a1, c, op) in enumerate(INNER)]
    outer = [(a0, a1, AGE_OUTER[i] if variant == "age" else c, op) for i, (a0, a1, c, op) in enumerate(OUTER)]

    def seg(r0, r1, a0, a1, c, op):
        d = sector(r0 + 4, r1 - 4, a0 + GAP / 2, a1 - GAP / 2)
        # A stroke in the segment's own colour rounds its corners.
        return (f'<g opacity="{op}"><path d="{d}" fill="{pal[c]}" stroke="{pal[c]}" stroke-width="8" stroke-linejoin="round"/>'
                f'<path d="{d}" fill="url(#gl)" stroke="url(#gl)" stroke-width="8" stroke-linejoin="round"/></g>')

    rims = [f'<path d="{arc(r, a0 + GAP / 2 + 1.4, a1 - GAP / 2 - 1.4)}" fill="none" stroke="#fff" '
            f'stroke-opacity="{0.5 * op:.2f}" stroke-width="3.5" stroke-linecap="round"/>'
            for r, ring in [(R1 - 2, inner), (R3 - 2, outer)] for a0, a1, c, op in ring]
    edefs, ebody = crescent_e()
    defs = f'''{edefs}
<radialGradient id="bg" cx="0.5" cy="0.4" r="0.8"><stop offset="0" stop-color="#162436"/><stop offset="0.7" stop-color="#0a1019"/><stop offset="1" stop-color="#05070b"/></radialGradient>
<radialGradient id="ef" cx="0.3" cy="0.25" r="0.95"><stop offset="0" stop-color="#ffffff"/><stop offset="0.6" stop-color="#e2fbfb"/><stop offset="1" stop-color="#9eeaee"/></radialGradient>
<radialGradient id="eg" cx="0.5" cy="0.5" r="0.5"><stop offset="0" stop-color="#bff6f7" stop-opacity="0.30"/><stop offset="1" stop-color="#bff6f7" stop-opacity="0"/></radialGradient>
<radialGradient id="hole" cx="0.45" cy="0.4" r="0.6"><stop offset="0" stop-color="#132637"/><stop offset="1" stop-color="#070c12"/></radialGradient>
<filter id="sh" x="-30%" y="-30%" width="160%" height="160%"><feDropShadow dx="0" dy="12" stdDeviation="14" flood-color="#000" flood-opacity="0.55"/></filter>
<filter id="esh" x="-40%" y="-40%" width="180%" height="180%"><feDropShadow dx="0" dy="6" stdDeviation="8" flood-color="#000" flood-opacity="0.45"/></filter>
<linearGradient id="gl" gradientUnits="userSpaceOnUse" x1="250" y1="190" x2="770" y2="860"><stop offset="0" stop-color="#fff" stop-opacity="0.34"/><stop offset="0.45" stop-color="#fff" stop-opacity="0.05"/><stop offset="0.7" stop-color="#fff" stop-opacity="0"/></linearGradient>'''
    body = ('<rect width="1024" height="1024" fill="url(#bg)"/>'
            '<g filter="url(#sh)">' + "".join(seg(R0, R1, *s) for s in inner) + "".join(seg(R2, R3, *s) for s in outer) + '</g>'
            + "".join(rims) +
            f'<circle cx="{CX}" cy="{CY}" r="{R0 - 10}" fill="url(#hole)"/>'
            f'<circle cx="{CX}" cy="{CY}" r="{R0 - 10}" fill="none" stroke="#fff" stroke-opacity="0.07" stroke-width="2"/>'
            f'<circle cx="{CX}" cy="{CY}" r="{R0 - 20}" fill="url(#eg)"/>' + ebody)
    return defs, body


def icon(variant):
    """The macOS app icon: the artwork on an 824 px squircle body with its shadow."""
    defs, body = artwork(variant)
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">
<defs>{defs}
<clipPath id="body"><rect x="100" y="100" width="824" height="824" rx="186"/></clipPath>
<filter id="iconShadow" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="14" stdDeviation="16" flood-color="#000" flood-opacity="0.32"/></filter>
<linearGradient id="rim" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity="0.28"/><stop offset="0.5" stop-color="#fff" stop-opacity="0.04"/><stop offset="1" stop-color="#fff" stop-opacity="0.10"/></linearGradient>
</defs>
<g filter="url(#iconShadow)"><g clip-path="url(#body)">{body}</g></g>
<rect x="101.5" y="101.5" width="821" height="821" rx="185" fill="none" stroke="url(#rim)" stroke-width="3"/>
</svg>
'''


def mark(variant):
    """The same artwork filling its own rounded square, for favicons and the app's UI: scaled so the
    chart fills it as it fills the app icon's body (512 / 412)."""
    defs, body = artwork(variant)
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">
<defs>{defs}
<clipPath id="body"><rect width="1024" height="1024" rx="230"/></clipPath>
</defs>
<g clip-path="url(#body)"><g transform="translate(512 512) scale(1.2427) translate(-512 -512)">{body}</g></g>
</svg>
'''


if __name__ == "__main__":
    for v in PALETTES:
        for kind, fn in [("icon", icon), ("mark", mark)]:
            with open(os.path.join(OUT, f"{kind}-{v}.svg"), "w") as f:
                f.write(fn(v))
    # The web app's favicon and sidebar mark.
    with open(os.path.join(OUT, "..", "..", "ui", "public", "favicon.svg"), "w") as f:
        f.write(mark("spectrum"))
