import { useEffect, useMemo, useRef, useState } from "react";
import type { SliceNode } from "#/api/types.gen.ts";
import { bytes, percent } from "#/lib/format.ts";
import { fill, readPalette, type Palette } from "./colors.ts";
import { CENTER, hitArc, hitRect, icicle, sunburst, treemap, type Arc, type Metric, type Rect } from "./layout.ts";

export type ChartView = "sunburst" | "treemap" | "icicle";

export interface Focus {
  node: SliceNode;
  path: string;
}

interface Props {
  view: ChartView;
  root: SliceNode;
  base: string;
  metric: Metric;
  onHover: (f: Focus | null) => void;
  onOpen: (path: string) => void;
  onSelect: (f: Focus) => void;
  onUp: () => void;
}

/** Draws the current folder on a canvas; SVG would not survive thousands of shapes. */
export function Chart({ view, root, base, metric, onHover, onOpen, onSelect, onUp }: Props) {
  const wrap = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const overlay = useRef<HTMLCanvasElement>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [hover, setHover] = useState<Arc | Rect | null>(null);
  const [scheme, setScheme] = useState(0);

  useEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setSize({ w: Math.floor(e.contentRect.width), h: Math.floor(e.contentRect.height) }));
    ro.observe(el);
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const bump = () => setScheme((n) => n + 1);
    mq.addEventListener("change", bump);
    return () => {
      ro.disconnect();
      mq.removeEventListener("change", bump);
    };
  }, []);

  const radius = Math.max(10, Math.min(size.w, size.h) / 2 - 8);
  const shapes = useMemo(() => {
    if (!size.w || !size.h) return [];
    if (view === "sunburst") return sunburst(root, radius, metric, base);
    if (view === "treemap") return treemap(root, size.w, size.h, metric, base);
    return icicle(root, size.w, size.h, metric, base);
  }, [view, root, metric, base, size, radius]);

  // The shapes are drawn once per layout; hovering only repaints the thin
  // overlay above them, so pointer movement stays cheap on large canvases.
  useEffect(() => {
    const c = canvas.current;
    if (!c || !size.w) return;
    const ctx = setup(c, size.w, size.h);
    const p = readPalette();
    if (view === "sunburst") drawSunburst(ctx, shapes as Arc[], size.w / 2, size.h / 2, p);
    else drawRects(ctx, shapes as Rect[], p, view === "treemap");
  }, [shapes, size, view, scheme]);

  useEffect(() => {
    const c = overlay.current;
    if (!c || !size.w) return;
    const ctx = setup(c, size.w, size.h);
    const p = readPalette();
    if (view === "sunburst") drawCenter(ctx, size.w / 2, size.h / 2, radius, root, hover as Arc | null, p, metric);
    else if (hover) outlineRect(ctx, hover as Rect, p);
  }, [hover, size, view, radius, root, metric, scheme]);

  const locate = (e: React.PointerEvent | React.MouseEvent) => {
    const b = overlay.current!.getBoundingClientRect();
    const x = e.clientX - b.left;
    const y = e.clientY - b.top;
    if (view === "sunburst") return hitArc(shapes as Arc[], x - size.w / 2, y - size.h / 2, radius);
    return hitRect(shapes as Rect[], x, y);
  };

  return (
    <div className="chart" ref={wrap}>
      <canvas ref={canvas} style={{ width: size.w, height: size.h }} aria-hidden />
      <canvas
        ref={overlay}
        className="overlay"
        style={{ width: size.w, height: size.h }}
        role="img"
        aria-label={`${view} of ${root.name}: ${(root.children ?? []).slice(0, 8).map((c) => `${c.name} ${bytes(c[metric])}`).join(", ")}`}
        onPointerMove={(e) => {
          const s = locate(e);
          const next = s && s !== "center" ? s : null;
          if (next !== hover) {
            setHover(next);
            onHover(next ? { node: next.node, path: next.path } : null);
          }
        }}
        onPointerLeave={() => {
          setHover(null);
          onHover(null);
        }}
        onClick={(e) => {
          const s = locate(e);
          if (s === "center") return onUp();
          if (!s || !s.path) return;
          if (s.node.kind === 0) onOpen(s.path);
          else onSelect({ node: s.node, path: s.path });
        }}
      />
    </div>
  );
}

function setup(c: HTMLCanvasElement, w: number, h: number) {
  const dpr = window.devicePixelRatio || 1;
  c.width = w * dpr;
  c.height = h * dpr;
  const ctx = c.getContext("2d")!;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, w, h);
  return ctx;
}

function arcPath(ctx: CanvasRenderingContext2D, a: Arc, cx: number, cy: number) {
  ctx.beginPath();
  ctx.arc(cx, cy, a.r1, a.a0 - Math.PI / 2, a.a1 - Math.PI / 2);
  ctx.arc(cx, cy, a.r0, a.a1 - Math.PI / 2, a.a0 - Math.PI / 2, true);
  ctx.closePath();
}

function drawSunburst(ctx: CanvasRenderingContext2D, arcs: Arc[], cx: number, cy: number, p: Palette) {
  ctx.lineWidth = 1.5;
  ctx.strokeStyle = p.surface;
  for (const a of arcs) {
    if (a.a1 - a.a0 < 0.002) continue;
    arcPath(ctx, a, cx, cy);
    ctx.fillStyle = fill(p, a.top, a.depth, a.node.kind, a.node.other);
    ctx.fill();
    ctx.stroke();
  }
}

/** The middle disc names what the pointer is on, or the folder itself. */
function drawCenter(ctx: CanvasRenderingContext2D, cx: number, cy: number, radius: number, root: SliceNode, hover: Arc | null, p: Palette, metric: Metric) {
  if (hover) {
    arcPath(ctx, hover, cx, cy);
    ctx.lineWidth = 2.5;
    ctx.strokeStyle = p.text;
    ctx.stroke();
  }
  const rc = radius * CENTER;
  ctx.beginPath();
  ctx.arc(cx, cy, rc - 2, 0, Math.PI * 2);
  ctx.fillStyle = p.surface;
  ctx.fill();
  const n = hover?.node ?? root;
  ctx.textAlign = "center";
  ctx.fillStyle = p.text;
  ctx.font = "600 17px Plex, system-ui, sans-serif";
  ctx.fillText(bytes(n[metric]), cx, cy + 2);
  ctx.font = "12.5px Plex, system-ui, sans-serif";
  ctx.fillStyle = p.text2;
  ctx.fillText(fitText(ctx, n.name, rc * 1.7), cx, cy - 18);
  if (hover) ctx.fillText(percent(n[metric] / Math.max(1, root[metric])), cx, cy + 20);
}

function drawRects(ctx: CanvasRenderingContext2D, rects: Rect[], p: Palette, nested: boolean) {
  ctx.textBaseline = "top";
  for (const r of rects) {
    const w = r.x1 - r.x0;
    const h = r.y1 - r.y0;
    if (w < 1 || h < 1) continue;
    ctx.fillStyle = fill(p, r.top, nested ? r.depth : 1, r.node.kind, r.node.other);
    ctx.beginPath();
    ctx.roundRect(r.x0, r.y0, Math.max(0, w - (nested ? 0 : 1.5)), Math.max(0, h - (nested ? 0 : 1.5)), nested && r.depth === 1 ? 5 : 3);
    ctx.fill();
    const label = nested ? r.depth === 1 || !r.node.children?.length : true;
    if (label && w > 56 && h > 17) {
      ctx.fillStyle = r.depth === 1 ? "#fff" : p.text;
      ctx.font = `${r.depth === 1 ? 600 : 400} 12px Plex, system-ui, sans-serif`;
      ctx.fillText(fitText(ctx, r.node.name, w - 10), r.x0 + 6, r.y0 + 3);
      if (h > 34 && (!nested || !r.node.children?.length)) {
        ctx.fillStyle = r.depth === 1 ? "rgba(255,255,255,.85)" : p.text2;
        ctx.font = "11px Plex, system-ui, sans-serif";
        ctx.fillText(bytes(r.node.alloc), r.x0 + 6, r.y0 + 18);
      }
    }
  }
}

function outlineRect(ctx: CanvasRenderingContext2D, r: Rect, p: Palette) {
  ctx.lineWidth = 2;
  ctx.strokeStyle = p.text;
  ctx.strokeRect(r.x0 + 1, r.y0 + 1, r.x1 - r.x0 - 2, r.y1 - r.y0 - 2);
}

function fitText(ctx: CanvasRenderingContext2D, text: string, max: number): string {
  if (ctx.measureText(text).width <= max) return text;
  let s = text;
  while (s.length > 1 && ctx.measureText(s + "…").width > max) s = s.slice(0, -1);
  return s + "…";
}
