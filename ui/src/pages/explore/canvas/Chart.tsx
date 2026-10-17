import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { SliceNode } from "#/api/types.gen.ts";
import { ago, bytes, count, percent } from "#/lib/format.ts";
import { easeInOut, easeOut, reducedMotion } from "#/lib/motion.ts";
import { useSchemeVersion } from "#/lib/theme.ts";
import { readPalette, type ColorMode, type Palette } from "./colors.ts";
import { colorer, drawArcLabels, drawArcs, drawCenter, drawIcicleLabels, drawRects, drawTreemapLabels, type Emphasis } from "./draw.ts";
import {
  arcZoom,
  CENTER,
  hitArc,
  hitRect,
  icicle,
  icicleZoom,
  lerpArc,
  lerpRect,
  lineage,
  sunburst,
  treemap,
  treemapZoom,
  type Arc,
  type Metric,
  type Rect,
  type Shape,
} from "./layout.ts";
import { frame, plan, type Frame, type Transition } from "./morph.ts";

export type ChartView = "sunburst" | "treemap" | "icicle";
type Any = Arc | Rect;

interface Props {
  view: ChartView;
  root: SliceNode;
  base: string;
  metric: Metric;
  /** Name of the folder above, when the middle can go back up. */
  upLabel?: string;
  disk?: number;
  color: ColorMode;
  /** Scan time and age bucket bounds in days, for colouring by age. */
  now: number;
  cuts: number[];
  hoverKey: string | null;
  stale: boolean;
  onHover: (s: Shape | null) => void;
  onOpen: (path: string) => void;
  onPick: (s: Shape) => void;
  onUp: () => void;
  onPrefetch: (path: string) => void;
}

const isArc = (s: Any): s is Arc => "a0" in s;
const lerpAny = (a: Any, b: Any, t: number): Any => (isArc(b) ? lerpArc(a as Arc, b, t) : lerpRect(a as Rect, b, t));
const within = (p: string, root: string) => p.startsWith(root.endsWith("/") ? root : root + "/");
const LABEL_FADE = 180;

interface Anim {
  tr: Transition<Any>;
  start: number;
  ms: number;
  ease: (t: number) => number;
  lerp: (a: Any, b: Any, t: number) => Any;
}

/**
 * Draws the current folder on a canvas (SVG would not survive thousands of
 * shapes). Every change of folder, metric or view morphs from what is on
 * screen to the new layout; at rest it adds labels and hover emphasis.
 */
export function Chart(props: Props) {
  const { view, root, base, metric, hoverKey, stale } = props;
  const wrap = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const tipEl = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [tip, setTip] = useState<Shape | null>(null);
  const [center, setCenter] = useState(false);
  const scheme = useSchemeVersion();
  const cb = useRef(props);
  cb.current = props;

  useEffect(() => {
    const ro = new ResizeObserver(([e]) => setSize({ w: Math.floor(e.contentRect.width), h: Math.floor(e.contentRect.height) }));
    ro.observe(wrap.current!);
    return () => ro.disconnect();
  }, []);

  const radius = Math.max(10, Math.min(size.w, size.h) / 2 - 10);
  const rc = radius * CENTER;
  const target = useMemo<Any[]>(() => {
    if (!size.w || !size.h) return [];
    if (view === "sunburst") return sunburst(root, radius, metric, base);
    if (view === "treemap") return treemap(root, size.w, size.h, metric, base);
    return icicle(root, size.w, size.h, metric, base);
  }, [view, root, metric, base, size, radius]);

  const st = useRef({
    shown: [] as Any[],
    base: "",
    view: "" as ChartView | "",
    size: "",
    anim: null as Anim | null,
    rest: 0,
    raf: 0,
    palette: null as Palette | null,
  });

  const emphasis = (): Emphasis => {
    const shapes = st.current.shown;
    const i = hoverKey ? shapes.findIndex((s) => s.key === hoverKey) : -1;
    if (i < 0) return { keys: null, hover: null };
    return { keys: new Set([...lineage(shapes, i)].map((j) => shapes[j].key)), hover: hoverKey };
  };

  const paint = (now: number) => {
    const s = st.current;
    const c = canvas.current;
    if (!c || !size.w) return;
    const ctx = c.getContext("2d")!;
    const dpr = window.devicePixelRatio || 1;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, size.w, size.h);
    const p = (s.palette ??= readPalette());
    let frames: Frame<Any>[];
    let moving = false;
    if (s.anim) {
      const t = Math.min(1, (now - s.anim.start) / s.anim.ms);
      frames = frame(s.anim.tr, s.anim.ease(t), s.anim.lerp);
      if (t >= 1) {
        s.anim = null;
        s.rest = now;
      } else moving = true;
    } else frames = s.shown.map((shape) => ({ shape, alpha: 1 }));
    const em = moving ? { keys: null, hover: null } : emphasis();
    const cx = size.w / 2;
    const cy = size.h / 2;
    const color = colorer(p, props.color, view, props.now, props.cuts);
    const arcs = frames.filter((f) => isArc(f.shape)) as Frame<Arc>[];
    const rects = frames.filter((f) => !isArc(f.shape)) as Frame<Rect>[];
    if (rects.length) drawRects(ctx, rects, p, em, color, size.w, size.h, view === "icicle" ? 1 : 0);
    if (arcs.length) drawArcs(ctx, arcs, cx, cy, p, em, color);
    const labels = moving ? 0 : Math.min(1, (now - s.rest) / LABEL_FADE);
    if (labels > 0) {
      if (view === "sunburst") drawArcLabels(ctx, s.shown as Arc[], cx, cy, em, color, labels);
      else if (view === "treemap") drawTreemapLabels(ctx, s.shown as Rect[], em, color, labels);
      else drawIcicleLabels(ctx, s.shown as Rect[], em, color, labels);
    }
    if (view === "sunburst") {
      const n = root[metric];
      drawCenter(ctx, cx, cy, rc, p, root.name, bytes(n), `${count(root.files)} files`, center && props.upLabel ? props.upLabel : null);
    }
    if (moving || labels < 1) s.raf = requestAnimationFrame(paint);
  };

  const kick = () => {
    cancelAnimationFrame(st.current.raf);
    st.current.raf = requestAnimationFrame(paint);
  };

  useLayoutEffect(() => {
    const c = canvas.current;
    if (!c || !size.w) return;
    const dpr = window.devicePixelRatio || 1;
    c.width = size.w * dpr;
    c.height = size.h * dpr;
  }, [size]);

  useEffect(() => {
    st.current.palette = null;
    kick();
  }, [scheme]);

  useEffect(() => {
    const s = st.current;
    const sizeKey = `${size.w}x${size.h}`;
    const now = performance.now();
    if (!target.length) return;
    const first = s.shown.length === 0 && !s.anim;
    if (s.size !== sizeKey && !first) {
      s.shown = target;
      s.anim = null;
      s.size = sizeKey;
      s.base = base;
      s.view = view;
      s.rest = now - LABEL_FADE;
      kick();
      return;
    }
    const reduce = reducedMotion();
    const prev: Any[] = s.anim ? s.anim.tr.moves.filter((m) => m[3] === 1).map((m) => s.anim!.lerp(m[0], m[1], s.anim!.ease(Math.min(1, (now - s.anim!.start) / s.anim!.ms)))) : s.shown;
    const same = s.view === view;
    const id = (x: Any) => x;
    let ghostIn: (x: Any) => Any = id;
    let ghostOut: (x: Any) => Any = id;
    let ms = 380;
    let ease = easeInOut;
    if (first) {
      ghostIn = (x) => (isArc(x) ? { ...x, a0: 0, a1: 0 } : view === "icicle" ? { ...x, x1: x.x0 } : shrink(x));
      ms = 700;
      ease = easeOut;
    } else if (same && base !== s.base) {
      const zoomIn = within(base, s.base);
      const f = zoomIn ? prev.find((x) => x.key === base) : within(s.base, base) ? target.find((x) => x.key === s.base) : undefined;
      if (f) {
        const cam = isArc(f) ? (arcZoom(f, rc) as (x: Any) => Any) : view === "treemap" ? (treemapZoom(f, size.w, size.h) as (x: Any) => Any) : (icicleZoom(f, size.h) as (x: Any) => Any);
        if (zoomIn) {
          ghostOut = cam;
          ghostIn = (x) => (isArc(x) ? { ...x, r1: x.r0 } : view === "icicle" ? { ...x, x1: x.x0 } : x);
        } else ghostIn = cam;
        ms = 520;
      }
    } else if (!same) ms = 280;
    const tr = plan(same ? prev : [], target, reduce ? id : ghostIn, reduce ? id : ghostOut);
    if (!same) for (const x of prev) tr.moves.push([x, x, 1, 0]);
    s.anim = { tr, start: now, ms: reduce ? 180 : ms, ease, lerp: reduce ? (_a, b) => b : lerpAny };
    s.shown = target;
    s.size = sizeKey;
    s.base = base;
    s.view = view;
    kick();
  }, [target]);

  useEffect(kick, [hoverKey, center, props.upLabel, props.color, props.now, props.cuts]);
  useEffect(() => () => cancelAnimationFrame(st.current.raf), []);

  const locate = (e: React.PointerEvent | React.MouseEvent): Any | "center" | null => {
    if (st.current.anim) return null;
    const b = canvas.current!.getBoundingClientRect();
    const x = e.clientX - b.left;
    const y = e.clientY - b.top;
    const shapes = st.current.shown;
    if (view === "sunburst") return hitArc(shapes as Arc[], x - size.w / 2, y - size.h / 2, radius);
    return hitRect(shapes as Rect[], x, y);
  };

  const prefetch = useRef<ReturnType<typeof setTimeout>>(undefined);
  const move = (e: React.PointerEvent) => {
    const hit = locate(e);
    const s = hit && hit !== "center" ? hit : null;
    setCenter(hit === "center");
    if ((s?.key ?? null) !== hoverKey) {
      cb.current.onHover(s);
      setTip(s);
      clearTimeout(prefetch.current);
      if (s?.path && s.node.kind === 0) prefetch.current = setTimeout(() => cb.current.onPrefetch(s.path), 120);
    }
    const el = tipEl.current;
    if (el && s) {
      const b = wrap.current!.getBoundingClientRect();
      const x = e.clientX - b.left;
      const y = e.clientY - b.top;
      const tw = el.offsetWidth;
      const th = el.offsetHeight;
      const left = x + 16 + tw > size.w ? x - 16 - tw : x + 16;
      const top = y + 16 + th > size.h ? y - 12 - th : y + 16;
      el.style.transform = `translate(${Math.max(4, left)}px, ${Math.max(4, top)}px)`;
    }
  };

  const total = Math.max(1, root[metric]);
  const label = `${view} of ${root.name}: ${(root.children ?? []).slice(0, 8).map((c) => `${c.name} ${bytes(c[metric])}`).join(", ")}`;
  return (
    <div className={`chart${stale ? " stale" : ""}`} ref={wrap} data-tour="chart">
      <canvas
        ref={canvas}
        style={{ width: size.w, height: size.h, cursor: center && props.upLabel ? "pointer" : tip ? "pointer" : "default" }}
        role="img"
        aria-label={label}
        onPointerMove={move}
        onPointerLeave={() => {
          setCenter(false);
          setTip(null);
          clearTimeout(prefetch.current);
          cb.current.onHover(null);
        }}
        onClick={(e) => {
          const hit = locate(e);
          if (hit === "center") return props.upLabel && props.onUp();
          if (!hit?.path) return;
          if (hit.node.kind === 0) props.onOpen(hit.path);
          else props.onPick(hit);
        }}
      />
      <div ref={tipEl} className={`chart-tip${tip ? " on" : ""}`} aria-hidden>
        {tip && (
          <>
            <strong className="num">{bytes(tip.node[metric])}</strong>
            <span className="ellipsis">{tip.node.other ? `${tip.node.other} smaller items` : tip.node.name}</span>
            {tip.depth > 1 && <span className="faint ellipsis chart-tip-trail">{trail(tip.key, base)}</span>}
            <span className="faint num">
              {percent(tip.node[metric] / total)} of this folder
              {props.disk ? ` · ${percent(tip.node[metric] / props.disk)} of the disk` : ""}
            </span>
            {tip.node.files > 1 && <span className="faint num">{count(tip.node.files)} files{tip.node.modified ? ` · changed ${ago(tip.node.modified)}` : ""}</span>}
            <span className="chart-tip-hint">{tip.node.kind === 0 && tip.path ? "Click to open" : tip.path ? "Click for details" : "Open the List to see these"}</span>
          </>
        )}
      </div>
    </div>
  );
}

/** Where a shape sits below the folder on screen, as "a › b › c". */
function trail(key: string, base: string): string {
  const rel = key.slice(base.length).replace(/^\//, "").split("\u0000")[0];
  return rel.split("/").slice(0, -1).join(" › ") || "";
}

function shrink(r: Rect): Rect {
  const cx = (r.x0 + r.x1) / 2;
  const cy = (r.y0 + r.y1) / 2;
  const k = 0.9;
  return { ...r, x0: cx + (r.x0 - cx) * k, x1: cx + (r.x1 - cx) * k, y0: cy + (r.y0 - cy) * k, y1: cy + (r.y1 - cy) * k };
}
