import { useFindings } from "#/api/hooks.ts";
import { bytes } from "#/lib/format.ts";
import { useSelection } from "#/lib/selection.ts";
import { useSheet } from "#/lib/sheet.ts";

/** Collects what was picked anywhere in the app and opens one plan for it. */
export function SelectionTray() {
  const { findings, paths, clear } = useSelection();
  const all = useFindings();
  const open = useSheet((s) => s.open);
  const n = findings.size + paths.size;
  if (n === 0) return null;
  const known = (all.data?.findings ?? []).filter((f) => findings.has(f.id)).reduce((s, f) => s + f.bytes, 0);
  return (
    <div className="tray" role="region" aria-label="Cleanup selection">
      <span>
        <strong>{n}</strong> selected{known > 0 && <span className="muted num"> · at least {bytes(known)}</span>}
      </span>
      <button className="btn ghost small" onClick={clear}>Clear</button>
      <button className="btn primary" onClick={() => open({ findingIds: [...findings], paths: [...paths] }, { clearSelection: true })}>
        Review cleanup
      </button>
    </div>
  );
}
