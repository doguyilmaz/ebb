import { Link } from "@tanstack/react-router";
import { useFindings, useMakePlan, usePresets, useSlice, useStatus, useVolume } from "#/api/hooks.ts";
import type { PresetView } from "#/api/types.gen.ts";
import { CleanupSheet } from "#/app/CleanupSheet.tsx";
import { ago, bytes, count } from "#/lib/format.ts";
import { Safety } from "#/ui/Safety.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { useState } from "react";
import { VolumeBar } from "./VolumeBar.tsx";

export default function Overview() {
  const status = useStatus();
  const volume = useVolume();
  const findings = useFindings();
  const slice = useSlice(undefined, 1, 60);
  if (volume.error) return <Problem error={volume.error} />;
  if (!volume.data || !status.data || !findings.data) return <Loading />;
  const apps = slice.data?.root?.children?.find((c) => c.name === "Applications")?.alloc ?? 0;
  const last = status.data.last;
  const all = findings.data.findings ?? [];
  const notes = findings.data.notes ?? [];
  const top = all.slice(0, 6);
  return (
    <div className="overview page-narrow">
      <VolumeBar volume={volume.data} apps={apps} reclaimable={findings.data.reclaimable} />

      <section className="win">
        <div>
          <h1>{bytes(findings.data.reclaimable)} can be cleaned</h1>
          <p className="muted">
            From {all.filter((f) => f.actions?.length).length} findings. Each one says why it is large and what
            removing it costs.
          </p>
        </div>
        <Presets />
      </section>

      <section className="stack">
        <div className="row">
          <h2>Largest findings</h2>
          <Link to="/findings" className="btn ghost small">
            All findings
          </Link>
        </div>
        <ul className="toplist">
          {top.map((f) => (
            <li key={f.id}>
              <Link to="/findings" hash={f.id} className="toprow">
                <span className="num size">{bytes(f.bytes)}</span>
                <span className="title">{f.title}</span>
                <Safety level={f.safety} />
                <span className="muted why">{f.why}</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>

      {notes.length > 0 && (
        <section className="notes">
          <h2>Good to know</h2>
          <ul>
            {notes.map((n) => (
              <li key={n} className="muted">
                {n}
              </li>
            ))}
          </ul>
        </section>
      )}

      {last && (
        <p className="faint num">
          Scanned {last.root} {ago(last.time)}: {count(last.files)} files in {count(last.dirs)} folders, {(last.elapsed / 1e9).toFixed(0)} s.
        </p>
      )}
    </div>
  );
}

function Presets() {
  const presets = usePresets();
  const make = useMakePlan();
  const [planId, setPlanId] = useState<string | null>(null);
  const useful = (presets.data ?? []).filter((p: PresetView) => p.bytes > 0);
  if (useful.length === 0) return null;
  return (
    <div className="presets">
      {useful.map((p) => (
        <button
          key={p.id}
          className="btn"
          disabled={make.isPending}
          onClick={() => make.mutate({ preset: p.id, source: "ui" }, { onSuccess: (plan) => setPlanId(plan.id) })}
        >
          {p.title}
          <span className="num muted">{bytes(p.bytes)}</span>
        </button>
      ))}
      {planId && <CleanupSheet planId={planId} plan={make.data} onClose={() => setPlanId(null)} />}
    </div>
  );
}
