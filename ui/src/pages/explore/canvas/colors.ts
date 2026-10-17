/** Chart colours come from the theme's CSS variables, read when the theme changes. */
export interface Palette {
  slots: RGB[];
  other: RGB;
  surface: RGB;
  surfaceCss: string;
  text: string;
  text2: string;
  text3: string;
  accent: string;
}

export type RGB = [number, number, number];

export function readPalette(): Palette {
  const css = getComputedStyle(document.documentElement);
  const v = (n: string) => css.getPropertyValue(n).trim();
  return {
    slots: [1, 2, 3, 4, 5, 6, 7, 8].map((i) => rgb(v(`--s${i}`))),
    other: rgb(v("--other")),
    surface: rgb(v("--surface")),
    surfaceCss: v("--surface"),
    text: v("--text"),
    text2: v("--text-2"),
    text3: v("--text-3"),
    accent: v("--accent"),
  };
}

export function rgb(hex: string): RGB {
  const h = hex.replace("#", "");
  const n = parseInt(h.length === 3 ? h.replace(/./g, (c) => c + c) : h, 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

export const css = ([r, g, b]: RGB) => `rgb(${r},${g},${b})`;

/** Mixes a colour toward another; deeper levels fade toward the surface so the top level reads first. */
export function mix(a: RGB, b: RGB, t: number): RGB {
  return [Math.round(a[0] + (b[0] - a[0]) * t), Math.round(a[1] + (b[1] - a[1]) * t), Math.round(a[2] + (b[2] - a[2]) * t)];
}

/** Colour follows the top-level item (fixed slot order), never the rank of a deeper one. */
export function fill(p: Palette, top: number, depth: number, kind: number, other?: number): RGB {
  const base = kind >= 3 || other || top < 0 || top >= p.slots.length ? p.other : p.slots[top];
  return mix(base, p.surface, Math.min(0.6, (depth - 1) * 0.16));
}

const lum = ([r, g, b]: RGB) => {
  const c = (v: number) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * c(r) + 0.7152 * c(g) + 0.0722 * c(b);
};

const INK = "#0b1720";
const INK_L = lum(rgb(INK));

/** Text on a coloured fill: white or ink, whichever contrasts more. */
export function inkOn(c: RGB): string {
  const l = lum(c);
  return (1.05) / (l + 0.05) >= (l + 0.05) / (INK_L + 0.05) ? "#ffffff" : INK;
}
