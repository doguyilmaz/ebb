import { Link } from "@tanstack/react-router";
import { ArrowLeft, File, Folder, FolderOpen, FolderSearch, ListPlus, ListX } from "lucide-react";
import { useEffect, useState } from "react";
import { useAges, useFindings, useNotes, useReveal, useSetNote } from "#/api/hooks.ts";
import type { SliceNode } from "#/api/types.gen.ts";
import { ageColor, oldShare } from "#/lib/ages.ts";
import { ago, bytes, count, percent } from "#/lib/format.ts";
import { useSelection } from "#/lib/selection.ts";
import { Safety } from "#/ui/Safety.tsx";
import { Info } from "#/ui/Tip.tsx";
import { childKey, childPath, colorLevel, type Metric } from "./canvas/layout.ts";

export interface Focus {
  node: SliceNode;
  path: string;
}

interface Props {
  root: SliceNode;
  base: string;
  metric: Metric;
  picked: Focus | null;
  hoverKey: string | null;
  onHover: (key: string | null) => void;
  onOpen: (path: string) => void;
  onPick: (f: Focus | null) => void;
  onPrefetch: (path: string) => void;
}

const within = (p: string, root: string) => p === root || p.startsWith(root.replace(/\/$/, "") + "/");
const slotColor = (n: SliceNode, i: number) => (n.kind >= 3 || n.other || i >= 8 ? "var(--other)" : `var(--s${i + 1})`);

/** The folder on screen, what is biggest inside it, and the item picked from it. */
export function SidePanel({ root, base, metric, picked, hoverKey, onHover, onOpen, onPick, onPrefetch }: Props) {
  const focus = picked ?? { node: root, path: base };
  return (
    <aside className="sidepanel" aria-label="Details">
      {picked && (
        <button className="btn ghost small back" onClick={() => onPick(null)}>
          <ArrowLeft size={14} aria-hidden /> {root.name}
        </button>
      )}
      <Head focus={focus} metric={metric} />
      {!picked && <Contents root={root} base={base} metric={metric} hoverKey={hoverKey} onHover={onHover} onOpen={onOpen} onPick={onPick} onPrefetch={onPrefetch} />}
      {focus.node.kind === 0 && <AgeBar path={focus.path} />}
      <Related path={focus.path} />
      <Actions focus={focus} isFolder={!picked} onOpen={onOpen} />
      {focus.path && <NoteEditor path={focus.path} />}
    </aside>
  );
}

function Head({ focus, metric }: { focus: Focus; metric: Metric }) {
  const n = focus.node;
  const shared = n.alloc - n.unique;
  const Icon = n.kind === 0 ? Folder : File;
  return (
    <div className="stack" style={{ gap: 10 }}>
      <div className="side-head">
        <Icon size={18} aria-hidden className="faint" />
        <h2 className="ellipsis" title={n.name}>{n.other ? `${n.other} smaller items` : n.name}</h2>
      </div>
      <div className="side-size">
        <span className="num">{bytes(n[metric])}</span>
        <Info term={metric === "unique" ? "unique" : "allocated"} />
      </div>
      {focus.path && focus.path !== n.name && <code className="faint wrap side-path">{focus.path}</code>}
      <dl className="facts">
        {shared > 0 && (
          <>
            <dt>Shared <Info term="shared" /></dt>
            <dd className="num">{bytes(shared)}</dd>
          </>
        )}
        <dt>Files <Info term="files" /></dt>
        <dd className="num">{count(n.files)}</dd>
        {n.modified ? (
          <>
            <dt>Changed <Info term="changed" /></dt>
            <dd>{ago(n.modified)}</dd>
          </>
        ) : null}
      </dl>
    </div>
  );
}

/** The items the chart colours, biggest first; the chart's legend and its table view. */
function Contents({ root, base, metric, hoverKey, onHover, onOpen, onPick, onPrefetch }: Omit<Props, "picked">) {
  const chain = colorLevel(root, metric);
  const holder = chain.at(-1) ?? root;
  const at = chain.reduce((b, n) => childPath(b, n) || b, base);
  const kids = holder.children ?? [];
  if (kids.length === 0) return null;
  const max = Math.max(1, ...kids.map((k) => k[metric]));
  const total = Math.max(1, holder[metric]);
  return (
    <section className="stack" style={{ gap: 4 }}>
      <h3 className="panel-label">{chain.length ? `Biggest inside ${holder.name}` : "Biggest inside"}</h3>
      <ol className="contents">
        {kids.slice(0, 40).map((k, i) => {
          const key = childKey(at, k);
          const path = childPath(at, k);
          const color = slotColor(k, i);
          return (
            <li key={key}>
              <button
                data-hover={hoverKey === key || undefined}
                data-term={k.other ? "other" : k.kind === 4 ? "small" : undefined}
                onPointerEnter={() => {
                  onHover(key);
                  if (path && k.kind === 0) onPrefetch(path);
                }}
                onPointerLeave={() => onHover(null)}
                onFocus={() => onHover(key)}
                onBlur={() => onHover(null)}
                onClick={() => path && (k.kind === 0 ? onOpen(path) : onPick({ node: k, path }))}
                aria-disabled={!path || undefined}
              >
                <i className="swatch" style={{ background: color }} />
                <span className="ellipsis">{k.other ? `${k.other} smaller items` : k.name}</span>
                <span className="num">{bytes(k[metric])}</span>
                <span className="faint num pct">{percent(k[metric] / total)}</span>
                <span className="share" aria-hidden>
                  <i style={{ width: `${(k[metric] / max) * 100}%`, background: color }} />
                </span>
              </button>
            </li>
          );
        })}
      </ol>
    </section>
  );
}

function AgeBar({ path }: { path: string }) {
  const ages = useAges(path || undefined);
  const b = ages.data?.buckets ?? [];
  const { old, total } = oldShare(b);
  if (!total) return null;
  return (
    <section className="stack" style={{ gap: 7 }}>
      <div className="row" style={{ gap: 6 }}>
        <h3 className="panel-label">How old</h3>
        <Info term="ages" />
      </div>
      <div className="agebar" role="img" aria-label={b.map((x) => `${x.label} ${bytes(x.bytes)}`).join(", ")}>
        {b.map((x, i) =>
          x.bytes > 0 ? <i key={x.label} style={{ flexGrow: x.bytes, background: ageColor(i, b.length) }} data-tip={`Changed ${x.label.toLowerCase()}: ${bytes(x.bytes)} (${percent(x.bytes / total)})`} /> : null,
        )}
      </div>
      <div className="agebar-scale faint">
        <span>Changed recently</span>
        <span>Long ago</span>
      </div>
      {old > 0 && <p className="muted small-text">{percent(old / total)} has not changed in over a year.</p>}
    </section>
  );
}

function Related({ path }: { path: string }) {
  const findings = useFindings();
  if (!path) return null;
  const related = (findings.data?.findings ?? []).filter((f) => (f.targets ?? []).some((t) => within(t.path, path) || within(path, t.path)));
  if (related.length === 0) return null;
  return (
    <section className="stack" style={{ gap: 6 }}>
      <h3 className="panel-label">Findings here</h3>
      {related.slice(0, 5).map((f) => (
        <Link key={f.id} to="/findings" hash={f.id} className="related">
          <span>{f.title}</span>
          <span className="row" style={{ gap: 8 }}>
            <span className="num muted">{bytes(f.bytes)}</span>
            <Safety level={f.safety} />
          </span>
        </Link>
      ))}
    </section>
  );
}

function Actions({ focus, isFolder, onOpen }: { focus: Focus; isFolder: boolean; onOpen: (p: string) => void }) {
  const reveal = useReveal();
  const selected = useSelection((s) => s.paths.has(focus.path));
  const toggle = useSelection((s) => s.togglePath);
  if (!focus.path) return null;
  return (
    <div className="actions">
      {focus.node.kind === 0 && !isFolder && (
        <button className="btn small" onClick={() => onOpen(focus.path)}>
          <FolderOpen size={14} aria-hidden /> Open
        </button>
      )}
      <button className="btn small" onClick={() => reveal.mutate(focus.path)}>
        <FolderSearch size={14} aria-hidden /> Show in Finder
      </button>
      {!isFolder && (
        <button className="btn small" onClick={() => toggle(focus.path)} data-tip="Adds it to the cleanup tray at the bottom. You review everything before it runs.">
          {selected ? <ListX size={14} aria-hidden /> : <ListPlus size={14} aria-hidden />}
          {selected ? "Remove from cleanup" : "Add to cleanup"}
        </button>
      )}
    </div>
  );
}

function NoteEditor({ path }: { path: string }) {
  const notes = useNotes();
  const initial = notes.data?.[`path:${path}`] ?? "";
  const [text, setText] = useState(initial);
  const save = useSetNote();
  useEffect(() => setText(initial), [initial, path]);
  return (
    <div className="stack" style={{ gap: 6 }}>
      <div className="row" style={{ gap: 6 }}>
        <label className="panel-label" htmlFor="note">Your note</label>
        <Info term="note" />
      </div>
      <textarea
        id="note"
        className="input"
        rows={2}
        value={text}
        placeholder="Why this is here, when to clean it…"
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text !== initial && save.mutate({ key: `path:${path}`, text })}
      />
    </div>
  );
}
