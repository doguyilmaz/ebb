import { ChevronRight, FolderOpen, FolderSearch, Search, Square, Trash2 } from "lucide-react";
import { useState } from "react";
import { api } from "#/api/client.ts";
import { useDuplicates, useFindDuplicates, useReveal, useStopDuplicates } from "#/api/hooks.ts";
import type { ChooseResponse, Group } from "#/api/types.gen.ts";
import { ago, baseName, bytes, count } from "#/lib/format.ts";
import { useSelection } from "#/lib/selection.ts";
import { useSheet } from "#/lib/sheet.ts";
import { Check } from "#/ui/Check.tsx";
import { PageHead } from "#/ui/PageHead.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { extras, groupKey } from "./extras.ts";

const sizes = [
  { label: "1 MB", value: 1 << 20 },
  { label: "10 MB", value: 10 << 20 },
  { label: "100 MB", value: 100 << 20 },
];
const phases: Record<string, string> = { sample: "Comparing the start and end of same-size files", hash: "Reading whole files to be sure" };

export default function Duplicates() {
  const dupes = useDuplicates();
  const find = useFindDuplicates();
  const stop = useStopDuplicates();
  const openSheet = useSheet((s) => s.open);
  // The findings tray sits at the bottom too; this one goes above it.
  const raised = useSelection((s) => s.findings.size + s.paths.size > 0);
  const [minSize, setMinSize] = useState(sizes[0].value);
  const [folder, setFolder] = useState("");
  const [picked, setPicked] = useState<ReadonlySet<string>>(new Set());
  const [keep, setKeep] = useState<ReadonlyMap<string, string>>(new Map());
  if (dupes.error) return <Problem error={dupes.error} />;
  if (!dupes.data) return <Loading />;
  const st = dupes.data;
  const groups = st.result?.groups ?? [];
  const chosen = extras(groups, picked, keep);
  const allPicked = groups.length > 0 && groups.every((g) => picked.has(groupKey(g)));
  const browse = async () => {
    const p = (await api.post<ChooseResponse>("/choose-folder", { purpose: "duplicates" }).catch(() => null))?.path;
    if (p) setFolder(p);
  };
  const toggle = (g: Group) => {
    const next = new Set(picked);
    if (!next.delete(groupKey(g))) next.add(groupKey(g));
    setPicked(next);
  };

  return (
    <>
      <PageHead title="Duplicates" sub="Files with the same content in your own folders. Keep one copy of each, move the others to the Trash.">
        <div className="seg" role="group" aria-label="Smallest file">
          {sizes.map((s) => (
            <button key={s.value} aria-pressed={minSize === s.value} onClick={() => setMinSize(s.value)} data-tip={`Only files of ${s.label} or more`}>
              {s.label}+
            </button>
          ))}
        </div>
        <button className="btn" onClick={browse} data-tip={folder ? folder : "Your home folder; apps' data, libraries and bundles are left out"}>
          <FolderOpen size={14} aria-hidden /> {folder ? baseName(folder) : "Home"}
        </button>
        {st.running ? (
          <button className="btn" onClick={() => stop.mutate()}>
            <Square size={13} aria-hidden /> Stop
          </button>
        ) : (
          <button className="btn primary" disabled={find.isPending} onClick={() => find.mutate({ path: folder || undefined, minSize })} data-tour="dupes">
            <Search size={14} aria-hidden /> {st.result ? "Search again" : "Find duplicates"}
          </button>
        )}
      </PageHead>

      {(find.error || st.error) && <p className="error" role="alert">{(find.error as Error | null)?.message ?? st.error}</p>}

      {st.running && (
        <section className="card dupes-progress" role="status">
          <span>{phases[st.progress?.phase ?? ""] ?? "Grouping files by size"}…</span>
          <span className="num faint">{st.progress ? `${count(st.progress.done)} of ${count(st.progress.total)}` : ""}</span>
          <div className="bar" aria-hidden>
            <i style={{ transform: `scaleX(${st.progress?.total ? st.progress.done / st.progress.total : 0})` }} />
          </div>
        </section>
      )}

      {!st.running && !st.result && (
        <section className="card empty">
          ebb reads files that have the same size and compares their content, so this takes a while on big folders. Bundles, Photos and Music
          libraries, your Library, node_modules and hidden folders are left out: their files belong to something else.
        </section>
      )}

      {st.result && (
        <>
          <p className="muted dupes-sum">
            {groups.length === 0 ? (
              <>No duplicates of {bytes(minSize)} or more in {st.result.root}.</>
            ) : (
              <>
                <strong className="num">{bytes(st.result.wasted)}</strong> in extra copies, {count(groups.length)} {groups.length === 1 ? "file" : "files"} with copies in{" "}
                <span className="mono">{st.result.root}</span>
              </>
            )}{" "}
            <span className="faint">· searched {ago(st.result.time)}</span>
          </p>
          {groups.length > 0 && (
            <section className="card dupes">
              <div className="dupes-head">
                <Check
                  state={allPicked ? "all" : picked.size ? "some" : "none"}
                  label="Select every file"
                  onClick={() => setPicked(allPicked ? new Set() : new Set(groups.map(groupKey)))}
                />
                <span className="faint small-text">The copy marked Keep stays; pick another if you like</span>
              </div>
              <ul>
                {groups.map((g) => (
                  <Row key={groupKey(g)} g={g} root={st.result!.root} picked={picked.has(groupKey(g))} onPick={() => toggle(g)} kept={keep.get(groupKey(g)) ?? groupKey(g)} onKeep={(p) => setKeep(new Map(keep).set(groupKey(g), p))} />
                ))}
              </ul>
            </section>
          )}
        </>
      )}

      {chosen.paths.length > 0 && (
        <div className="tray dupes-tray" role="region" aria-label="Selected copies" data-raised={raised || undefined}>
          <span>
            {chosen.paths.length} extra {chosen.paths.length === 1 ? "copy" : "copies"} · <strong className="num">{bytes(chosen.bytes)}</strong>
          </span>
          <button className="btn ghost small" onClick={() => setPicked(new Set())}>Clear</button>
          <button className="btn primary small" onClick={() => openSheet({ paths: chosen.paths, source: "ui" })}>
            <Trash2 size={13} aria-hidden /> Review
          </button>
        </div>
      )}
    </>
  );
}

/** A copy's place, from the folder that was searched. */
const within = (root: string, p: string) => (p.startsWith(root + "/") ? p.slice(root.length + 1) : p);

function Row({ g, root, picked, onPick, kept, onKeep }: { g: Group; root: string; picked: boolean; onPick: () => void; kept: string; onKeep: (path: string) => void }) {
  const [open, setOpen] = useState(false);
  const reveal = useReveal();
  const copies = g.copies ?? [];
  return (
    <li className="dupe" data-open={open}>
      <Check state={picked ? "all" : "none"} label={`Select ${baseName(copies[0]?.path ?? "")}`} onClick={onPick} />
      <button className="dupe-main" onClick={() => setOpen(!open)} aria-expanded={open}>
        <ChevronRight size={14} className="chev" aria-hidden />
        <span className="ellipsis">{baseName(copies[0]?.path ?? "")}</span>
        <span className="faint num">
          {copies.length} × {bytes(g.size)}
        </span>
        <span className="num">{bytes(g.wasted)}</span>
      </button>
      {open && (
        <ul className="dupe-copies">
          {copies.map((c) => (
            <li key={c.path}>
              <label className="row">
                <input type="radio" name={`keep-${copies[0].path}`} checked={kept === c.path} onChange={() => onKeep(c.path)} />
                <span className="tag">{kept === c.path ? "Keep" : "Extra"}</span>
              </label>
              <span className="mono ellipsis" title={c.path}>{within(root, c.path)}</span>
              <span className="faint small-text">{ago(c.mtime)}</span>
              <button className="icon-btn" aria-label={`Show ${c.path} in Finder`} data-tip="Show in Finder" onClick={() => reveal.mutate(c.path)}>
                <FolderSearch size={14} />
              </button>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}
