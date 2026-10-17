import { ChevronDown, ChevronRight, EyeOff, FolderSearch, Square, SquareCheck } from "lucide-react";
import { useEffect, useState } from "react";
import { useFindings, useIgnore, useReveal, useSetNote } from "#/api/hooks.ts";
import type { FindingView } from "#/api/types.gen.ts";
import { bytes } from "#/lib/format.ts";
import { useSelection } from "#/lib/selection.ts";
import { useSheet } from "#/lib/sheet.ts";
import { Presets } from "#/pages/overview/Overview.tsx";
import { Safety } from "#/ui/Safety.tsx";
import { Loading, Problem } from "#/ui/states.tsx";

const categories: Record<string, string> = {
  "package-cache": "Package caches",
  "build-artifact": "Build output",
  simulator: "Simulators",
  sdk: "SDKs, emulators and Xcode",
  "stale-project": "Untouched projects",
  "app-cache": "App caches",
  logs: "Logs",
  installer: "Installers",
  "large-file": "Large files",
  trash: "Trash",
  leftover: "Leftovers of removed apps",
  duplicate: "Duplicates",
  snapshot: "Time Machine",
  growth: "Recent growth",
  "user-data": "Your data",
};

export default function Findings() {
  const [showIgnored, setShowIgnored] = useState(false);
  const findings = useFindings(showIgnored);
  useEffect(() => {
    const id = decodeURIComponent(location.hash.slice(1));
    if (id) document.getElementById(`f-${id}`)?.scrollIntoView({ block: "start" });
  }, [findings.data]);
  if (findings.error) return <Problem error={findings.error} />;
  if (!findings.data) return <Loading />;
  const list = findings.data.findings ?? [];
  const groups = new Map<string, FindingView[]>();
  for (const f of list) groups.set(f.category, [...(groups.get(f.category) ?? []), f]);
  const order = [...groups.keys()].sort((a, b) => groups.get(b)!.reduce((n, f) => n + f.bytes, 0) - groups.get(a)!.reduce((n, f) => n + f.bytes, 0));
  return (
    <div className="findings page-narrow">
      <div className="row findings-head">
        <h1>{bytes(findings.data.reclaimable)} can be cleaned</h1>
        <label className="row muted" style={{ marginLeft: "auto", gap: 6 }}>
          <input type="checkbox" checked={showIgnored} onChange={(e) => setShowIgnored(e.target.checked)} /> Show ignored
        </label>
      </div>
      <Presets />
      {order.map((cat) => (
        <section key={cat} className="stack" aria-labelledby={`cat-${cat}`}>
          <h2 id={`cat-${cat}`}>
            {categories[cat] ?? cat} <span className="faint num">{bytes(groups.get(cat)!.reduce((n, f) => n + f.bytes, 0))}</span>
          </h2>
          {groups.get(cat)!.map((f) => (
            <FindingCard key={f.id} f={f} />
          ))}
        </section>
      ))}
      {list.length === 0 && <p className="empty">Nothing stands out on this disk.</p>}
    </div>
  );
}

function FindingCard({ f }: { f: FindingView }) {
  const [open, setOpen] = useState(false);
  const openSheet = useSheet((s) => s.open);
  const selected = useSelection((s) => s.findings.has(f.id));
  const toggle = useSelection((s) => s.toggleFinding);
  const ignore = useIgnore();
  const reveal = useReveal();
  const actionable = (f.actions?.length ?? 0) > 0 && f.actions![0].kind !== "manual";
  const targets = f.targets ?? [];
  return (
    <article id={`f-${f.id}`} className={`finding ${f.ignored ? "ignored" : ""}`}>
      <header className="finding-head">
        {actionable ? (
          <button className="icon-btn" aria-pressed={selected} aria-label={selected ? "Remove from cleanup" : "Add to cleanup"} onClick={() => toggle(f.id)}>
            {selected ? <SquareCheck size={18} /> : <Square size={18} />}
          </button>
        ) : (
          <span style={{ width: 26 }} />
        )}
        <span className="num size">{bytes(f.bytes)}</span>
        <h3>{f.title}</h3>
        <Safety level={f.safety} />
        {f.owner && <span className="faint">{f.owner}</span>}
        <span className="grow" />
        {actionable && (
          <button className="btn small" onClick={() => openSheet({ findingIds: [f.id] })}>
            Clean
          </button>
        )}
      </header>
      <p>{f.why}</p>
      <p className="muted">{f.impact}</p>
      {f.alloc > f.bytes && <p className="faint">Takes {bytes(f.alloc)} on disk; {bytes(f.alloc - f.bytes)} of that is shared with copies elsewhere and would stay.</p>}
      {f.evidence?.length ? (
        <ul className="evidence">
          {f.evidence.slice(0, open ? 50 : 3).map((e) => (
            <li key={e} className="muted">{e}</li>
          ))}
        </ul>
      ) : null}
      {f.notes && <p className="faint">{f.notes}</p>}
      {f.actions?.[0]?.kind === "manual" && f.actions[0].hint && <code className="hint">{f.actions[0].hint}</code>}
      <div className="row finding-foot">
        {targets.length > 0 && (
          <button className="btn ghost small" aria-expanded={open} onClick={() => setOpen(!open)}>
            {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />} {targets.length} {targets.length === 1 ? "location" : "locations"}
          </button>
        )}
        {targets[0] && (
          <button className="btn ghost small" onClick={() => reveal.mutate(targets[0].path)}>
            <FolderSearch size={14} aria-hidden /> Show in Finder
          </button>
        )}
        <button className="btn ghost small" onClick={() => ignore.mutate({ id: f.id, undo: f.ignored })}>
          <EyeOff size={14} aria-hidden /> {f.ignored ? "Stop ignoring" : "Ignore"}
        </button>
      </div>
      {open && (
        <ul className="targets">
          {targets.map((t) => (
            <li key={t.path}>
              <span className="num">{bytes(t.bytes)}</span>
              <span className="mono ellipsis" title={t.path}>{t.path}</span>
            </li>
          ))}
        </ul>
      )}
      <Note id={f.id} initial={f.userNote ?? ""} />
    </article>
  );
}

function Note({ id, initial }: { id: string; initial: string }) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(initial);
  const save = useSetNote();
  useEffect(() => setText(initial), [initial]);
  if (!editing && !initial)
    return (
      <button className="btn ghost small add-note" onClick={() => setEditing(true)}>
        Add a note
      </button>
    );
  return editing ? (
    <textarea
      className="input"
      rows={2}
      autoFocus
      value={text}
      placeholder="Why this stays, when to clean it…"
      onChange={(e) => setText(e.target.value)}
      onBlur={() => {
        setEditing(false);
        if (text !== initial) save.mutate({ key: `finding:${id}`, text });
      }}
    />
  ) : (
    <button className="note" onClick={() => setEditing(true)}>{initial}</button>
  );
}
