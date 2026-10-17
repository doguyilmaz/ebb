import { ChevronDown, ChevronRight, EyeOff, FolderSearch, Hand, Square, SquareCheck, Terminal, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useFindings, useIgnore, useReveal, useSetNote } from "#/api/hooks.ts";
import type { Action, FindingView } from "#/api/types.gen.ts";
import { bytes } from "#/lib/format.ts";
import { categoryTitle } from "#/lib/glossary.ts";
import { useSelection } from "#/lib/selection.ts";
import { useSheet } from "#/lib/sheet.ts";
import { Presets } from "#/pages/overview/Overview.tsx";
import { CountUp } from "#/ui/CountUp.tsx";
import { PageHead } from "#/ui/PageHead.tsx";
import { Safety } from "#/ui/Safety.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { Info } from "#/ui/Tip.tsx";

const levels = ["regenerable", "caution", "user-data"] as const;

export default function Findings() {
  const [showIgnored, setShowIgnored] = useState(false);
  const findings = useFindings(showIgnored);
  useEffect(() => {
    const id = decodeURIComponent(location.hash.slice(1));
    if (id) (document.getElementById(`f-${id}`) ?? document.getElementById(id))?.scrollIntoView({ block: "start" });
  }, [findings.data]);
  if (findings.error) return <Problem error={findings.error} />;
  if (!findings.data) return <Loading label="Reading the findings…" />;
  const list = findings.data.findings ?? [];
  const groups = new Map<string, FindingView[]>();
  for (const f of list) groups.set(f.category, [...(groups.get(f.category) ?? []), f]);
  const size = (cat: string) => groups.get(cat)!.reduce((n, f) => n + f.bytes, 0);
  const order = [...groups.keys()].sort((a, b) => size(b) - size(a));
  return (
    <>
      <PageHead title="Findings" sub="Everything that takes real space, with why it is there and what removing it costs. Tick what you want and review the plan; nothing runs before you approve it.">
        <label className="row muted small-text">
          <input type="checkbox" checked={showIgnored} onChange={(e) => setShowIgnored(e.target.checked)} /> Show ignored
          <Info term="ignore" />
        </label>
      </PageHead>
      <section className="card summary">
        <div className="stack" style={{ gap: 6 }}>
          <span className="row faint small-text" style={{ gap: 4 }}>
            Can be cleaned <Info term="reclaimable" />
          </span>
          <CountUp id="findings-total" value={findings.data.reclaimable} format={bytes} className="summary-figure num" />
        </div>
        <div className="safety-key" aria-label="Safety levels">
          {levels.map((l) => (
            <span key={l}>
              <Safety level={l} />
              <Info term={`safety.${l}`} />
            </span>
          ))}
        </div>
      </section>
      <Presets compact />
      {order.map((cat) => (
        <section key={cat} id={`cat-${cat}`} className="category" aria-labelledby={`h-${cat}`}>
          <div className="category-head">
            <h2 id={`h-${cat}`}>{categoryTitle(cat)}</h2>
            <Info term={`category.${cat}` as "category.logs"} />
            <span className="size num">{bytes(size(cat))}</span>
          </div>
          <div className="stack findings-list" style={{ gap: 10 }}>
            {groups.get(cat)!.map((f) => (
              <FindingCard key={f.id} f={f} />
            ))}
          </div>
        </section>
      ))}
      {list.length === 0 && <p className="empty">Nothing stands out on this disk.</p>}
    </>
  );
}

const ActionIcon = ({ a }: { a: Action }) => (a.kind === "manual" ? <Hand size={13} aria-hidden /> : a.kind === "command" ? <Terminal size={13} aria-hidden /> : <Trash2 size={13} aria-hidden />);
const actionText = (a: Action) =>
  a.kind === "command" ? `Runs ${(a.argv ?? []).join(" ")}` : a.kind === "trash" ? "Moves to the Trash" : a.kind === "delete" ? "Deletes it" : a.kind === "empty-trash" ? "Empties the Trash" : "You do it yourself";

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
    <article id={`f-${f.id}`} className={`finding ${f.ignored ? "ignored" : ""}`} data-selected={selected || undefined} data-tour="finding">
      <header className="finding-head">
        {actionable ? (
          <button className="icon-btn" aria-pressed={selected} aria-label={selected ? "Remove from cleanup" : "Add to cleanup"} data-tip={selected ? "In the cleanup tray" : "Add to the cleanup tray"} data-tour="select" onClick={() => toggle(f.id)}>
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
      {f.actions?.[0] && (
        <p className="row muted small-text" style={{ gap: 6 }}>
          <ActionIcon a={f.actions[0]} />
          <span className="ellipsis">{actionText(f.actions[0])}</span>
          <Info term={`action.${f.actions[0].kind}` as "action.trash"} />
        </p>
      )}
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
        <button className="btn ghost small" data-term="ignore" onClick={() => ignore.mutate({ id: f.id, undo: f.ignored })}>
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
      <button className="btn ghost small add-note" data-term="note" onClick={() => setEditing(true)}>
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
