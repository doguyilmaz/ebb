import { Link } from "@tanstack/react-router";
import { FolderOpen, FolderSearch, ListPlus, ListX } from "lucide-react";
import { useEffect, useState } from "react";
import { useFindings, useNotes, useReveal, useSetNote } from "#/api/hooks.ts";
import { ago, bytes, count } from "#/lib/format.ts";
import { useSelection } from "#/lib/selection.ts";
import { Safety } from "#/ui/Safety.tsx";
import type { Focus } from "./canvas/Chart.tsx";

const within = (p: string, root: string) => p === root || p.startsWith(root.replace(/\/$/, "") + "/");

/** What the pointer is on, what ebb knows about it, and what can be done with it. */
export function SidePanel({ focus, onOpen, isRoot }: { focus: Focus; onOpen: (p: string) => void; isRoot: boolean }) {
  const findings = useFindings();
  const notes = useNotes();
  const reveal = useReveal();
  const selected = useSelection((s) => s.paths.has(focus.path));
  const toggle = useSelection((s) => s.togglePath);
  const n = focus.node;
  const related = (findings.data?.findings ?? []).filter((f) => (f.targets ?? []).some((t) => within(t.path, focus.path) || within(focus.path, t.path)));
  const shared = n.alloc - n.unique;
  return (
    <aside className="sidepanel" aria-live="polite">
      <div className="stack" style={{ gap: 4 }}>
        <h2 className="ellipsis" title={n.name}>{isRoot ? "This folder" : n.name}</h2>
        {focus.path && <code className="faint wrap">{focus.path}</code>}
      </div>
      <dl className="facts">
        <dt>Size</dt>
        <dd className="num">{bytes(n.alloc)}</dd>
        {shared > 0 && (
          <>
            <dt>Shared</dt>
            <dd className="num" title="Blocks also used by clones or hardlinks elsewhere">{bytes(shared)}</dd>
          </>
        )}
        <dt>Files</dt>
        <dd className="num">{count(n.files)}</dd>
        {n.modified ? (
          <>
            <dt>Changed</dt>
            <dd>{ago(n.modified)}</dd>
          </>
        ) : null}
      </dl>
      {related.length > 0 && (
        <div className="stack" style={{ gap: 8 }}>
          <h2>Findings here</h2>
          {related.slice(0, 5).map((f) => (
            <Link key={f.id} to="/findings" hash={f.id} className="related">
              <span>{f.title}</span>
              <span className="row" style={{ gap: 8 }}>
                <span className="num muted">{bytes(f.bytes)}</span>
                <Safety level={f.safety} />
              </span>
            </Link>
          ))}
        </div>
      )}
      {focus.path && (
        <div className="actions">
          {n.kind === 0 && !isRoot && (
            <button className="btn small" onClick={() => onOpen(focus.path)}>
              <FolderOpen size={14} aria-hidden /> Open
            </button>
          )}
          <button className="btn small" onClick={() => reveal.mutate(focus.path)}>
            <FolderSearch size={14} aria-hidden /> Show in Finder
          </button>
          {!isRoot && (
            <button className="btn small" onClick={() => toggle(focus.path)}>
              {selected ? <ListX size={14} aria-hidden /> : <ListPlus size={14} aria-hidden />}
              {selected ? "Remove from cleanup" : "Add to cleanup"}
            </button>
          )}
        </div>
      )}
      {focus.path && <NoteEditor path={focus.path} initial={notes.data?.[`path:${focus.path}`] ?? ""} />}
    </aside>
  );
}

function NoteEditor({ path, initial }: { path: string; initial: string }) {
  const [text, setText] = useState(initial);
  const save = useSetNote();
  useEffect(() => setText(initial), [initial, path]);
  return (
    <div className="stack" style={{ gap: 6 }}>
      <label className="faint" htmlFor="note">Your note</label>
      <textarea
        id="note"
        className="input"
        rows={3}
        value={text}
        placeholder="Why this is here, when to clean it…"
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text !== initial && save.mutate({ key: `path:${path}`, text })}
      />
    </div>
  );
}
