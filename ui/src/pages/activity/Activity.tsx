import { Undo2 } from "lucide-react";
import { useOps, useUndo } from "#/api/hooks.ts";
import { ago, bytes } from "#/lib/format.ts";
import { Loading, Problem } from "#/ui/states.tsx";

const verbs: Record<string, string> = { delete: "Deleted", trash: "Moved to Trash", command: "Ran the tool's cleanup", "empty-trash": "Emptied the Trash", undo: "Put back" };

export default function Activity() {
  const ops = useOps();
  const undo = useUndo();
  if (ops.error) return <Problem error={ops.error} />;
  if (!ops.data) return <Loading />;
  const list = ops.data.ops ?? [];
  const undone = new Set(list.filter((o) => o.action === "undo").map((o) => o.id.replace(/^undo-/, "")));
  return (
    <div className="page-narrow stack">
      <h1>What ebb did</h1>
      {list.length === 0 && <p className="empty">Nothing cleaned yet. Every action ebb takes is listed here, and items moved to the Trash can be put back.</p>}
      <ul className="ops">
        {list.map((op) => (
          <li key={op.id}>
            <span className="faint">{ago(op.time)}</span>
            <span className={op.result === "done" ? "" : "error"}>{op.result === "done" ? verbs[op.action] ?? op.action : `${op.result}: ${op.error ?? ""}`}</span>
            <span className="num">{bytes(op.bytes)}</span>
            <span className="mono ellipsis" title={op.path}>{op.path}</span>
            {op.trashUrl && op.result === "done" && !undone.has(op.id) && (
              <button className="btn small" disabled={undo.isPending} onClick={() => undo.mutate(op.id)}>
                <Undo2 size={13} aria-hidden /> Put back
              </button>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
