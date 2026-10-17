import { hierarchy, partition, treemap as d3treemap, treemapSquarify, type HierarchyNode } from "d3-hierarchy";
import type { SliceNode } from "#/api/types.gen.ts";

export type Metric = "alloc" | "unique";

export interface Shape {
  node: SliceNode;
  /** Filesystem path, or "" for merged "other" and small-files nodes. */
  path: string;
  /** Stable across layouts and zoom levels; morphs match shapes by it. */
  key: string;
  /** Index of the parent shape in the same layout, -1 for the top level. */
  parent: number;
  depth: number;
  /** Index of the coloured ancestor among its siblings; colours follow it. */
  top: number;
  /** 1 at the coloured level, growing with depth so deeper shapes fade. */
  tone: number;
  /** Position among its siblings by size, 0 for the biggest to 1 for the smallest. */
  rank: number;
}
export interface Arc extends Shape {
  a0: number;
  a1: number;
  r0: number;
  r1: number;
}
export interface Rect extends Shape {
  x0: number;
  y0: number;
  x1: number;
  y1: number;
}

/** Fraction of the radius the centre (current folder) takes. */
export const CENTER = 0.24;
/** Height of a treemap folder's name band. */
export const HEADER = 17;

const framedBox = (depth: number, folder: boolean, w: number, h: number) => depth >= 1 && folder && w > 40 && h > HEADER + 14;
/** A treemap folder big enough for a frame and a name band; smaller ones spend their area on their contents. */
export const isFramed = (r: Rect) => framedBox(r.depth, !!r.node.children?.length, r.x1 - r.x0, r.y1 - r.y0);

const navigable = (n: SliceNode) => (n.kind === 0 || n.kind === 1 || n.kind === 2) && !n.other;
const join = (base: string, name: string) => (base.endsWith("/") ? base + name : `${base}/${name}`);

/** The key a child of the folder at base gets in every layout. */
export const childKey = (base: string, n: SliceNode) => (navigable(n) ? join(base, n.name) : `${base}\u0000${n.kind}`);
/** The path a child of the folder at base opens, or "" when it is a merged group. */
export const childPath = (base: string, n: SliceNode) => (navigable(n) ? join(base, n.name) : "");

/**
 * Colours start at the first level that branches: a folder that is nearly all
 * one subfolder (like /Users holding one home) passes its colours down a level.
 */
export function colorLevel(root: SliceNode, metric: Metric): SliceNode[] {
  const chain: SliceNode[] = [];
  for (let n = root; ; ) {
    const big = (n.children ?? []).find((k) => k.kind === 0 && k.children?.length && k[metric] >= 0.8 * n[metric]);
    if (!big) return chain;
    chain.push(big);
    n = big;
  }
}

function build(root: SliceNode, metric: Metric) {
  return hierarchy(root, (d) => d.children ?? undefined).sum((d) => (d.children?.length ? 0 : Math.max(0, d[metric])));
}

function shapes<S extends Shape>(h: HierarchyNode<SliceNode>, base: string, metric: Metric, geometry: (d: HierarchyNode<SliceNode>) => Omit<S, keyof Shape>): S[] {
  const all = h.descendants().slice(1);
  const level = colorLevel(h.data, metric).length + 1;
  const index = new Map(all.map((d, i) => [d, i]));
  const keys = new Map<HierarchyNode<SliceNode>, string>([[h, base]]);
  const paths = new Map<HierarchyNode<SliceNode>, string>([[h, base]]);
  return all.map((d) => {
    const up = d.parent!;
    const key = childKey(keys.get(up)!, d.data);
    keys.set(d, key);
    const parentPath = paths.get(up) ?? "";
    const path = parentPath && childPath(parentPath, d.data);
    paths.set(d, path);
    let top = 0;
    let tone = 1;
    if (d.depth >= level) {
      let c = d;
      while (c.depth > level) c = c.parent!;
      top = (c.parent!.data.children ?? []).indexOf(c.data);
      tone = d.depth - level + 1;
    }
    const sib = up.children!;
    const rank = sib.length > 1 ? sib.indexOf(d) / (sib.length - 1) : 0;
    return { node: d.data, path, key, parent: up === h ? -1 : index.get(up)!, depth: d.depth, top, tone, rank, ...geometry(d) } as S;
  });
}

export function sunburst(root: SliceNode, radius: number, metric: Metric, base = root.name): Arc[] {
  const h = build(root, metric);
  partition<SliceNode>().size([Math.PI * 2, 1])(h);
  const levels = Math.max(1, h.height);
  const rc = radius * CENTER;
  const ring = (radius - rc) / levels;
  return shapes<Arc>(h, base, metric, (d) => {
    const p = d as unknown as { x0: number; x1: number };
    return { a0: p.x0, a1: p.x1, r0: rc + (d.depth - 1) * ring, r1: rc + d.depth * ring };
  });
}

export function treemap(root: SliceNode, w: number, h: number, metric: Metric, base = root.name): Rect[] {
  const hr = build(root, metric).sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
  const framed = (d: HierarchyNode<SliceNode>) => {
    const r = d as unknown as { x0: number; y0: number; x1: number; y1: number };
    return framedBox(d.depth, !!d.children, r.x1 - r.x0, r.y1 - r.y0);
  };
  d3treemap<SliceNode>()
    .tile(treemapSquarify.ratio(1.15))
    .size([w, h])
    .paddingInner((d) => (d.depth === 0 ? 3 : 1))
    .paddingOuter((d) => (framed(d) ? 2 : 0))
    .paddingTop((d) => (framed(d) ? HEADER : 0))
    .round(true)(hr);
  return shapes<Rect>(hr, base, metric, (d) => {
    const r = d as unknown as { x0: number; y0: number; x1: number; y1: number };
    return { x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 };
  });
}

/** Columns by depth, left to right; each column splits its parent's height. */
export function icicle(root: SliceNode, w: number, h: number, metric: Metric, base = root.name): Rect[] {
  const hr = build(root, metric).sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
  partition<SliceNode>().size([h, 1]).padding(0)(hr);
  const col = w / Math.max(1, hr.height);
  return shapes<Rect>(hr, base, metric, (d) => {
    const p = d as unknown as { x0: number; x1: number };
    return { x0: (d.depth - 1) * col, x1: d.depth * col, y0: p.x0, y1: p.x1 };
  });
}

const mixN = (a: number, b: number, t: number) => a + (b - a) * t;
export const lerpArc = (a: Arc, b: Arc, t: number): Arc => ({ ...b, a0: mixN(a.a0, b.a0, t), a1: mixN(a.a1, b.a1, t), r0: mixN(a.r0, b.r0, t), r1: mixN(a.r1, b.r1, t) });
export const lerpRect = (a: Rect, b: Rect, t: number): Rect => ({ ...b, x0: mixN(a.x0, b.x0, t), x1: mixN(a.x1, b.x1, t), y0: mixN(a.y0, b.y0, t), y1: mixN(a.y1, b.y1, t) });

/** Where an arc goes when the chart zooms into f: f becomes the middle, its slice the whole circle. */
export function arcZoom(f: Arc, rc: number): (s: Arc) => Arc {
  const span = f.a1 - f.a0 || 1;
  const ang = (a: number) => Math.max(0, Math.min(1, (a - f.a0) / span)) * Math.PI * 2;
  const rad = (r: number) => (r >= f.r1 ? rc + (r - f.r1) : r <= f.r0 ? 0 : (rc * (r - f.r0)) / (f.r1 - f.r0));
  return (s) => ({ ...s, a0: ang(s.a0), a1: ang(s.a1), r0: rad(s.r0), r1: rad(s.r1) });
}

/** Where a treemap rect goes when the chart zooms into f: f's contents fill the canvas. */
export function treemapZoom(f: Rect, w: number, h: number): (s: Rect) => Rect {
  const top = f.y0 + (isFramed(f) ? HEADER : 0);
  const sx = w / Math.max(1, f.x1 - f.x0);
  const sy = h / Math.max(1, f.y1 - top);
  return (s) => ({ ...s, x0: (s.x0 - f.x0) * sx, x1: (s.x1 - f.x0) * sx, y0: (s.y0 - top) * sy, y1: (s.y1 - top) * sy });
}

/** Where an icicle rect goes when the chart zooms into f: f's column leaves, its children take the first. */
export function icicleZoom(f: Rect, h: number): (s: Rect) => Rect {
  const sy = h / Math.max(1, f.y1 - f.y0);
  return (s) => ({ ...s, x0: s.x0 - f.x1, x1: s.x1 - f.x1, y0: (s.y0 - f.y0) * sy, y1: (s.y1 - f.y0) * sy });
}

/** The arc under (x, y), relative to the centre; "center" for the middle disc. */
export function hitArc(arcs: Arc[], x: number, y: number, radius: number): Arc | "center" | null {
  const r = Math.hypot(x, y);
  if (r < radius * CENTER) return "center";
  let a = Math.atan2(y, x) + Math.PI / 2;
  if (a < 0) a += Math.PI * 2;
  for (const s of arcs) if (r >= s.r0 && r < s.r1 && a >= s.a0 && a < s.a1) return s;
  return null;
}

/** The deepest rectangle under (x, y). */
export function hitRect(rects: Rect[], x: number, y: number): Rect | null {
  for (let i = rects.length - 1; i >= 0; i--) {
    const r = rects[i];
    if (x >= r.x0 && x < r.x1 && y >= r.y0 && y < r.y1) return r;
  }
  return null;
}

/** The shape and everything above and below it, for hover emphasis. */
export function lineage(shapes: Shape[], i: number): Set<number> {
  const out = new Set<number>();
  for (let p = i; p >= 0; p = shapes[p].parent) out.add(p);
  const prefix = shapes[i].key + "/";
  shapes.forEach((s, j) => s.key.startsWith(prefix) && out.add(j));
  return out;
}
