import { bytes } from "#/lib/format.ts";
import { ageFill, css, fill, frame, inkOn, type ColorMode, type Palette, type RGB } from "./colors.ts";
import { HEADER, isFramed, type Arc, type Rect, type Shape } from "./layout.ts";
import type { Frame } from "./morph.ts";

const FONT = "Plex, system-ui, sans-serif";
const DIM = 0.26;
const HALF_PI = Math.PI / 2;

export type Colorer = (s: Shape) => RGB;

/** How a view colours a shape: by its coloured ancestor or by age, with folder frames in the treemap. */
export function colorer(p: Palette, mode: ColorMode, view: "sunburst" | "treemap" | "icicle", now: number, cuts: number[]): Colorer {
  return (s) => {
    const c = mode === "age" ? ageFill(p, s.node.modified, now, cuts) : fill(p, s.top, s.tone, s.node.kind, s.node.other, s.rank);
    return view === "treemap" && s.node.children?.length ? frame(c) : c;
  };
}

/**
 * Shapes are grouped by colour and opacity so one frame is a few dozen fills
 * instead of thousands; that keeps morphs smooth on a whole-disk slice.
 */
class Batches {
  private paths = new Map<string, { path: Path2D; color: string; alpha: number; gap: boolean }>();
  /** gap: separate the shape from its neighbours; off for slivers, where the gaps would be all you see. */
  get(color: string, alpha: number, gap = true): Path2D {
    const a = Math.round(alpha * 20) / 20;
    const k = `${color}|${a}|${gap}`;
    let b = this.paths.get(k);
    if (!b) this.paths.set(k, (b = { path: new Path2D(), color, alpha: a, gap }));
    return b.path;
  }
  flush(ctx: CanvasRenderingContext2D, surface: string, width: number) {
    ctx.lineWidth = width;
    ctx.strokeStyle = surface;
    for (const b of this.paths.values()) {
      if (b.alpha <= 0) continue;
      ctx.globalAlpha = b.alpha;
      ctx.fillStyle = b.color;
      ctx.fill(b.path);
      if (width > 0 && b.gap) ctx.stroke(b.path);
    }
    ctx.globalAlpha = 1;
  }
}

const widths = new Map<string, number>();
function measure(ctx: CanvasRenderingContext2D, text: string): number {
  const k = ctx.font + "\u0000" + text;
  let w = widths.get(k);
  if (w === undefined) {
    if (widths.size > 30000) widths.clear();
    widths.set(k, (w = ctx.measureText(text).width));
  }
  return w;
}

/** The text, shortened with an ellipsis to fit max, or "" when fewer than keep letters would remain. */
export function fitText(ctx: CanvasRenderingContext2D, text: string, max: number, keep = 3): string {
  if (measure(ctx, text) <= max) return text;
  let lo = 0;
  let hi = text.length;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (measure(ctx, text.slice(0, mid) + "…") <= max) lo = mid;
    else hi = mid - 1;
  }
  return lo >= Math.min(keep, text.length - 1) && lo > 0 ? text.slice(0, lo) + "…" : "";
}

const nameOf = (s: Shape) => (s.node.other ? `${s.node.other} more` : s.node.name);

function arcPath(path: Path2D, a: Arc, cx: number, cy: number) {
  path.moveTo(cx + a.r1 * Math.cos(a.a0 - HALF_PI), cy + a.r1 * Math.sin(a.a0 - HALF_PI));
  path.arc(cx, cy, a.r1, a.a0 - HALF_PI, a.a1 - HALF_PI);
  path.arc(cx, cy, Math.max(0, a.r0), a.a1 - HALF_PI, a.a0 - HALF_PI, true);
  path.closePath();
}

export interface Emphasis {
  keys: Set<string> | null;
  hover: string | null;
}

const dim = (em: Emphasis, key: string) => (em.keys && !em.keys.has(key) ? DIM : 1);

export function drawArcs(ctx: CanvasRenderingContext2D, frames: Frame<Arc>[], cx: number, cy: number, p: Palette, em: Emphasis, color: Colorer) {
  const b = new Batches();
  for (const { shape: a, alpha } of frames) {
    const len = (a.a1 - a.a0) * a.r1;
    if (len < 0.5 || a.r1 - a.r0 < 0.5 || alpha <= 0.01) continue;
    arcPath(b.get(css(color(a)), alpha * dim(em, a.key), len > 4), a, cx, cy);
  }
  b.flush(ctx, p.surfaceCss, 1);
  const hovered = em.hover && frames.find((f) => f.shape.key === em.hover);
  if (hovered) {
    const path = new Path2D();
    arcPath(path, hovered.shape, cx, cy);
    ctx.lineWidth = 2;
    ctx.strokeStyle = p.text;
    ctx.stroke(path);
  }
}

/** Names on arcs that have room: along the ring when wide, along the radius when thin. */
export function drawArcLabels(ctx: CanvasRenderingContext2D, arcs: Arc[], cx: number, cy: number, em: Emphasis, color: Colorer, alpha: number) {
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  for (const a of arcs) {
    if (a.depth > 3) continue;
    const thick = a.r1 - a.r0;
    const rm = (a.r0 + a.r1) / 2;
    const along = (a.a1 - a.a0) * rm;
    if (thick < 13 || along < 13) continue;
    const name = nameOf(a);
    ctx.font = `${a.depth === 1 ? 600 : 500} 11.5px ${FONT}`;
    const mid = (a.a0 + a.a1) / 2;
    const keep = a.depth === 1 ? 4 : 7;
    let text = "";
    let radial = false;
    if (a.depth === 1 || along > thick) {
      text = fitText(ctx, name, along - 10, keep);
      if (!text && thick > 40) {
        text = fitText(ctx, name, thick - 10, keep);
        radial = !!text;
      }
    } else {
      text = fitText(ctx, name, thick - 10, keep);
      radial = true;
    }
    if (!text) continue;
    const two = !radial && thick >= 34 && along > 46;
    ctx.globalAlpha = alpha * dim(em, a.key);
    ctx.fillStyle = inkOn(color(a));
    ctx.save();
    ctx.translate(cx + rm * Math.sin(mid), cy - rm * Math.cos(mid));
    if (radial) ctx.rotate(mid > Math.PI ? mid + HALF_PI : mid - HALF_PI);
    else ctx.rotate(mid > HALF_PI && mid < 3 * HALF_PI ? mid + Math.PI : mid);
    ctx.fillText(text, 0, two ? -7 : 0);
    if (two) {
      ctx.font = `400 11px ${FONT}`;
      ctx.globalAlpha *= 0.85;
      ctx.fillText(bytes(a.node.alloc), 0, 8);
    }
    ctx.restore();
  }
  ctx.globalAlpha = 1;
}

/** The middle names the folder on screen, or invites going back up. */
export function drawCenter(ctx: CanvasRenderingContext2D, cx: number, cy: number, rc: number, p: Palette, title: string, value: string, sub: string, back: string | null) {
  ctx.beginPath();
  ctx.arc(cx, cy, rc - 3, 0, Math.PI * 2);
  ctx.fillStyle = p.surfaceCss;
  ctx.fill();
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  const w = rc * 1.62;
  ctx.font = `500 12.5px ${FONT}`;
  ctx.fillStyle = back ? p.accent : p.text2;
  ctx.fillText(fitText(ctx, back ? `↑ ${back}` : title, w) || "…", cx, cy - 20);
  ctx.font = `600 ${rc > 70 ? 22 : 17}px ${FONT}`;
  ctx.fillStyle = p.text;
  ctx.fillText(value, cx, cy + 2);
  ctx.font = `400 11.5px ${FONT}`;
  ctx.fillStyle = p.text3;
  ctx.fillText(fitText(ctx, sub, w), cx, cy + 22);
}

function rectPath(path: Path2D, x: number, y: number, w: number, h: number, radius: number) {
  if (w < 8 || h < 8) path.rect(x, y, w, h);
  else path.roundRect(x, y, w, h, radius);
}

/** gap: surface left between neighbouring cells (icicle); the treemap's padding already separates them. */
export function drawRects(ctx: CanvasRenderingContext2D, frames: Frame<Rect>[], p: Palette, em: Emphasis, color: Colorer, w: number, h: number, gap: number) {
  const b = new Batches();
  for (const { shape: r, alpha } of frames) {
    const rw = r.x1 - r.x0 - gap;
    const rh = r.y1 - r.y0 - gap;
    if (rw < 0.5 || rh < 0.5 || alpha <= 0.01 || r.x1 < 0 || r.y1 < 0 || r.x0 > w || r.y0 > h) continue;
    rectPath(b.get(css(color(r)), alpha * dim(em, r.key)), r.x0, r.y0, rw, rh, 3);
  }
  b.flush(ctx, p.surfaceCss, 0);
  const hovered = em.hover && frames.find((f) => f.shape.key === em.hover);
  if (hovered) {
    const r = hovered.shape;
    ctx.lineWidth = 2;
    ctx.strokeStyle = p.text;
    ctx.beginPath();
    ctx.roundRect(r.x0 + 1, r.y0 + 1, Math.max(0, r.x1 - r.x0 - 2 - gap), Math.max(0, r.y1 - r.y0 - 2 - gap), 3);
    ctx.stroke();
  }
}

/** Treemap: framed folders name their band, boxes name themselves, bigger boxes in bigger type. */
export function drawTreemapLabels(ctx: CanvasRenderingContext2D, rects: Rect[], em: Emphasis, color: Colorer, alpha: number) {
  ctx.textBaseline = "middle";
  for (const r of rects) {
    const w = r.x1 - r.x0;
    const h = r.y1 - r.y0;
    if (w < 34 || h < 14) continue;
    const folder = !!r.node.children?.length;
    if (folder && !isFramed(r)) continue;
    ctx.globalAlpha = alpha * dim(em, r.key);
    ctx.fillStyle = inkOn(color(r));
    const size = bytes(r.node.alloc);
    if (folder) {
      ctx.font = `400 11px ${FONT}`;
      const sw = measure(ctx, size);
      const room = w > sw + 70;
      ctx.font = `600 11.5px ${FONT}`;
      ctx.textAlign = "left";
      const t = fitText(ctx, nameOf(r), w - 12 - (room ? sw + 8 : 0));
      if (t) ctx.fillText(t, r.x0 + 6, r.y0 + HEADER / 2 + 1);
      if (room) {
        ctx.font = `400 11px ${FONT}`;
        ctx.textAlign = "right";
        ctx.globalAlpha *= 0.8;
        ctx.fillText(size, r.x1 - 6, r.y0 + HEADER / 2 + 1);
      }
      continue;
    }
    const big = w > 150 && h > 56;
    ctx.textAlign = "left";
    ctx.font = `${big ? 600 : 500} ${big ? 13.5 : 11.5}px ${FONT}`;
    const t = fitText(ctx, nameOf(r), w - 12);
    if (!t) continue;
    const top = r.y0 + (big ? 14 : 10);
    ctx.fillText(t, r.x0 + 6, top);
    if (h >= (big ? 44 : 30)) {
      ctx.font = `400 ${big ? 12 : 11}px ${FONT}`;
      ctx.globalAlpha *= 0.8;
      ctx.fillText(size, r.x0 + 6, top + (big ? 17 : 14));
    }
  }
  ctx.globalAlpha = 1;
  ctx.textAlign = "left";
}

/** Icicle: one name per cell, its size below it when the cell is tall enough. */
export function drawIcicleLabels(ctx: CanvasRenderingContext2D, rects: Rect[], em: Emphasis, color: Colorer, alpha: number) {
  ctx.textBaseline = "middle";
  ctx.textAlign = "left";
  for (const r of rects) {
    const w = r.x1 - r.x0;
    const h = r.y1 - r.y0;
    if (w < 40 || h < 15) continue;
    ctx.font = `${r.depth === 1 ? 600 : 500} 11.5px ${FONT}`;
    const t = fitText(ctx, nameOf(r), w - 14);
    if (!t) continue;
    ctx.globalAlpha = alpha * dim(em, r.key);
    ctx.fillStyle = inkOn(color(r));
    const two = h >= 34;
    const y = r.y0 + h / 2 - (two ? 7 : 0);
    ctx.fillText(t, r.x0 + 7, y);
    if (two) {
      ctx.font = `400 11px ${FONT}`;
      ctx.globalAlpha *= 0.8;
      ctx.fillText(bytes(r.node.alloc), r.x0 + 7, y + 15);
    }
  }
  ctx.globalAlpha = 1;
}
