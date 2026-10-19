import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { statusQuery, useStartScan } from "#/api/hooks.ts";
import { decline, useFdaAsk } from "#/lib/fda.ts";
import { FdaSteps, OpenFda } from "./FullDiskAccess.tsx";

/** Asks for Full Disk Access before a scan; turning it on goes on with that scan. */
export function FdaAsk() {
  const { pending, ask } = useFdaAsk();
  const start = useStartScan();
  const dialog = useRef<HTMLDialogElement>(null);
  const status = useQuery({ ...statusQuery, enabled: !!pending, refetchInterval: 2000 });
  const granted = !!status.data?.fullDiskAccess;

  useEffect(() => {
    if (pending) dialog.current?.showModal();
    else dialog.current?.close();
  }, [pending]);

  useEffect(() => {
    if (!pending || !granted) return;
    start.mutate(pending);
    ask(null);
  }, [pending, granted]);

  const scanAnyway = () => {
    decline();
    if (pending) start.mutate(pending);
    ask(null);
  };

  return (
    <dialog ref={dialog} className="fda-ask" aria-labelledby="fda-ask-title" onClose={() => ask(null)}>
      <div className="stack" style={{ gap: 14 }}>
        <div className="stack" style={{ gap: 6 }}>
          <h2 id="fda-ask-title">First, let ebb see everything</h2>
          <p className="muted">
            Without Full Disk Access, macOS hides app data, Mail and Messages from ebb and asks about each protected folder separately while it
            scans.
          </p>
          {status.data && <FdaSteps access={status.data.access} />}
        </div>
        <p className="waiting small-text" role="status">
          <span className="pulse" aria-hidden /> Scans as soon as it is on
        </p>
        <div className="row" style={{ gap: 10, flexWrap: "wrap" }}>
          <OpenFda primary />
          <button className="btn ghost" onClick={scanAnyway} data-tip="macOS then asks about each protected folder">
            Scan without it
          </button>
          <span className="grow" />
          <button className="btn ghost" onClick={() => ask(null)}>
            Cancel
          </button>
        </div>
      </div>
    </dialog>
  );
}
