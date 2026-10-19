import { useQueryClient } from "@tanstack/react-query";
import { ExternalLink, Loader2, MessageSquareText, RefreshCw, ScanSearch, ShieldCheck } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { listen } from "#/api/events.ts";
import { apiPort, claim, connect, forgetToken, isRemote, onAuthRejected, onTokenFromOtherTab, pairedBefore, setApiPort, token, wasEmbedded } from "#/api/client.ts";
import { detectOS, isSafari } from "#/lib/os.ts";
import { isBehind, pageVersion, versionLabel } from "#/lib/version.ts";
import { Command } from "#/ui/Command.tsx";

type State =
  | { kind: "connecting" }
  | { kind: "absent" }
  | { kind: "not-allowed" }
  | { kind: "unpaired"; expired: boolean }
  | { kind: "no-token" }
  | { kind: "ready" };

const RETRY_MS = 2500;
const MAX_RETRY_MS = 30_000;
const RELEASES = "https://github.com/doguyilmaz/ebb/releases/latest";
const SITE = "https://ebb.doguyilmaz.com";

/**
 * Finds ebb on this Mac and holds the app back until it may talk to it. Every
 * state says what is going on and the one thing to do; the page keeps checking
 * on its own, so running the command it shows is enough.
 */
export function Connect({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ kind: "connecting" });
  const [port, setPort] = useState(apiPort());
  const [busy, setBusy] = useState(false);
  const [running, setRunning] = useState("");
  const qc = useQueryClient();

  const attempt = useCallback(async () => {
    const c = await connect();
    setRunning(c?.health.version ?? "");
    if (!c) return setState({ kind: "absent" });
    if (!c.health.allowed) return setState({ kind: "not-allowed" });
    if (!c.remote) return setState(token() ? { kind: "ready" } : { kind: "no-token" });
    if (token()) return setState({ kind: "ready" });
    try {
      const r = await claim();
      if (r === "claimed") return setState({ kind: "ready" });
      setState((s) => {
        const expired = r === "refused" || (s.kind === "unpaired" && s.expired);
        return s.kind === "unpaired" && s.expired === expired ? s : { kind: "unpaired", expired };
      });
    } catch {
      setState({ kind: "absent" });
    }
  }, []);

  useEffect(() => {
    // On a phone or a PC there is no ebb to find, so the page asks nothing of 127.0.0.1.
    if (!wasEmbedded() && detectOS() !== "mac") setState({ kind: "absent" });
    else void attempt();
  }, [attempt]);

  useEffect(() => {
    if (state.kind === "ready" || state.kind === "connecting" || state.kind === "no-token") return;
    if (state.kind === "absent" && !wasEmbedded() && detectOS() !== "mac") return;
    let delay = RETRY_MS;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const tick = async () => {
      if (document.visibilityState === "visible") await attempt();
      if (stopped) return;
      delay = Math.min(delay * 2, MAX_RETRY_MS);
      timer = setTimeout(tick, delay);
    };
    const onVisible = () => {
      if (document.visibilityState !== "visible") return;
      clearTimeout(timer);
      delay = RETRY_MS;
      void tick();
    };
    timer = setTimeout(tick, delay);
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      stopped = true;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [state.kind, attempt]);

  useEffect(() => {
    if (state.kind === "ready") return;
    return onTokenFromOtherTab(() => void attempt());
  }, [state.kind, attempt]);

  useEffect(() => {
    if (state.kind !== "ready") return;
    const off = onAuthRejected(() => {
      if (!isRemote()) return setState({ kind: "no-token" });
      forgetToken();
      setState({ kind: "connecting" });
      void attempt();
    });
    const unlisten = listen(qc);
    return () => {
      off();
      unlisten();
    };
  }, [state.kind, qc, attempt]);

  if (state.kind === "ready") return <>{children}</>;

  const retry = async () => {
    setBusy(true);
    setApiPort(port);
    await attempt();
    setBusy(false);
  };
  const where = `127.0.0.1:${port}`;
  const pickPort = (p: number) => {
    setPort(p);
    if (p > 0 && p < 65536) setApiPort(p);
  };
  const version = running ? { label: versionLabel(running), tip: "The version of ebb running on this Mac" } : pageVersion ? { label: versionLabel(pageVersion), tip: "The ebb release this page comes from" } : undefined;
  const footer = <Footer port={port} setPort={pickPort} onRetry={retry} busy={busy} />;

  switch (state.kind) {
    case "connecting":
      return (
        <Gate version={version} title="Looking for ebb…">
          <p className="muted row" role="status">
            <Loader2 size={15} className="spin" aria-hidden /> Checking this Mac
          </p>
        </Gate>
      );
    case "no-token":
      return (
        <Gate version={version} title="Open ebb from its link" sub="This window has no access key. The link ebb opens carries one, which keeps other pages on this Mac from using ebb.">
          <Command value="ebb open" />
        </Gate>
      );
    case "not-allowed":
      return (
        <Gate version={version} title="Allow this page first" sub="ebb is running, but it only talks to pages you allow, so no other website can look at your disk. Run this on your Mac:">
          <Command value={`ebb web allow ${location.origin}`} />
          <Waiting where={where} text="Connects as soon as you allow it" />
          {footer}
        </Gate>
      );
    case "unpaired": {
      const behind = isBehind(running);
      return (
        <Gate version={version} title="Connect this page to ebb" sub="ebb is running on this Mac. Run this once in Terminal to let this page in. It stays connected after that.">
          <Command value={behind ? "brew upgrade ebb && ebb open" : "ebb open"} />
          {behind && <p className="faint small-text">The ebb running on this Mac is older than this page, so it needs an update first.</p>}
          {state.expired && <p className="faint small-text">The link you opened has expired or was used already. Running the command makes a new one.</p>}
          <Waiting where={where} text="Connects as soon as you run it" />
          {footer}
        </Gate>
      );
    }
    default:
      if (wasEmbedded() || pairedBefore())
        return (
          <Gate version={version} title="ebb is not running" sub="This page was connected to ebb on this Mac. Start it again and this page reconnects by itself.">
            <Command value="ebb open" />
            <Waiting where={where} text="Reconnects on its own" />
            {footer}
          </Gate>
        );
      return detectOS() === "mac" ? (
        <Gate version={version} title="Start ebb on this Mac" sub="This page works with ebb running on your own Mac. Your files and what ebb finds about them never leave it." intro>
          <InstallGuide />
          <Waiting where={where} text="Connects on its own" />
          {footer}
        </Gate>
      ) : (
        <Gate version={version} title="ebb runs on macOS" sub="It reads what only macOS knows about a disk: APFS clones and snapshots, purgeable space and the caches Xcode and simulators leave behind. Open this page on your Mac to get started." intro />
      );
  }
}

function InstallGuide() {
  return (
    <>
      <ol className="steps">
        <Step n={1} title="Install ebb">
          <Command value="brew install doguyilmaz/tap/ebb" />
          <p className="faint small-text">
            No Homebrew? Get it at{" "}
            <a href="https://brew.sh" target="_blank" rel="noreferrer">brew.sh</a> or download the <code>darwin</code> archive (one binary for Apple
            silicon and Intel) from the{" "}
            <a href={RELEASES} target="_blank" rel="noreferrer">latest release</a> and put <code>ebb</code> on your PATH.
          </p>
        </Step>
        <Step n={2} title="Open it">
          <Command value="ebb open" />
          <p className="faint small-text">
            Starts ebb, offers to keep it running in the background and opens this page connected. Until you approve a cleanup, ebb only reads.
          </p>
        </Step>
      </ol>
      {isSafari() && (
        <p className="gate-note small-text">
          Safari does not let web pages reach your own Mac. Use Chrome, Edge or Firefox for this page. When Safari is your default browser,{" "}
          <code>ebb open</code> opens ebb's own window, which works there.
        </p>
      )}
    </>
  );
}

function Step({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <li className="step">
      <span className="step-n" aria-hidden>{n}</span>
      <div className="stack" style={{ gap: 8, minWidth: 0, flex: 1 }}>
        <span className="step-title">
          {title}
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

type Version = { label: string; tip: string };

function Gate({ title, sub, intro, version, children }: { title: string; sub?: string; intro?: boolean; version?: Version; children?: ReactNode }) {
  const wide = intro && !!children;
  return (
    <main className="gate">
      <div className={wide ? "gate-wrap wide" : "gate-wrap"}>
        <div className="gate-brand">
          <img src="/favicon.svg" width={36} height={36} alt="" />
          <div className="stack" style={{ gap: 0 }}>
            <span className="gate-name">
              ebb
              {version && <span className="brand-version num" data-tip={version.tip}>{version.label}</span>}
            </span>
            <span className="faint small-text">Where your disk space went and how to get it back</span>
          </div>
          <span className="grow" />
          <a className="small-text row" style={{ gap: 4, flex: "none" }} href={SITE} target="_blank" rel="noreferrer">
            About ebb <ExternalLink size={12} aria-hidden />
          </a>
        </div>
        <div className="gate-card">
          <div className="gate-head">
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
          </div>
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
