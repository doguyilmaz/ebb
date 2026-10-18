import { useQueryClient } from "@tanstack/react-query";
import { ExternalLink, Loader2, MessageSquareText, RefreshCw, ScanSearch, ShieldCheck } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { listen } from "#/api/events.ts";
import { apiPort, connect, isRemote, onAuthRejected, pair, setApiPort, token, wasEmbedded } from "#/api/client.ts";
import { detectOS, isSafari } from "#/lib/os.ts";
import { Command } from "#/ui/Command.tsx";

type State =
  | { kind: "connecting" }
  | { kind: "absent" }
  | { kind: "not-allowed" }
  | { kind: "pairing"; code: string; confirm: string }
  | { kind: "no-token" }
  | { kind: "ready" };

const RETRY_MS = 2500;
const RELEASES = "https://github.com/doguyilmaz/ebb/releases/latest";
const FDA_SETTINGS = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles";

/**
 * Finds ebb on this Mac and holds the app back until it may talk to it. Every
 * state says what is going on and the one thing to do; the page keeps checking
 * on its own, so starting ebb or confirming the page is enough.
 */
export function Connect({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ kind: "connecting" });
  const [port, setPort] = useState(apiPort());
  const [busy, setBusy] = useState(false);
  const qc = useQueryClient();

  const attempt = useCallback(async () => {
    const c = await connect();
    if (!c) return setState({ kind: "absent" });
    if (!c.health.allowed) return setState({ kind: "not-allowed" });
    if (!c.remote) return setState(token() ? { kind: "ready" } : { kind: "no-token" });
    if (token()) return setState({ kind: "ready" });
    try {
      const r = await pair();
      setState(r.paired ? { kind: "ready" } : { kind: "pairing", code: r.code, confirm: r.confirm });
    } catch {
      setState({ kind: "absent" });
    }
  }, []);

  useEffect(() => {
    void attempt();
  }, [attempt]);

  useEffect(() => {
    if (state.kind === "ready" || state.kind === "connecting" || state.kind === "no-token") return;
    const id = setInterval(() => void attempt(), state.kind === "pairing" ? 2000 : RETRY_MS);
    return () => clearInterval(id);
  }, [state.kind, attempt]);

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

  const retry = async () => {
    setBusy(true);
    setApiPort(port);
    await attempt();
    setBusy(false);
  };
  const where = `127.0.0.1:${port}`;
  const serve = port === 47340 ? "ebb serve" : `ebb serve --port ${port}`;
  const pickPort = (p: number) => {
    setPort(p);
    if (p > 0 && p < 65536) setApiPort(p);
  };
  const footer = <Footer port={port} setPort={pickPort} onRetry={retry} busy={busy} />;

  switch (state.kind) {
    case "connecting":
      return (
        <Gate title="Looking for ebb…">
          <p className="muted row" role="status">
            <Loader2 size={15} className="spin" aria-hidden /> Checking this Mac
          </p>
        </Gate>
      );
    case "no-token":
      return (
        <Gate title="Open ebb from its link" sub="This window has no access key. The link ebb prints carries one, which keeps other pages on this Mac from using ebb.">
          <Command value="ebb serve --open" />
          <p className="faint small-text">Already running? Use the link it printed in Terminal.</p>
        </Gate>
      );
    case "not-allowed":
      return (
        <Gate title="Allow this page first" sub="ebb is running, but it only talks to pages you allow, so no other website can look at your disk. Run this on your Mac:">
          <Command value={`ebb web allow ${location.origin}`} />
          <Waiting where={where} text="Connects as soon as you allow it" />
          {footer}
        </Gate>
      );
    case "pairing":
      return (
        <Gate title="Confirm this page on your Mac" sub="ebb never hands its data to a web page until you confirm it once. Run this in Terminal:">
          <Command value={state.confirm} />
          <p className="muted">Terminal shows a code. It must match this one:</p>
          <div className="code" aria-label={`Code ${state.code.split("").join(" ")}`}>{state.code}</div>
          <p className="faint small-text">
            A different code means something else asked. Run <code>ebb web reject</code>, then reload this page.
          </p>
          <Waiting where={where} text="Connects as soon as you confirm" />
          {footer}
        </Gate>
      );
    default:
      if (wasEmbedded())
        return (
          <Gate title="ebb stopped" sub="This window was talking to ebb on this Mac, and ebb is not running right now. Start it again; this page reconnects by itself.">
            <Command value={serve} />
            <Waiting where={where} text="Reconnects on its own" />
            {footer}
          </Gate>
        );
      return detectOS() === "mac" ? (
        <Gate title="Start ebb on this Mac" sub="This page works with ebb running on your own Mac. Your files and what ebb finds about them never leave it." intro>
          <InstallGuide serve={serve} />
          <Waiting where={where} text="Connects on its own" />
          {footer}
        </Gate>
      ) : (
        <Gate title="ebb runs on macOS" sub="It reads what only macOS knows about a disk: APFS clones and snapshots, purgeable space, and the caches Xcode and simulators leave behind. Open this page on your Mac to get started." intro />
      );
  }
}

function InstallGuide({ serve }: { serve: string }) {
  return (
    <>
      <ol className="steps">
        <Step n={1} title="Install ebb">
          <Command value="brew install doguyilmaz/tap/ebb" />
          <p className="faint small-text">
            No Homebrew? Get it at{" "}
            <a href="https://brew.sh" target="_blank" rel="noreferrer">brew.sh</a>, or download the <code>darwin</code> archive (one binary for Apple
            silicon and Intel) from the{" "}
            <a href={RELEASES} target="_blank" rel="noreferrer">latest release</a> and put <code>ebb</code> on your PATH.
          </p>
        </Step>
        <Step n={2} title="Start it">
          <Command value={serve} />
          <p className="faint small-text">It runs in that Terminal window and this page connects to it. Until you approve a cleanup, ebb only reads.</p>
        </Step>
        <Step n={3} title="Let it see everything" optional>
          <p className="muted small-text">
            macOS hides some folders (Mail, Safari, other apps' data) until Terminal has Full Disk Access. Without it ebb still works; that space shows as
            “Not visible”.
          </p>
          <a className="btn small" href={FDA_SETTINGS}>
            Open Full Disk Access settings <ExternalLink size={13} aria-hidden />
          </a>
        </Step>
      </ol>
      {isSafari() && (
        <p className="gate-note small-text">
          Safari does not let web pages reach your own Mac. Use Chrome, Edge or Firefox for this page, or the window <code>ebb serve --open</code> opens.
        </p>
      )}
    </>
  );
}

function Step({ n, title, optional, children }: { n: number; title: string; optional?: boolean; children: ReactNode }) {
  return (
    <li className="step">
      <span className="step-n" aria-hidden>{n}</span>
      <div className="stack" style={{ gap: 8, minWidth: 0, flex: 1 }}>
        <span className="step-title">
          {title}
          {optional && <span className="faint"> · optional</span>}
        </span>
        {children}
      </div>
    </li>
  );
}

/** Says the page is already looking, so there is nothing to press after starting ebb. */
function Waiting({ where, text }: { where: string; text: string }) {
  return (
    <p className="waiting small-text" role="status">
      <span className="pulse" aria-hidden />
      Waiting for ebb on <code>{where}</code>
      <span className="faint"> · {text}</span>
    </p>
  );
}

const features = [
  { icon: ScanSearch, text: "Shows where your space went" },
  { icon: MessageSquareText, text: "Explains every large item" },
  { icon: ShieldCheck, text: "Cleans only what you approve" },
];

function Gate({ title, sub, intro, children }: { title: string; sub?: string; intro?: boolean; children?: ReactNode }) {
  return (
    <main className="gate">
      <div className="gate-wrap">
        <div className="gate-brand">
          <img src="/favicon.svg" width={36} height={36} alt="" />
          <div className="stack" style={{ gap: 0 }}>
            <span className="gate-name">ebb</span>
            <span className="faint small-text">Where your disk space went, and how to get it back</span>
          </div>
        </div>
        <div className="gate-card">
          <div className="stack" style={{ gap: 6 }}>
            <h1>{title}</h1>
            {sub && <p className="muted">{sub}</p>}
          </div>
          {intro && (
            <ul className="gate-features">
              {features.map(({ icon: Icon, text }) => (
                <li key={text}>
                  <Icon size={17} aria-hidden />
                  {text}
                </li>
              ))}
            </ul>
          )}
          {children}
        </div>
      </div>
    </main>
  );
}

/** Check now, the port ebb listens on (ebb serve --port), and ebb's own window. */
function Footer({ port, setPort, onRetry, busy }: { port: number; setPort: (p: number) => void; onRetry: () => void; busy: boolean }) {
  return (
    <div className="gate-foot">
      <button className="btn small" onClick={onRetry} disabled={busy}>
        <RefreshCw size={13} className={busy ? "spin" : undefined} aria-hidden /> Check again
      </button>
      <span className="faint small-text">Checks every few seconds</span>
      <span className="grow" />
      <label className="faint small-text" htmlFor="port" data-tip="The port ebb listens on. Change it if you started ebb with --port.">
        Port
      </label>
      <input id="port" className="input mono" type="number" value={port} onChange={(e) => setPort(Number(e.target.value) || 0)} onKeyDown={(e) => e.key === "Enter" && onRetry()} style={{ width: 92 }} />
      <a className="small-text row" style={{ gap: 4 }} href={`http://127.0.0.1:${port}/`} target="_blank" rel="noreferrer">
        Local window <ExternalLink size={12} aria-hidden />
      </a>
    </div>
  );
}
