import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type ReactNode } from "react";
import { listen } from "#/api/events.ts";
import { connect, isRemote, onAuthRejected, pair, token } from "#/api/client.ts";

type State =
  | { kind: "connecting" }
  | { kind: "absent" }
  | { kind: "not-allowed" }
  | { kind: "pairing"; code: string; confirm: string }
  | { kind: "no-token" }
  | { kind: "ready" };

/** Finds ebb on this machine and holds the app back until it may talk to it. */
export function Connect({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ kind: "connecting" });
  const [attempt, setAttempt] = useState(0);
  const qc = useQueryClient();

  useEffect(() => {
    let stop = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    (async () => {
      const c = await connect();
      if (stop) return;
      if (!c) return setState({ kind: "absent" });
      if (!c.health.allowed) return setState({ kind: "not-allowed" });
      if (!c.remote) return setState(token() ? { kind: "ready" } : { kind: "no-token" });
      if (token()) return setState({ kind: "ready" });
      const tryPair = async () => {
        try {
          const r = await pair();
          if (stop) return;
          if (r.paired) return setState({ kind: "ready" });
          setState({ kind: "pairing", code: r.code, confirm: r.confirm });
          timer = setTimeout(tryPair, 2000);
        } catch {
          if (!stop) setState({ kind: "absent" });
        }
      };
      tryPair();
    })();
    return () => {
      stop = true;
      clearTimeout(timer);
    };
  }, [attempt]);

  useEffect(() => {
    if (state.kind !== "ready") return;
    const off = onAuthRejected(() => setState(isRemote() ? { kind: "connecting" } : { kind: "no-token" }));
    const unlisten = listen(qc);
    return () => {
      off();
      unlisten();
    };
  }, [state.kind, qc]);

  if (state.kind === "ready") return <>{children}</>;
  const retry = <button className="btn" onClick={() => setAttempt((n) => n + 1)}>Try again</button>;
  return (
    <div className="gate">
      <div className="panel">
        {state.kind === "connecting" && <p className="muted">Looking for ebb on this Mac…</p>}
        {state.kind === "absent" && (
          <>
            <h1>ebb is not running</h1>
            <p className="muted">Start it on this Mac, then try again:</p>
            <code>ebb serve</code>
            {retry}
          </>
        )}
        {state.kind === "not-allowed" && (
          <>
            <h1>This page is not allowed yet</h1>
            <p className="muted">Allow it on the Mac you want to use, then try again:</p>
            <code>ebb web allow {location.origin}</code>
            {retry}
          </>
        )}
        {state.kind === "pairing" && (
          <>
            <h1>Confirm this page on your Mac</h1>
            <p className="muted">Check that your Mac shows the same code, then run:</p>
            <div className="code">{state.code}</div>
            <code>{state.confirm}</code>
            <p className="faint">Or approve it in Settings of ebb's local window. This page connects as soon as you do.</p>
          </>
        )}
        {state.kind === "no-token" && (
          <>
            <h1>Open ebb from its own link</h1>
            <p className="muted">This window has no access key. Run this and use the link it opens:</p>
            <code>ebb serve --open</code>
          </>
        )}
      </div>
    </div>
  );
}
