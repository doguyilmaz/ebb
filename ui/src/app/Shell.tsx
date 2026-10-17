import { Link } from "@tanstack/react-router";
import { Activity, Compass, Gauge, ListChecks, RefreshCw, Settings } from "lucide-react";
import type { ReactNode } from "react";
import { useStartScan, useStatus } from "#/api/hooks.ts";
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
  const live = useLive((s) => s.scan);
  const running = status.data?.scan.running || start.isPending || !!live;
  return (
    <header className="topbar">
      <div className="grow">
        {running ? (
          <div className="stack" style={{ gap: 6 }}>
            <span className="muted num">
              Scanning {live ? `${count(live.files)} files · ${bytes(live.bytes)}` : "…"}
            </span>
            <div className="progress">
              <i />
            </div>
          </div>
        ) : status.data?.last ? (
          <span className="muted mono">{status.data.last.root}</span>
        ) : (
          <span className="muted">Nothing scanned yet</span>
        )}
      </div>
      <kbd className="faint kbd" title="Command palette">⌘K</kbd>
      <button className="btn primary" disabled={running} onClick={() => start.mutate({})}>
        <RefreshCw size={15} aria-hidden />
        {status.data?.last ? "Scan again" : "Scan this Mac"}
      </button>
    </header>
  );
}
