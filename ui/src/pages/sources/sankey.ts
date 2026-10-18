import type { Flows, Link as FlowLink, Node as FlowNode } from "#/api/types.gen.ts";

export interface Box extends FlowNode {
  x: number;
  y: number;
  h: number;
  labelY: number; // the label's middle: the node's, moved just enough to clear its neighbours
}

export interface Band extends FlowLink {
  path: string;
  y0: number; // where it leaves its source
  y1: number; // where it reaches its target
  w: number;
}

export interface Layout {
  boxes: Box[];
  bands: Band[];
}

const NODE_W = 10;
const GAP = 10;

/**
 * Lays out three columns (place → owner → kind): nodes stacked biggest first,
 * every column the same scale so a byte is as tall everywhere, and bands
 * leaving and entering each node in the order of the node at their other end,
 * which keeps crossings few.
 */
export function layout(f: Flows, width: number, height: number): Layout {
  const cols = [0, 1, 2].map((c) => (f.nodes ?? []).filter((n) => n.column === c));
  const sum = (ns: FlowNode[]) => ns.reduce((s, n) => s + n.bytes, 0);
  const k = Math.min(...cols.filter((c) => c.length).map((c) => (height - GAP * (c.length - 1)) / Math.max(1, sum(c))));
  const xs = [0, (width - NODE_W) / 2, width - NODE_W];
  const byId = new Map<string, Box>();
  cols.forEach((c, i) => {
    const used = sum(c) * k + GAP * (c.length - 1);
    let y = (height - used) / 2;
    for (const n of c) {
      const h = Math.max(1, n.bytes * k);
      byId.set(n.id, { ...n, x: xs[i], y, h, labelY: y + h / 2 });
      y += h + GAP;
    }
  });
  const links = (f.links ?? []).filter((l) => byId.has(l.from) && byId.has(l.to));
  const out = new Map<string, number>();
  const into = new Map<string, number>();
  const bands: Band[] = [];
  const order = (key: (l: FlowLink) => number) => [...links].sort((a, b) => key(a) - key(b));
  // Leave each source ordered by target position, enter each target ordered by source position.
  for (const l of order((l) => byId.get(l.to)!.y)) {
    const s = byId.get(l.from)!;
    const w = Math.max(0.5, l.bytes * k);
    const y0 = s.y + (out.get(l.from) ?? 0);
    out.set(l.from, (out.get(l.from) ?? 0) + w);
    bands.push({ ...l, y0, y1: 0, w, path: "" });
  }
  const byKey = new Map(bands.map((b) => [b.from + ">" + b.to, b]));
  for (const l of order((l) => byId.get(l.from)!.y)) {
    const t = byId.get(l.to)!;
    const b = byKey.get(l.from + ">" + l.to)!;
    b.y1 = t.y + (into.get(l.to) ?? 0);
    into.set(l.to, (into.get(l.to) ?? 0) + b.w);
    const x0 = byId.get(l.from)!.x + NODE_W;
    const x1 = t.x;
    const m = (x0 + x1) / 2;
    b.path = `M${x0},${b.y0}C${m},${b.y0} ${m},${b.y1} ${x1},${b.y1}L${x1},${b.y1 + b.w}C${m},${b.y1 + b.w} ${m},${b.y0 + b.w} ${x0},${b.y0 + b.w}Z`;
  }
  for (let c = 0; c < 3; c++) spread([...byId.values()].filter((b) => b.column === c), height);
  return { boxes: [...byId.values()], bands };
}

export const LABEL_GAP = 15;

/** Pushes labels apart top-down, then back up from the bottom if the last one fell off. */
function spread(col: Box[], height: number) {
  col.sort((a, b) => a.labelY - b.labelY);
  for (let i = 1; i < col.length; i++) col[i].labelY = Math.max(col[i].labelY, col[i - 1].labelY + LABEL_GAP);
  const over = (col.at(-1)?.labelY ?? 0) - (height - LABEL_GAP / 2);
  if (over > 0) {
    col[col.length - 1].labelY -= over;
    for (let i = col.length - 2; i >= 0; i--) col[i].labelY = Math.min(col[i].labelY, col[i + 1].labelY - LABEL_GAP);
  }
}

export const NODE_WIDTH = NODE_W;

/** The disk bar's colours: apps, what can be cleaned, everything else. */
export function kindColor(kind: string): string {
  if (kind === "apps") return "var(--s1)";
  if (kind === "yours" || kind === "unclaimed" || kind === "user-data") return "var(--s4)";
  return "var(--s3)";
}
