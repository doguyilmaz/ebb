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
  tide: RGB;
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
    tide: rgb(v("--tide")),
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

/**
 * Hue follows the coloured ancestor (fixed slot order), never a rank. Deeper
 * levels fade toward the surface and smaller siblings a little more, so
 * neighbours stay apart without new hues.
 */
export function fill(p: Palette, top: number, tone: number, kind: number, other = 0, rank = 0): RGB {
  const base = kind >= 3 || other || top < 0 || top >= p.slots.length ? p.other : p.slots[top];
  return mix(base, p.surface, Math.min(0.66, (tone - 1) * 0.15 + rank * 0.2));
}

const BLACK: RGB = [0, 0, 0];
/** A folder's frame: its own colour, darker, so the boxes inside read as one group. */
export const frame = (c: RGB): RGB => mix(c, BLACK, 0.24);

export type ColorMode = "folder" | "age";

/** One hue from the surface to the tide: older data is stronger. cuts are bucket bounds in days, ascending. */
export function ageFill(p: Palette, modified: number | undefined, now: number, cuts: number[]): RGB {
  if (!modified) return p.other;
  const days = (now - modified) / 86400;
  let i = 0;
  while (i < cuts.length && days >= cuts[i]) i++;
  return mix(p.surface, p.tide, 0.22 + (0.78 * i) / Math.max(1, cuts.length));
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
