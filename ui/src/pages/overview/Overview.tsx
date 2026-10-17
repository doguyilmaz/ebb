import { Link } from "@tanstack/react-router";
import { ArrowRight, CircleAlert, Lightbulb } from "lucide-react";
import { useFindings, usePresets, useSlice, useStatus, useVolume } from "#/api/hooks.ts";
import type { PresetView } from "#/api/types.gen.ts";
import { bytes, percent } from "#/lib/format.ts";
import { useSheet } from "#/lib/sheet.ts";
import { CountUp } from "#/ui/CountUp.tsx";
import { Safety } from "#/ui/Safety.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { Info } from "#/ui/Tip.tsx";
import { AgeChart } from "./AgeChart.tsx";
import { CategoryBars } from "./CategoryBars.tsx";
import { VolumeBar } from "./VolumeBar.tsx";

export default function Overview() {
  const status = useStatus();
  const volume = useVolume();
  const findings = useFindings();
  const slice = useSlice(undefined, 1, 60);
  if (volume.error) return <Problem error={volume.error} />;
  if (!volume.data || !status.data || !findings.data) return <Loading label="Reading the last scan…" />;
  const v = volume.data;
  const apps = slice.data?.root?.children?.find((c) => c.name === "Applications")?.alloc ?? 0;
  const all = findings.data.findings ?? [];
  const notes = findings.data.notes ?? [];
  const reclaimable = findings.data.reclaimable;
  const used = (v.capacity - v.free) / Math.max(1, v.capacity);
  const low = v.free / Math.max(1, v.capacity) < 0.1;
  const actionable = all.filter((f) => f.actions?.length && f.actions[0].kind !== "manual").length;

  return (
    <>
      <section className="hero">
        <div>
          <span className="hero-label">
            Free on this Mac <Info term="segment.free" />
            {low && (
              <span className="chip-low">
                <CircleAlert size={13} aria-hidden /> Low on space
              </span>
            )}
          </span>
          <div className="hero-figure num">
            <CountUp id="free" value={v.free} format={bytes} />
          </div>
          <p className="hero-sub">
            of {bytes(v.capacity)} · {percent(used)} used
            {v.purgeable > 0 && (
              <>
                {" "}
                · {bytes(v.purgeable)} more macOS frees on its own <Info term="purgeable" />
              </>
            )}
          </p>
        </div>
        <div className="hero-cta">
          <Link to="/findings" className="btn primary big">
            Review {bytes(reclaimable)} you can clean <ArrowRight size={16} aria-hidden />
          </Link>
          <span className="faint small-text">{actionable} findings, each with why it is large and what removing it costs</span>
        </div>
      </section>

      <VolumeBar volume={v} apps={apps} reclaimable={reclaimable} />

      <div className="grid-2">
        <CategoryBars findings={all} total={reclaimable} />
        <AgeChart />
      </div>

      <Presets />

      {all.length > 0 && (
        <section className="card">
          <div className="card-head">
            <h2>Largest findings</h2>
            <span className="grow" />
            <Link to="/findings" className="btn ghost small">
              All findings <ArrowRight size={13} aria-hidden />
            </Link>
          </div>
          <ul className="toplist">
            {all.slice(0, 6).map((f) => (
              <li key={f.id}>
                <Link to="/findings" hash={f.id} className="toprow">
                  <span className="num size">{bytes(f.bytes)}</span>
                  <span className="ellipsis">{f.title}</span>
                  <Safety level={f.safety} />
                  <span className="muted why">{f.why}</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {notes.length > 0 && (
        <section className="stack">
          <h2>Good to know</h2>
          <ul className="notes">
            {notes.map((n) => (
              <li key={n}>
                <Lightbulb size={15} aria-hidden />
                {n}
              </li>
            ))}
          </ul>
        </section>
      )}
    </>
  );
}

/** Ready-made cleanups; each opens a plan to review, nothing runs on the click itself. */
export function Presets({ compact }: { compact?: boolean }) {
  const presets = usePresets();
  const open = useSheet((s) => s.open);
  const useful = (presets.data ?? []).filter((p: PresetView) => p.bytes > 0);
  if (useful.length === 0) return null;
  if (compact)
    return (
      <div className="presets">
        {useful.map((p) => (
          <button key={p.id} className="btn" onClick={() => open({ preset: p.id })}>
            {p.title}
            <span className="num faint">{bytes(p.bytes)}</span>
          </button>
        ))}
      </div>
    );
  return (
    <section className="stack" data-tour="presets">
      <div className="card-head">
        <h2>Quick cleanups</h2>
        <Info term="presets" />
      </div>
      <div className="presets-grid">
        {useful.map((p) => (
          <button key={p.id} className="preset" onClick={() => open({ preset: p.id })}>
            <span className="preset-size num">{bytes(p.bytes)}</span>
            <span className="preset-title">{p.title}</span>
            <Safety level={p.maxSafety} />
          </button>
        ))}
      </div>
    </section>
  );
}
