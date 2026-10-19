import { Ban, Hand, Loader2, Terminal, Trash2, Undo2, X } from "lucide-react";
import { useEffect, useReducer, useRef } from "react";
import { api } from "#/api/client.ts";
import { useRefreshResults, useUndo } from "#/api/hooks.ts";
import type { Plan, PlanItem, PlanResult } from "#/api/types.gen.ts";
import { bytes } from "#/lib/format.ts";
import { useLive } from "#/lib/live.ts";
import type { SheetRequest } from "#/lib/sheet.ts";
import { Safety } from "#/ui/Safety.tsx";
import { canRun, initial, reduce, summary } from "./cleanup.ts";

const actionLabel = (it: PlanItem) => {
  switch (it.action.kind) {
    case "command":
      return (it.command ?? []).join(" ");
    case "delete":
      return "Delete";
    case "trash":
      return "Move to Trash";
    case "empty-trash":
      return "Empty the Trash";
    default:
      return "Do it yourself";
  }
};

const ActionIcon = ({ it }: { it: PlanItem }) =>
  it.blocked ? <Ban size={14} aria-hidden /> : it.action.kind === "manual" ? <Hand size={14} aria-hidden /> : it.action.kind === "command" ? <Terminal size={14} aria-hidden /> : <Trash2 size={14} aria-hidden />;

const askers: Record<string, string> = { mcp: "An assistant", cli: "The terminal" };

/**
 * Shows exactly what a cleanup will do, asks, runs it and reports what
 * happened. A plan made elsewhere is only approved here: whoever asked runs it.
 */
export function CleanupSheet({ request, onClose, onDone }: { request: SheetRequest; onClose: () => void; onDone?: () => void }) {
  const [s, dispatch] = useReducer(reduce, initial);
  const dialog = useRef<HTMLDialogElement>(null);
  const events = useLive((l) => l.planEvents);
  const resetPlan = useLive((l) => l.resetPlan);
  const refresh = useRefreshResults();
  const undo = useUndo();

  const external = "planId" in request;
  useEffect(() => {
    dialog.current?.showModal();
    resetPlan();
    ("planId" in request ? api.get<Plan>(`/plans/${request.planId}`) : api.post<Plan>("/plans", { ...request, source: "ui" }))
      .then((plan) => dispatch({ type: "planned", plan }))
      .catch((e) => dispatch({ type: "error", message: e.message }));
  }, []);

  const run = async () => {
    if (!s.plan) return;
    dispatch({ type: "run" });
    try {
      await api.post(`/plans/${s.plan.id}/approve`);
      if (external) {
        dispatch({ type: "approved" });
        return;
      }
      const result = await api.post<PlanResult>(`/plans/${s.plan.id}/execute`);
      dispatch({ type: "done", result });
      refresh();
      onDone?.();
    } catch (e) {
      dispatch({ type: "error", message: (e as Error).message });
    }
  };

  const sum = s.plan ? summary(s.plan) : null;
  const doneIdx = new Map(events.map((e) => [e.item, e]));
  return (
    <dialog ref={dialog} className="sheet" onClose={onClose} aria-labelledby="sheet-title">
      <header className="sheet-head">
        <h1 id="sheet-title">
          {s.phase === "done" ? "Cleanup finished" : s.phase === "approved" ? "Approved" : external ? `${askers[s.plan?.source ?? ""] ?? "Something"} asks to clean` : "Review the cleanup"}
        </h1>
        <button className="icon-btn" aria-label="Close" onClick={() => dialog.current?.close()}>
          <X size={18} />
        </button>
      </header>
      <div className="sheet-body">
        {s.phase === "planning" && !s.error && <p className="muted">Checking each item on disk…</p>}
        {s.error && <p className="error" role="alert">{s.error}</p>}
        {s.phase === "approved" && (
          <p className="result-line">Nothing has been touched yet. {askers[s.plan?.source ?? ""] ?? "Whoever asked"} can run it now; every item is checked again first.</p>
        )}
        {s.plan && s.phase !== "done" && s.phase !== "approved" && (
          <>
            <p className="muted">
              {sum!.runnable} {sum!.runnable === 1 ? "item" : "items"} will be cleaned, about <strong>{bytes(sum!.bytes)}</strong>.
              {sum!.blocked > 0 && ` ${sum!.blocked} blocked.`}
              {sum!.manual > 0 && ` ${sum!.manual} you do yourself.`}
            </p>
            <ul className="plan-items">
              {(s.plan.items ?? []).map((it, i) => {
                const ev = doneIdx.get(i);
                return (
                  <li key={it.path} className={it.blocked ? "blocked" : ""}>
                    <span className="pi-action">
                      {ev?.state === "running" ? <Loader2 size={14} className="spin" aria-hidden /> : <ActionIcon it={it} />}
                      <span className="mono ellipsis" title={actionLabel(it)}>{actionLabel(it)}</span>
                    </span>
                    <span className="num">{bytes(it.bytes)}</span>
                    <Safety level={it.safety} />
                    <span className="mono pi-path">{it.path}</span>
                    {(it.blocked || it.warnings?.length || it.action.hint || ev?.err) && (
                      <span className="pi-note">
                        {it.blocked && <span className="error">{it.blocked}</span>}
                        {it.warnings?.map((w) => <span key={w} className="muted">{w}</span>)}
                        {it.action.kind === "manual" && it.action.hint && <code>{it.action.hint}</code>}
                        {ev?.err && <span className="error">{ev.err}</span>}
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>
            {sum!.trash > 0 && <p className="faint">Items moved to the Trash free their space once the Trash is emptied. Until then they can be put back.</p>}
          </>
        )}
        {s.phase === "done" && s.result && (
          <div className="stack">
            <p className="result-line">
              Freed about <strong>{bytes(s.result.freed)}</strong>: {s.result.done} done
              {s.result.refused > 0 && `, ${s.result.refused} refused`}
              {s.result.failed > 0 && `, ${s.result.failed} failed`}.
            </p>
            <ul className="plan-items">
              {(s.result.ops ?? [])
                .filter((op) => op.result !== "done" || op.trashUrl)
                .map((op) => (
                  <li key={op.id}>
                    <span className={op.result === "done" ? "muted" : "error"}>{op.result === "done" ? "In the Trash" : op.result}</span>
                    {op.trashUrl && (
                      <button className="btn small" disabled={undo.isPending} onClick={() => undo.mutate(op.id)}>
                        <Undo2 size={13} aria-hidden /> Put back
                      </button>
                    )}
                    <span className="mono pi-path">{op.path}</span>
                    {op.error && <span className="pi-note error">{op.error}</span>}
                  </li>
                ))}
            </ul>
          </div>
        )}
      </div>
      <footer className="sheet-foot">
        {s.phase === "review" && s.plan?.needsApproval && (
          <label className="row confirm">
            <input type="checkbox" checked={s.confirmed} onChange={(e) => dispatch({ type: "confirm", value: e.target.checked })} />
            I checked these items; some cannot be rebuilt by a tool.
          </label>
        )}
        <div className="grow" />
        {s.phase === "done" || s.phase === "approved" ? (
          <button className="btn primary" onClick={() => dialog.current?.close()}>Close</button>
        ) : (
          <>
            <button className="btn" onClick={() => dialog.current?.close()}>Cancel</button>
            <button className="btn primary" disabled={!canRun(s)} onClick={run}>
              {external ? (s.phase === "running" ? "Approving…" : "Approve") : s.phase === "running" ? "Cleaning…" : `Clean ${sum ? bytes(sum.bytes) : ""}`}
            </button>
          </>
        )}
      </footer>
    </dialog>
  );
}
