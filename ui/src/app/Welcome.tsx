import { Check, MessageSquareText, ScanSearch, ShieldCheck, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { useStatus, useStopScan } from "#/api/hooks.ts";
import { useScan } from "#/lib/fda.ts";
import { term } from "#/lib/glossary.ts";
import { bytes, count } from "#/lib/format.ts";
import { useLive } from "#/lib/live.ts";

const steps = [
  { icon: ScanSearch, title: "Scan", text: "ebb reads every folder once, in about a minute. It only reads: nothing is changed or uploaded." },
  { icon: MessageSquareText, title: "Understand", text: "Every large item says why it is there, how safe it is to remove and what removing it costs." },
  { icon: ShieldCheck, title: "Clean safely", text: "You review a plan before anything runs. Things go to the Trash, so they can be put back." },
];

/** The first screen: what ebb does and one button to begin; the scan asks for Full Disk Access first. */
export function Welcome() {
  const status = useStatus();
  const { run: scan, start } = useScan();
  const live = useLive((s) => s.scan);
  if (status.data?.scan.running || start.isPending || live) return <ScanProgress />;
  const st = status.data;
  return (
    <div className="welcome">
      <div className="stack" style={{ gap: 8 }}>
        <span className="welcome-kicker">Welcome to ebb</span>
        <h1 className="welcome-title">See where your disk space went and get it back safely</h1>
      </div>
      <ol className="welcome-steps">
        {steps.map(({ icon: Icon, title, text }, i) => (
          <li key={title} style={{ ["--i" as string]: i }}>
            <span className="welcome-n">
              <Icon size={18} aria-hidden />
            </span>
            <h2>{title}</h2>
            <p className="muted">{text}</p>
          </li>
        ))}
      </ol>
      <div className="welcome-go">
        <button className="btn primary big" disabled={start.isPending} onClick={() => scan({})}>
          <ScanSearch size={17} aria-hidden /> Scan this Mac
        </button>
        {st && (
          <span className="faint small-text row" style={{ gap: 6 }}>
            {st.fullDiskAccess && <Check size={14} aria-hidden />}
            {st.fullDiskAccess ? "Full Disk Access is on · " : ""}Reads {st.defaultRoot}
          </span>
        )}
      </div>
    </div>
  );
}

const tips = ["category.package-cache", "category.build-artifact", "category.simulator", "category.app-cache", "category.logs", "category.installer", "unique", "segment.hidden"];

/** While the first scan runs: live numbers, the folder being read, and what ebb will look for. */
export function ScanProgress() {
  const live = useLive((s) => s.scan);
  const stop = useStopScan();
  const [tip, setTip] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setTip((t) => (t + 1) % tips.length), 5000);
    return () => clearInterval(id);
  }, []);
  const t = term(tips[tip]);
  return (
    <div className="welcome scanning">
      <div className="stack" style={{ gap: 8 }}>
        <span className="welcome-kicker row" style={{ gap: 8 }}>
          <span className="pulse" aria-hidden /> Reading your disk
        </span>
        <h1 className="welcome-title">This takes about a minute</h1>
        <p className="muted">ebb reads every folder once. Nothing is changed or uploaded.</p>
      </div>
      <dl className="readout" aria-live="off">
        <div>
          <dt>Files</dt>
          <dd className="num">{count(live?.files ?? 0)}</dd>
        </div>
        <div>
          <dt>Folders</dt>
          <dd className="num">{count(live?.dirs ?? 0)}</dd>
        </div>
        <div>
          <dt>Read so far</dt>
          <dd className="num">{bytes(live?.bytes ?? 0)}</dd>
        </div>
        <div>
          <dt>Time</dt>
          <dd className="num">{Math.round((live?.elapsedMs ?? 0) / 1000)} s</dd>
        </div>
      </dl>
      <div className="stack" style={{ gap: 8 }}>
        <div className="progress">
          <i />
        </div>
        <code className="faint ellipsis readout-path">{live?.path || "Starting…"}</code>
      </div>
      {t && (
        <div className="welcome-tip">
          <span className="panel-label" key={`h${tip}`}>While you wait · {t.title}</span>
          <p className="muted" key={`b${tip}`}>{t.body}</p>
        </div>
      )}
      <div>
        <button className="btn" disabled={stop.isPending} onClick={() => stop.mutate()}>
          <Square size={11} aria-hidden fill="currentColor" /> Stop
        </button>
      </div>
    </div>
  );
}
