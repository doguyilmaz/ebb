/** Chart colours come from the theme's CSS variables, read at draw time. */
export interface Palette {
  slots: string[];
  other: string;
  surface: string;
  text: string;
  text2: string;
  line: string;
}

export function readPalette(): Palette {
  const css = getComputedStyle(document.documentElement);
  const v = (n: string) => css.getPropertyValue(n).trim();
  return {
    slots: [1, 2, 3, 4, 5, 6, 7, 8].map((i) => v(`--s${i}`)),
    other: v("--other"),
    surface: v("--surface"),
    text: v("--text"),
    text2: v("--text-2"),
    line: v("--line"),
  };
}

function rgb(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  const n = parseInt(h.length === 3 ? h.replace(/./g, (c) => c + c) : h, 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

/** Mixes a colour toward the surface; deeper levels fade so the top level reads first. */
export function mix(a: string, b: string, t: number): string {
  const [r1, g1, b1] = rgb(a);
  const [r2, g2, b2] = rgb(b);
  const m = (x: number, y: number) => Math.round(x + (y - x) * t);
  return `rgb(${m(r1, r2)}, ${m(g1, g2)}, ${m(b1, b2)})`;
}

/** Colour follows the top-level folder (fixed slot order), never the rank of a deeper item. */
export function fill(p: Palette, top: number, depth: number, kind: number, other?: number): string {
  const base = kind >= 3 || other || top < 0 || top >= p.slots.length ? p.other : p.slots[top];
  return mix(base, p.surface, Math.min(0.62, (depth - 1) * 0.17));
}
