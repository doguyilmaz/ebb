import { useNavigate } from "@tanstack/react-router";
import { useLayoutEffect, useRef, useState } from "react";
import type { Flows } from "#/api/types.gen.ts";
import { bytes, percent } from "#/lib/format.ts";
import { categoryTitle } from "#/lib/glossary.ts";
import { kindColor, layout, NODE_WIDTH } from "./sankey.ts";

const HEIGHT = 520;
const LEFT = 150;
const RIGHT = 190;

const kindLabels: Record<string, string> = { yours: "Your files", apps: "Apps", unclaimed: "Not claimed by a finding" };
export const kindLabel = (k: string) => kindLabels[k] ?? categoryTitle(k);
const isFindingKind = (k: string) => !(k in kindLabels) && k !== "user-data";

/** Place → owner → kind as bands whose width is the space; hover a node to follow its flows. */
export function FlowChart({ flows }: { flows: Flows }) {
  const wrap = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(900);
  const [hover, setHover] = useState<string | null>(null);
  const navigate = useNavigate();
  useLayoutEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setWidth(Math.max(560, Math.round(e.contentRect.width))));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  const inner = width - LEFT - RIGHT;
  const { boxes, bands } = layout(flows, inner, HEIGHT);
  const kindOf = new Map<string, string>();
  for (const b of bands) if (b.to.startsWith("k:")) kindOf.set(b.from + ">" + b.to, b.to.slice(2));
  const touches = (from: string, to: string) => hover === null || hover === from || hover === to;
  const label = (id: string, raw: string) => (id.startsWith("k:") ? kindLabel(raw) : raw);
  const total = Math.max(1, flows.total);

  return (
    <div className="flowchart" ref={wrap}>
      <svg width={width} height={HEIGHT} role="img" aria-label="Where the scanned space comes from: places, owners and kinds">
        <g transform={`translate(${LEFT},0)`}>
          {bands.map((b) => {
            const kind = kindOf.get(b.from + ">" + b.to);
            const on = touches(b.from, b.to);
            return (
              <path
                key={b.from + ">" + b.to}
                d={b.path}
                className="band"
                data-on={hover !== null && on ? true : undefined}
                style={{ fill: kind ? kindColor(kind) : "var(--text-3)", opacity: on ? (hover ? 0.55 : kind ? 0.38 : 0.2) : 0.06 }}
                data-tip={`${label(b.from, boxes.find((n) => n.id === b.from)?.label ?? "")} → ${label(b.to, boxes.find((n) => n.id === b.to)?.label ?? "")}`}
                data-value={bytes(b.bytes)}
              />
            );
          })}
          {boxes.map((n) => {
            const kind = n.id.startsWith("k:") ? n.label : null;
            const clickable = n.column === 0 || (kind !== null && isFindingKind(kind));
            const text = label(n.id, n.label);
            const tx = n.column === 0 ? n.x - 8 : n.x + NODE_WIDTH + 8;
            return (
              <g
                key={n.id}
                className="flow-node"
                data-clickable={clickable || undefined}
                onPointerEnter={() => setHover(n.id)}
                onPointerLeave={() => setHover(null)}
                onClick={() => {
                  if (n.column === 0 && n.path) navigate({ to: "/explore", search: { path: n.path } });
                  else if (kind && isFindingKind(kind)) navigate({ to: "/findings", hash: `cat-${kind}` });
                }}
                data-tip={text}
                data-value={`${bytes(n.bytes)} · ${percent(n.bytes / total)}`}
              >
                <rect x={n.x} y={n.y} width={NODE_WIDTH} height={n.h} rx={2} style={{ fill: kind ? kindColor(kind) : "var(--text-2)" }} />
                <text x={tx} y={n.labelY} dy="0.35em" textAnchor={n.column === 0 ? "end" : "start"}>
                  {text}
                  <tspan className="faint"> {bytes(n.bytes)}</tspan>
                </text>
              </g>
            );
          })}
        </g>
      </svg>
      <div className="flow-legend small-text">
        <span><i style={{ background: "var(--s3)" }} /> Can be cleaned</span>
        <span><i style={{ background: "var(--s1)" }} /> Apps</span>
        <span><i style={{ background: "var(--s4)" }} /> Everything else</span>
        <span className="faint">Click a folder to explore it, a kind to see its findings</span>
      </div>
    </div>
  );
}
