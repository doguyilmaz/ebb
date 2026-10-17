import { hierarchy, partition, treemap as d3treemap, treemapSquarify, type HierarchyNode } from "d3-hierarchy";
import type { SliceNode } from "#/api/types.gen.ts";

export type Metric = "alloc" | "unique";

interface Shape {
  node: SliceNode;
  /** Filesystem path, or "" for merged "other" and small-files nodes. */
  path: string;
  depth: number;
  /** Index of the top-level child this shape belongs to; colours follow it. */
  top: number;
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
export const CENTER = 0.22;

const navigable = (n: SliceNode) => (n.kind === 0 || n.kind === 1 || n.kind === 2) && !n.other;

function build(root: SliceNode, metric: Metric) {
  return hierarchy(root, (d) => d.children ?? undefined).sum((d) => (d.children?.length ? 0 : Math.max(0, d[metric])));
}

function shapeInfo(d: HierarchyNode<SliceNode>, base: string): Shape {
  const chain = d.ancestors().reverse().slice(1);
  const path = chain.every((a) => navigable(a.data)) ? [base, ...chain.map((a) => a.data.name)].join("/").replace(/\/+/g, "/") : "";
  const topNode = chain[0];
  const top = topNode ? (topNode.parent?.children ?? []).indexOf(topNode) : -1;
  return { node: d.data, path, depth: d.depth, top };
}

export function sunburst(root: SliceNode, radius: number, metric: Metric, base = root.name): Arc[] {
  const h = build(root, metric);
  partition<SliceNode>().size([Math.PI * 2, 1])(h);
  const levels = Math.max(1, h.height);
  const rc = radius * CENTER;
  const ring = (radius - rc) / levels;
  return h
    .descendants()
    .slice(1)
    .map((d) => {
      const p = d as unknown as { x0: number; x1: number };
      return { ...shapeInfo(d, base), a0: p.x0, a1: p.x1, r0: rc + (d.depth - 1) * ring, r1: rc + d.depth * ring };
    });
}

export function treemap(root: SliceNode, w: number, h: number, metric: Metric, base = root.name): Rect[] {
  const hr = build(root, metric).sort((a, b) => (b.value ?? 0) - (a.value ?? 0));
  d3treemap<SliceNode>()
    .tile(treemapSquarify)
    .size([w, h])
    .paddingInner(2)
    .paddingTop((d) => (d.depth >= 1 && d.children ? 18 : 0))
    .round(true)(hr);
  return hr
    .descendants()
    .slice(1)
    .map((d) => {
      const r = d as unknown as { x0: number; y0: number; x1: number; y1: number };
      return { ...shapeInfo(d, base), x0: r.x0, y0: r.y0, x1: r.x1, y1: r.y1 };
    });
}

/** Columns by depth, left to right; each column splits its parent's height. */
export function icicle(root: SliceNode, w: number, h: number, metric: Metric, base = root.name): Rect[] {
  const hr = build(root, metric);
  partition<SliceNode>().size([h, 1])(hr);
  const col = w / Math.max(1, hr.height);
  return hr
    .descendants()
    .slice(1)
    .map((d) => {
      const p = d as unknown as { x0: number; x1: number };
      return { ...shapeInfo(d, base), x0: (d.depth - 1) * col, x1: d.depth * col, y0: p.x0, y1: p.x1 };
    });
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
