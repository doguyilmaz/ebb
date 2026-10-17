import { bytes } from "#/lib/format.ts";
import { css, fill, inkOn, type Palette, type RGB } from "./colors.ts";
import { HEADER, type Arc, type Rect } from "./layout.ts";
import type { Frame } from "./morph.ts";

const FONT = "Plex, system-ui, sans-serif";
const DIM = 0.26;
const HALF_PI = Math.PI / 2;

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
    if (widths.size > 20000) widths.clear();
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

const colorOf = (p: Palette, s: Arc | Rect, depthFade: boolean): RGB => fill(p, s.top, depthFade ? s.tone : 1, s.node.kind, s.node.other);

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

export function drawArcs(ctx: CanvasRenderingContext2D, frames: Frame<Arc>[], cx: number, cy: number, p: Palette, em: Emphasis) {
  const b = new Batches();
  for (const { shape: a, alpha } of frames) {
    const len = (a.a1 - a.a0) * a.r1;
    if (len < 0.5 || a.r1 - a.r0 < 0.5 || alpha <= 0.01) continue;
    const k = em.keys && !em.keys.has(a.key) ? DIM : 1;
    arcPath(b.get(css(colorOf(p, a, true)), alpha * k, len > 4), a, cx, cy);
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
export function drawArcLabels(ctx: CanvasRenderingContext2D, arcs: Arc[], cx: number, cy: number, p: Palette, em: Emphasis, alpha: number) {
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  for (const a of arcs) {
    if (a.depth > 3) continue;
    const span = a.a1 - a.a0;
    const thick = a.r1 - a.r0;
    const rm = (a.r0 + a.r1) / 2;
    const along = span * rm;
    if (thick < 13 || along < 13) continue;
    const name = a.node.other ? "other" : a.node.name;
    ctx.font = `${a.depth === 1 ? 600 : 500} 11.5px ${FONT}`;
    const mid = (a.a0 + a.a1) / 2;
    let text = "";
    let radial = false;
    const keep = a.depth === 1 ? 4 : 7;
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
    ctx.globalAlpha = alpha * (em.keys && !em.keys.has(a.key) ? DIM : 1);
    ctx.fillStyle = inkOn(colorOf(p, a, true));
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

function rectPath(path: Path2D, r: Rect, radius: number) {
  const w = r.x1 - r.x0;
  const h = r.y1 - r.y0;
  path.roundRect(r.x0, r.y0, w, h, Math.min(radius, w / 2, h / 2));
}

export function drawRects(ctx: CanvasRenderingContext2D, frames: Frame<Rect>[], p: Palette, em: Emphasis, nested: boolean, w: number, h: number) {
  const b = new Batches();
  for (const { shape: r, alpha } of frames) {
    if (r.x1 - r.x0 < 0.5 || r.y1 - r.y0 < 0.5 || alpha <= 0.01 || r.x1 < 0 || r.y1 < 0 || r.x0 > w || r.y0 > h) continue;
    const k = em.keys && !em.keys.has(r.key) ? DIM : 1;
    rectPath(b.get(css(colorOf(p, r, nested)), alpha * k), nested ? r : { ...r, x1: r.x1 - 1, y1: r.y1 - 1 }, nested ? 4 : 3);
  }
  b.flush(ctx, p.surfaceCss, 0);
  const hovered = em.hover && frames.find((f) => f.shape.key === em.hover);
  if (hovered) {
    const r = hovered.shape;
    ctx.lineWidth = 2;
    ctx.strokeStyle = p.text;
    ctx.beginPath();
    ctx.roundRect(r.x0 + 1, r.y0 + 1, Math.max(0, r.x1 - r.x0 - 2), Math.max(0, r.y1 - r.y0 - 2), 3);
    ctx.stroke();
  }
}

/** Treemap: folders name their header band, leaves name themselves when there is room. Icicle: one name per cell. */
export function drawRectLabels(ctx: CanvasRenderingContext2D, rects: Rect[], p: Palette, em: Emphasis, nested: boolean, alpha: number) {
  ctx.textBaseline = "middle";
  for (const r of rects) {
    const w = r.x1 - r.x0;
    const h = r.y1 - r.y0;
    const parent = nested && !!r.node.children?.length;
    if (w < 44 || h < (parent ? HEADER : 16)) continue;
    const ink = inkOn(colorOf(p, r, nested));
    ctx.globalAlpha = alpha * (em.keys && !em.keys.has(r.key) ? DIM : 1);
    ctx.fillStyle = ink;
    const name = r.node.other ? `${r.node.other} more` : r.node.name;
    const size = bytes(r.node.alloc);
    if (parent) {
      ctx.font = `400 11px ${FONT}`;
      const sw = measure(ctx, size);
      ctx.font = `600 11.5px ${FONT}`;
      ctx.textAlign = "left";
      const t = fitText(ctx, name, w - 14 - (w > sw + 90 ? sw + 8 : 0));
      if (t) ctx.fillText(t, r.x0 + 6, r.y0 + HEADER / 2 + 1);
      if (w > sw + 90) {
        ctx.font = `400 11px ${FONT}`;
        ctx.textAlign = "right";
        ctx.globalAlpha *= 0.8;
        ctx.fillText(size, r.x1 - 6, r.y0 + HEADER / 2 + 1);
      }
      continue;
    }
    ctx.textAlign = "left";
    ctx.font = `${nested && r.depth > 1 ? 400 : 500} 11.5px ${FONT}`;
    const t = fitText(ctx, name, w - 12);
    if (!t) continue;
    const two = h >= 34;
    const y = nested ? r.y0 + 11 : r.y0 + h / 2 - (two ? 7 : 0);
    ctx.fillText(t, r.x0 + 6, y);
    if (two) {
      ctx.font = `400 11px ${FONT}`;
      ctx.globalAlpha *= 0.8;
      ctx.fillText(size, r.x0 + 6, y + 15);
    }
  }
  ctx.globalAlpha = 1;
  ctx.textAlign = "left";
}
