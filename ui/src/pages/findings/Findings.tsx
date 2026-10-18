import { ChevronRight, EyeOff, FolderSearch, Hand, Terminal, Trash2, Wand2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useFindings, useIgnore, useReveal, useSetNote } from "#/api/hooks.ts";
import type { Action, FindingView } from "#/api/types.gen.ts";
import { bytes } from "#/lib/format.ts";
import { categoryTitle } from "#/lib/glossary.ts";
import { useSelection } from "#/lib/selection.ts";
import { useSheet } from "#/lib/sheet.ts";
import { AskPanel } from "#/app/AskPanel.tsx";
import { Presets } from "#/pages/overview/Overview.tsx";
import { Check } from "#/ui/Check.tsx";
import { CountUp } from "#/ui/CountUp.tsx";
import { PageHead } from "#/ui/PageHead.tsx";
import { Safety } from "#/ui/Safety.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { Info } from "#/ui/Tip.tsx";
import { grouped, groupState, matches, pickable, type Filter } from "./triage.ts";

const filters: { id: Filter; label: string }[] = [
  { id: "all", label: "All" },
  { id: "regenerable", label: "Rebuilds itself" },
  { id: "caution", label: "Check first" },
  { id: "user-data", label: "Your data" },
];

export default function Findings() {
  const [showIgnored, setShowIgnored] = useState(false);
  const [filter, setFilter] = useState<Filter>("all");
  const [open, setOpen] = useState<string | null>(null);
  const findings = useFindings(showIgnored);
  const picked = useSelection((s) => s.findings);
  const setMany = useSelection((s) => s.setFindings);

  useEffect(() => {
    const id = decodeURIComponent(location.hash.slice(1));
    if (!id) return;
    const row = document.getElementById(`f-${id}`);
    if (row) {
      setOpen(id);
      row.scrollIntoView({ block: "center" });
    } else document.getElementById(id)?.scrollIntoView({ block: "start" });
  }, [findings.data]);

  if (findings.error) return <Problem error={findings.error} />;
  if (!findings.data) return <Loading label="Reading the findings…" />;
  const all = findings.data.findings ?? [];
  const shown = all.filter((f) => matches(f, filter));
  const easy = all.filter((f) => pickable(f) && f.safety === "regenerable" && !f.ignored);
  const easyBytes = easy.reduce((n, f) => n + f.bytes, 0);
  const allEasyPicked = easy.length > 0 && easy.every((f) => picked.has(f.id));
  const count = (id: Filter) => all.filter((f) => matches(f, id)).length;

  return (
    <>
      <PageHead title="Findings" sub="What takes space and whether it is safe to remove. Pick what you want gone, then review the plan; nothing runs before you approve it.">
        <label className="row muted small-text">
          <input type="checkbox" checked={showIgnored} onChange={(e) => setShowIgnored(e.target.checked)} /> Show ignored
          <Info term="ignore" />
        </label>
      </PageHead>

      <section className="card triage-head">
        <div className="stack" style={{ gap: 4 }}>
          <span className="row faint small-text" style={{ gap: 4 }}>
            Can be cleaned <Info term="reclaimable" />
          </span>
          <CountUp id="findings-total" value={findings.data.reclaimable} format={bytes} className="summary-figure num" />
        </div>
        {easy.length > 0 && (
          <div className="easy">
            <p>
              <strong>{bytes(easyBytes)}</strong> rebuilds itself, so it is the safe place to start.
            </p>
            <button className="btn" onClick={() => setMany(easy.map((f) => f.id), !allEasyPicked)} data-term="safety.regenerable">
              <Wand2 size={14} aria-hidden /> {allEasyPicked ? "Unselect them" : `Select all ${easy.length}`}
            </button>
          </div>
        )}
      </section>

      <Presets compact />

      <div className="triage-bar">
        <div className="seg" role="tablist" aria-label="Show">
          {filters.map((x) => (
            <button key={x.id} role="tab" aria-selected={filter === x.id} aria-pressed={filter === x.id} onClick={() => setFilter(x.id)} data-term={x.id === "all" ? undefined : `safety.${x.id}`}>
              {x.label}
              <span className="seg-count num">{count(x.id)}</span>
            </button>
          ))}
        </div>
        <span className="faint small-text">Click a row for the details</span>
      </div>

      {grouped(shown).map(([cat, list]) => (
        <Group key={cat} cat={cat} list={list} open={open} onOpen={setOpen} />
      ))}
      {shown.length === 0 && <p className="empty card">{all.length === 0 ? "Nothing stands out on this disk." : "Nothing here with this filter."}</p>}
    </>
  );
}

function Group({ cat, list, open, onOpen }: { cat: string; list: FindingView[]; open: string | null; onOpen: (id: string | null) => void }) {
  const [collapsed, setCollapsed] = useState(false);
  const picked = useSelection((s) => s.findings);
  const setMany = useSelection((s) => s.setFindings);
  const state = groupState(list, picked);
  const ids = list.filter(pickable).map((f) => f.id);
  const size = list.reduce((n, f) => n + f.bytes, 0);
  return (
    <section id={`cat-${cat}`} className="group" aria-labelledby={`h-${cat}`}>
      <header className="group-head">
        {ids.length > 0 ? (
          <Check state={state} label={`Select all ${categoryTitle(cat)}`} onClick={() => setMany(ids, state !== "all")} />
        ) : (
          <span className="check-space" />
        )}
        <button className="group-toggle" aria-expanded={!collapsed} onClick={() => setCollapsed(!collapsed)}>
          <ChevronRight size={15} aria-hidden className="chev" />
          <h2 id={`h-${cat}`}>{categoryTitle(cat)}</h2>
          <span className="faint small-text num">{list.length}</span>
        </button>
        <Info term={`category.${cat}` as "category.logs"} />
        <span className="grow" />
        <span className="num group-size">{bytes(size)}</span>
      </header>
      {!collapsed && (
        <ul className="rows">
          {list.map((f) => (
            <Row key={f.id} f={f} open={open === f.id} onToggle={() => onOpen(open === f.id ? null : f.id)} />
          ))}
        </ul>
      )}
    </section>
  );
}


const ActionIcon = ({ a }: { a: Action }) => (a.kind === "manual" ? <Hand size={13} aria-hidden /> : a.kind === "command" ? <Terminal size={13} aria-hidden /> : <Trash2 size={13} aria-hidden />);
const actionText = (a: Action) =>
  a.kind === "command" ? `Runs ${(a.argv ?? []).join(" ")}` : a.kind === "trash" ? "Moves it to the Trash" : a.kind === "delete" ? "Deletes it" : a.kind === "empty-trash" ? "Empties the Trash" : "You do it yourself";

function Row({ f, open, onToggle }: { f: FindingView; open: boolean; onToggle: () => void }) {
  const openSheet = useSheet((s) => s.open);
  const selected = useSelection((s) => s.findings.has(f.id));
  const toggle = useSelection((s) => s.toggleFinding);
  const can = pickable(f);
  // Details stay mounted after the first open so closing can animate too.
  const [seen, setSeen] = useState(open);
  useEffect(() => {
    if (open) setSeen(true);
  }, [open]);
  return (
    <li id={`f-${f.id}`} className="frow" data-open={open || undefined} data-selected={selected || undefined} data-ignored={f.ignored || undefined} data-tour="finding">
      <div className="frow-main">
        {can ? <Check state={selected ? "all" : "none"} label={`Select ${f.title}`} onClick={() => toggle(f.id)} tour="select" /> : <span className="check-space" data-tip="ebb shows what to do; you do this one yourself." />}
        <button className="frow-open" aria-expanded={open} aria-controls={`d-${f.id}`} onClick={onToggle}>
          <span className="num frow-size">{bytes(f.bytes)}</span>
          <span className="frow-text">
            <span className="frow-title">{f.title}</span>
            <span className="frow-why">{f.why}</span>
          </span>
        </button>
        <Safety level={f.safety} />
        {can ? (
          <button className="btn small" onClick={() => openSheet({ findingIds: [f.id] })} data-tip="Shows the exact plan for just this one">
            Clean
          </button>
        ) : (
          <span className="frow-manual faint small-text">Manual</span>
        )}
      </div>
      <div className="frow-details" id={`d-${f.id}`} role="region" aria-label={`${f.title} details`}>
        <div>{seen && <Details f={f} />}</div>
      </div>
    </li>
  );
}

function Details({ f }: { f: FindingView }) {
  const ignore = useIgnore();
  const reveal = useReveal();
  const targets = f.targets ?? [];
  const a = f.actions?.[0];
  return (
    <div className="details">
      <dl className="details-grid">
        <dt>Why it is large</dt>
        <dd>{f.why}</dd>
        <dt>If you remove it</dt>
        <dd>{f.impact}</dd>
        {a && (
          <>
            <dt>How</dt>
            <dd className="row" style={{ gap: 6 }}>
              <ActionIcon a={a} />
              {actionText(a)}
              <Info term={`action.${a.kind}` as "action.trash"} />
            </dd>
          </>
        )}
        {a?.kind === "manual" && a.hint && (
          <>
            <dt>What to do</dt>
            <dd>
              <code className="hint">{a.hint}</code>
            </dd>
          </>
        )}
        {f.alloc > f.bytes && (
          <>
            <dt>
              Shared <Info term="shared" />
            </dt>
            <dd className="muted">
              Takes {bytes(f.alloc)} on disk; {bytes(f.alloc - f.bytes)} of that is shared with copies elsewhere and would stay.
            </dd>
          </>
        )}
        {f.evidence?.length ? (
          <>
            <dt>Seen</dt>
            <dd className="muted">{f.evidence.slice(0, 6).join(" · ")}</dd>
          </>
        ) : null}
        {f.notes && (
          <>
            <dt>Note</dt>
            <dd className="muted">{f.notes}</dd>
          </>
        )}
      </dl>
      {targets.length > 0 && (
        <ul className="targets">
          {targets.slice(0, 50).map((t) => (
            <li key={t.path}>
              <span className="num">{bytes(t.bytes)}</span>
              <span className="mono ellipsis" title={t.path}>{t.path}</span>
              <button className="icon-btn" aria-label={`Show ${t.path} in Finder`} data-tip="Show in Finder" onClick={() => reveal.mutate(t.path)}>
                <FolderSearch size={14} />
              </button>
            </li>
          ))}
          {targets.length > 50 && <li className="faint small-text">and {targets.length - 50} more</li>}
        </ul>
      )}
      <AskPanel target={{ findingId: f.id }} />
      <div className="row details-foot">
        <Note id={f.id} initial={f.userNote ?? ""} />
        <span className="grow" />
        <button className="btn ghost small" data-term="ignore" onClick={() => ignore.mutate({ id: f.id, undo: f.ignored })}>
          <EyeOff size={14} aria-hidden /> {f.ignored ? "Stop ignoring" : "Ignore"}
        </button>
      </div>
    </div>
  );
}

function Note({ id, initial }: { id: string; initial: string }) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(initial);
  const save = useSetNote();
  useEffect(() => setText(initial), [initial]);
  if (!editing && !initial)
    return (
      <button className="btn ghost small" data-term="note" onClick={() => setEditing(true)}>
        Add a note
      </button>
    );
  return editing ? (
    <textarea
      className="input note-input"
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
