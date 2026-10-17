import { X } from "lucide-react";
import { useFindings } from "#/api/hooks.ts";
import { bytes } from "#/lib/format.ts";
import { useSelection } from "#/lib/selection.ts";
import { useSheet } from "#/lib/sheet.ts";
import { CountUp } from "#/ui/CountUp.tsx";

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
      <span className="row" style={{ gap: 10 }}>
        <strong className="num">{n} selected</strong>
        {known > 0 && (
          <span className="muted">
            {paths.size > 0 ? "at least " : ""}
            <CountUp id="tray" value={known} format={bytes} ms={400} className="num tray-bytes" />
          </span>
        )}
      </span>
      <button className="btn ghost small" onClick={clear}>
        <X size={13} aria-hidden /> Clear
      </button>
      <button className="btn primary" onClick={() => open({ findingIds: [...findings], paths: [...paths] }, { clearSelection: true })}>
        Review cleanup
      </button>
    </div>
  );
}
