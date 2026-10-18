import { useVirtualizer } from "@tanstack/react-virtual";
import { File, Square, SquareCheck } from "lucide-react";
import { useRef, useState } from "react";
import { useFiles } from "#/api/hooks.ts";
import { ago, bytes, count } from "#/lib/format.ts";
import { readPrefs, savePrefs } from "#/lib/prefs.ts";
import { useSelection } from "#/lib/selection.ts";
import { Loading, Problem } from "#/ui/states.tsx";
import type { Focus } from "./SidePanel.tsx";

const sizes = [
  { label: "1 MB+", value: 1 << 20 },
  { label: "100 MB+", value: 100 << 20 },
  { label: "1 GB+", value: 1 << 30 },
];
const ages = [
  { label: "Any age", days: 0 },
  { label: "6 months+", days: 182 },
  { label: "1 year+", days: 365 },
  { label: "2 years+", days: 730 },
];

/** The biggest files anywhere below the folder, filtered by size and by how long nobody changed them. */
export function FilesView({ path, onPick }: { path: string; onPick: (f: Focus) => void }) {
  const [minSize, setMinSize] = useState(() => readPrefs().filesMin ?? sizes[1].value);
  const [days, setDays] = useState(() => readPrefs().filesAge ?? 0);
  const files = useFiles(path, minSize, days);
  const selected = useSelection((s) => s.paths);
  const toggle = useSelection((s) => s.togglePath);
  const scroller = useRef<HTMLDivElement>(null);
  const rows = files.data?.items ?? [];
  const virt = useVirtualizer({ count: rows.length, getScrollElement: () => scroller.current, estimateSize: () => 44, overscan: 12 });
  const where = (p: string) => {
    const dir = p.slice(0, p.lastIndexOf("/"));
    return dir.startsWith(path + "/") ? dir.slice(path.length + 1) : dir === path ? "" : dir;
  };
  const total = rows.reduce((n, r) => n + r.alloc, 0);

  return (
    <div className="listview filesview">
      <div className="fv-bar">
        <div className="seg" role="group" aria-label="Size">
          {sizes.map((s) => (
            <button key={s.value} aria-pressed={minSize === s.value} onClick={() => (setMinSize(s.value), savePrefs({ filesMin: s.value }))}>
              {s.label}
            </button>
          ))}
        </div>
        <div className="seg" role="group" aria-label="Unchanged for">
          {ages.map((a) => (
            <button key={a.days} aria-pressed={days === a.days} onClick={() => (setDays(a.days), savePrefs({ filesAge: a.days }))} data-term={a.days ? "age" : undefined}>
              {a.label}
            </button>
          ))}
        </div>
        <span className="faint small-text num">{files.data && `${count(rows.length)} files · ${bytes(total)}`}</span>
      </div>
      {files.error ? (
        <Problem error={files.error} />
      ) : !files.data ? (
        <Loading />
      ) : rows.length === 0 ? (
        <p className="empty">No file of {bytes(minSize)} or more{days ? ` unchanged for ${ages.find((a) => a.days === days)?.label.replace("+", " or more")}` : ""} in this folder.</p>
      ) : (
        <div className="lv-scroll" ref={scroller} data-stale={files.isPlaceholderData || undefined}>
          <div style={{ height: virt.getTotalSize(), position: "relative" }}>
            {virt.getVirtualItems().map((v) => {
              const it = rows[v.index];
              const picked = selected.has(it.path);
              return (
                <div key={it.path} className="lv-row fv-row" style={{ position: "absolute", top: 0, left: 0, right: 0, transform: `translateY(${v.start}px)` }}>
                  <button className="icon-btn" aria-label={picked ? "Remove from cleanup" : "Add to cleanup"} aria-pressed={picked} onClick={() => toggle(it.path)}>
                    {picked ? <SquareCheck size={16} /> : <Square size={16} />}
                  </button>
                  <button
                    className="lv-name"
                    onClick={() => onPick({ path: it.path, node: { id: 0, name: it.name, kind: 1, alloc: it.alloc, unique: it.bytes, files: 1, modified: it.modified, children: null } })}
                  >
                    <File size={15} aria-hidden />
                    <span className="fv-name">
                      <span className="ellipsis">{it.name}</span>
                      <span className="faint small-text mono ellipsis">{where(it.path) || "here"}</span>
                    </span>
                    {it.findings?.length ? <span className="tag">finding</span> : null}
                  </button>
                  <span className="num">{bytes(it.alloc)}</span>
                  <span className="muted">{it.modified ? ago(it.modified) : "-"}</span>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
