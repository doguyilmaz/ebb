import { useVirtualizer } from "@tanstack/react-virtual";
import { Folder, File, Square, SquareCheck } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { useChildren } from "#/api/hooks.ts";
import type { Item } from "#/api/types.gen.ts";
import { ago, bytes, count, percent } from "#/lib/format.ts";
import { useSelection } from "#/lib/selection.ts";
import { Loading, Problem } from "#/ui/states.tsx";
import type { Focus } from "./SidePanel.tsx";

type Sort = "size" | "name" | "files" | "modified";

/** The table view: every child of the folder, sortable, for reading exact numbers. */
export function ListView({ path, onOpen, onPick }: { path: string; onOpen: (p: string) => void; onPick: (f: Focus) => void }) {
  const [small, setSmall] = useState(false);
  const [sort, setSort] = useState<Sort>("size");
  const children = useChildren(path, small);
  const selected = useSelection((s) => s.paths);
  const toggle = useSelection((s) => s.togglePath);
  const scroller = useRef<HTMLDivElement>(null);
  const rows = useMemo(() => {
    const items: Item[] = [...(children.data?.items ?? [])];
    const by: Record<Sort, (a: Item, b: Item) => number> = {
      size: (a, b) => b.alloc - a.alloc,
      name: (a, b) => a.name.localeCompare(b.name),
      files: (a, b) => b.files - a.files,
      modified: (a, b) => b.modified - a.modified,
    };
    return items.sort(by[sort]);
  }, [children.data, sort]);
  const total = rows.reduce((n, r) => n + r.alloc, 0) || 1;
  const virt = useVirtualizer({ count: rows.length, getScrollElement: () => scroller.current, estimateSize: () => 38, overscan: 12 });

  if (children.error) return <Problem error={children.error} />;
  if (!children.data) return <Loading />;
  const head = (s: Sort, label: string) => (
    <button className="th" aria-pressed={sort === s} onClick={() => setSort(s)}>
      {label}
    </button>
  );
  return (
    <div className="listview">
      <div className="lv-row lv-head">
        <span />
        {head("name", "Name")}
        {head("size", "Size")}
        {head("files", "Files")}
        {head("modified", "Changed")}
      </div>
      <div className="lv-scroll" ref={scroller}>
        <div style={{ height: virt.getTotalSize(), position: "relative" }}>
          {virt.getVirtualItems().map((v) => {
            const it = rows[v.index];
            const picked = selected.has(it.path);
            return (
              <div key={it.path} className="lv-row" style={{ position: "absolute", top: 0, left: 0, right: 0, transform: `translateY(${v.start}px)` }}>
                <button className="icon-btn" aria-label={picked ? "Remove from cleanup" : "Add to cleanup"} aria-pressed={picked} onClick={() => toggle(it.path)}>
                  {picked ? <SquareCheck size={16} /> : <Square size={16} />}
                </button>
                <button
                  className="lv-name"
                  onClick={() => (it.kind === "dir" ? onOpen(it.path) : onPick({ path: it.path, node: { id: 0, name: it.name, kind: 1, alloc: it.alloc, unique: it.bytes, files: it.files, modified: it.modified, children: null } }))}
                >
                  {it.kind === "dir" ? <Folder size={15} aria-hidden /> : <File size={15} aria-hidden />}
                  <span className="ellipsis">{it.name}</span>
                  {it.findings?.length ? <span className="tag">finding</span> : null}
                  {it.note ? <span className="tag">note</span> : null}
                </button>
                <span className="lv-size num">
                  <i style={{ width: `${(it.alloc / total) * 100}%` }} />
                  {bytes(it.alloc)} <span className="faint">{percent(it.alloc / total)}</span>
                </span>
                <span className="num muted">{count(it.files)}</span>
                <span className="muted">{it.modified ? ago(it.modified) : "-"}</span>
              </div>
            );
          })}
        </div>
      </div>
      <label className="lv-foot muted">
        <input type="checkbox" checked={small} onChange={(e) => setSmall(e.target.checked)} /> Also list files under 1 MB
        {small && children.data.small && ` (${children.data.small.length})`}
      </label>
      {small && (children.data.small?.length ?? 0) > 0 && (
        <ul className="small-list">
          {children.data.small!.slice(0, 200).map((f) => (
            <li key={f.name}>
              <span className="mono ellipsis">{f.name}</span>
              <span className="num muted">{bytes(f.size)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
