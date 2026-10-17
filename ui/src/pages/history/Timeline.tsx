import { useLayoutEffect, useRef, useState } from "react";
import type { Meta } from "#/api/types.gen.ts";
import { bytes } from "#/lib/format.ts";
import { firstTime } from "#/lib/theme.ts";
import { niceTicks, scale } from "./timeline.ts";

const H = 240;
const PAD = { top: 16, right: 18, bottom: 28, left: 64 };
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });

/** Used space per scan over time: one line, a crosshair that snaps to the nearest scan, click to compare. */
export function Timeline({ snaps, from, to, onPick }: { snaps: Meta[]; from?: string; to?: string; onPick: (id: string) => void }) {
  const box = useRef<HTMLDivElement>(null);
  const [w, setW] = useState(0);
  const [hover, setHover] = useState<number | null>(null);
  const [enter] = useState(() => firstTime("timeline"));
  useLayoutEffect(() => {
    const ro = new ResizeObserver(([e]) => setW(e.contentRect.width));
    ro.observe(box.current!);
    return () => ro.disconnect();
  }, []);
  const pts = [...snaps].sort((a, b) => a.time.localeCompare(b.time));
  const ticks = niceTicks(Math.min(...pts.map((p) => p.used)), Math.max(...pts.map((p) => p.used)), 4);
  const t0 = new Date(pts[0]?.time ?? 0).getTime();
  const t1 = new Date(pts.at(-1)?.time ?? 0).getTime();
  const x = scale([t0, t1 === t0 ? t0 + 1 : t1], [PAD.left, Math.max(PAD.left + 1, w - PAD.right)]);
  const y = scale([ticks[0], ticks.at(-1)!], [H - PAD.bottom, PAD.top]);
  const xy = pts.map((p) => [x(new Date(p.time).getTime()), y(p.used)] as const);
  const line = xy.map(([a, b], i) => `${i ? "L" : "M"}${a.toFixed(1)},${b.toFixed(1)}`).join("");
  const area = xy.length ? `${line}L${xy.at(-1)![0]},${H - PAD.bottom}L${xy[0][0]},${H - PAD.bottom}Z` : "";
  const near = (cx: number) => xy.reduce((best, p, i) => (Math.abs(p[0] - cx) < Math.abs(xy[best][0] - cx) ? i : best), 0);
  const h = hover === null ? null : pts[hover];
  const prev = hover !== null && hover > 0 ? pts[hover - 1] : null;
  return (
    <div className="timeline" ref={box}>
      {w > 0 && pts.length > 0 && (
        <svg
          width={w}
          height={H}
          role="img"
          aria-label={`Used space over ${pts.length} scans, from ${bytes(pts[0].used)} to ${bytes(pts.at(-1)!.used)}`}
          onPointerMove={(e) => setHover(near(e.clientX - e.currentTarget.getBoundingClientRect().left))}
          onPointerLeave={() => setHover(null)}
          onClick={() => hover !== null && onPick(pts[hover].id)}
        >
          {ticks.map((t) => (
            <g key={t}>
              <line x1={PAD.left} x2={w - PAD.right} y1={y(t)} y2={y(t)} className="tl-grid" />
              <text x={PAD.left - 10} y={y(t)} className="tl-tick" textAnchor="end" dominantBaseline="middle">
                {bytes(t)}
              </text>
            </g>
          ))}
          {[pts[0], pts.at(-1)!].filter((p, i, a) => i === 0 || p !== a[0]).map((p, i) => (
            <text key={p.id} x={x(new Date(p.time).getTime())} y={H - 8} className="tl-tick" textAnchor={i ? "end" : "start"}>
              {when(p.time)}
            </text>
          ))}
          <path d={area} className="tl-area" />
          <path d={line} className={`tl-line${enter ? " enter" : ""}`} pathLength={1} />
          {h && <line x1={xy[hover!][0]} x2={xy[hover!][0]} y1={PAD.top} y2={H - PAD.bottom} className="tl-cross" />}
          {xy.map(([a, b], i) => (
            <circle key={pts[i].id} cx={a} cy={b} r={pts[i].id === from || pts[i].id === to ? 6 : 4} className="tl-point" data-on={pts[i].id === from || pts[i].id === to || undefined} />
          ))}
        </svg>
      )}
      {h && (
        <div className="chart-tip on tl-tip" style={{ left: Math.min(w - 220, Math.max(0, xy[hover!][0] + 12)), top: Math.max(0, xy[hover!][1] - 70) }}>
          <strong className="num">{bytes(h.used)} used</strong>
          <span className="faint num">{bytes(h.free)} free</span>
          {prev && (
            <span className="faint num">
              {h.used >= prev.used ? "+" : "−"}
              {bytes(Math.abs(h.used - prev.used))} since the scan before
            </span>
          )}
          <span className="faint">{when(h.time)}{h.incremental ? " · update" : ""}</span>
          <span className="chart-tip-hint">Click to compare</span>
        </div>
      )}
    </div>
  );
}
