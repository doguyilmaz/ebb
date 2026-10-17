import { Link } from "@tanstack/react-router";
import { Activity, Compass, Gauge, ListChecks, RefreshCw, Settings, Square } from "lucide-react";
import type { ReactNode } from "react";
import { useStartScan, useStatus, useStopScan } from "#/api/hooks.ts";
import { ago, bytes, count } from "#/lib/format.ts";
import { useLive } from "#/lib/live.ts";
import { SelectionTray } from "./SelectionTray.tsx";

const nav = [
  { to: "/", label: "Overview", icon: Gauge },
  { to: "/explore", label: "Explore", icon: Compass },
  { to: "/findings", label: "Findings", icon: ListChecks },
  { to: "/activity", label: "Activity", icon: Activity },
  { to: "/settings", label: "Settings", icon: Settings },
] as const;

export function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="shell">
      <aside className="side">
        <div className="brand">
          <img src="/favicon.svg" width={24} height={24} alt="" />
          ebb
        </div>
        {nav.map(({ to, label, icon: Icon }) => (
          <Link key={to} to={to} className="nav" activeOptions={{ exact: to === "/" }}>
            <Icon size={16} aria-hidden />
            {label}
          </Link>
        ))}
        <SideFoot />
      </aside>
      <div className="main">
        <TopBar />
        <div className="page">{children}</div>
        <SelectionTray />
      </div>
    </div>
  );
}

function SideFoot() {
  const status = useStatus();
  const last = status.data?.last;
  if (!last) return null;
  const used = last.used / Math.max(1, last.used + last.free);
  return (
    <div className="side-foot">
      <div className="waterline" title={`${bytes(last.free)} free`}>
        <i style={{ height: `${Math.round(used * 100)}%` }} />
      </div>
      <span className="muted">{bytes(last.free)} free</span>
      <span className="faint">scanned {ago(last.time)}</span>
    </div>
  );
}

function TopBar() {
  const status = useStatus();
  const start = useStartScan();
  const stop = useStopScan();
  const live = useLive((s) => s.scan);
  const running = status.data?.scan.running || start.isPending || !!live;
  const last = status.data?.last;
  // A rescan reads about as many files as the last one did; that is the best estimate there is.
  const share = live && last?.files ? Math.min(0.99, live.files / last.files) : null;
  return (
    <header className="topbar">
      <div className="grow">
        {running ? (
          <div className="stack" style={{ gap: 6 }}>
            <span className="muted num">
              Scanning {live ? `${count(live.files)} files · ${bytes(live.bytes)} · ${Math.round(live.elapsedMs / 1000)} s` : "…"}
              {share !== null && <span className="faint"> · about {Math.round(share * 100)}%</span>}
            </span>
            <div className={`progress${share !== null ? " known" : ""}`} role="progressbar" aria-valuenow={share !== null ? Math.round(share * 100) : undefined}>
              <i style={share !== null ? { transform: `scaleX(${share})` } : undefined} />
            </div>
          </div>
        ) : status.data?.last ? (
          <span className="muted mono">{status.data.last.root}</span>
        ) : (
          <span className="muted">Nothing scanned yet</span>
        )}
      </div>
      <kbd className="faint kbd" title="Command palette">⌘K</kbd>
      {running ? (
        <button className="btn" disabled={stop.isPending} onClick={() => stop.mutate()} data-tip="Stops reading. Your last scan stays as it was.">
          <Square size={13} aria-hidden fill="currentColor" />
          Stop
        </button>
      ) : (
        <button className="btn primary" onClick={() => start.mutate({})}>
          <RefreshCw size={15} aria-hidden />
          {status.data?.last ? "Scan again" : "Scan this Mac"}
        </button>
      )}
    </header>
  );
}
