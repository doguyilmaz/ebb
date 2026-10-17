import { Link, useLocation } from "@tanstack/react-router";
import { Activity, Compass, Gauge, ListChecks, RefreshCw, Search, Settings, Square } from "lucide-react";
import type { ReactNode } from "react";
import { useStartScan, useStatus, useStopScan } from "#/api/hooks.ts";
import { ago, bytes, count, percent } from "#/lib/format.ts";
import { useLive } from "#/lib/live.ts";
import { usePalette } from "#/lib/palette.ts";
import { SelectionTray } from "./SelectionTray.tsx";

const nav = [
  { to: "/", label: "Overview", icon: Gauge },
  { to: "/explore", label: "Explore", icon: Compass },
  { to: "/findings", label: "Findings", icon: ListChecks },
  { to: "/activity", label: "Activity", icon: Activity },
  { to: "/settings", label: "Settings", icon: Settings },
] as const;

export function Shell({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const wide = pathname.startsWith("/explore");
  return (
    <div className="shell">
      <aside className="side">
        <div className="brand">
          <img src="/favicon.svg" width={26} height={26} alt="" />
          ebb
        </div>
        <nav className="stack" style={{ gap: 2 }} aria-label="Pages">
          {nav.map(({ to, label, icon: Icon }) => (
            <Link key={to} to={to} className="nav" activeOptions={{ exact: to === "/" }}>
              <Icon size={16} aria-hidden />
              {label}
            </Link>
          ))}
        </nav>
        <DiskMeter />
      </aside>
      <div className="main">
        <TopBar title={nav.find((n) => (n.to === "/" ? pathname === "/" : pathname.startsWith(n.to)))?.label ?? ""} />
        <div className="content">
          <div className={wide ? "page wide" : "page"}>{children}</div>
        </div>
        <SelectionTray />
      </div>
    </div>
  );
}

/** How full the disk is, always in view. The status colour appears only when space is actually low. */
function DiskMeter() {
  const status = useStatus();
  const last = status.data?.last;
  if (!last) return null;
  const total = Math.max(1, last.used + last.free);
  const used = last.used / total;
  const low = last.free / total < 0.1;
  return (
    <div className="disk-meter" data-low={low || undefined}>
      <div className="row between small-text">
        <span className="medium">This Mac</span>
        <span className="num faint">{percent(used)} full</span>
      </div>
      <div className="meter" role="img" aria-label={`${percent(used)} of the disk used, ${bytes(last.free)} free`}>
        <i style={{ transform: `scaleX(${used})` }} />
      </div>
      <span className="faint small-text num">
        {low ? <span className="low">Low on space · </span> : null}
        {bytes(last.free)} free
      </span>
    </div>
  );
}

function TopBar({ title }: { title: string }) {
  const status = useStatus();
  const start = useStartScan();
  const stop = useStopScan();
  const live = useLive((s) => s.scan);
  const openPalette = usePalette((s) => s.setOpen);
  const running = status.data?.scan.running || start.isPending || !!live;
  const last = status.data?.last;
  // A rescan reads about as many files as the last one did; that is the best estimate there is.
  const share = live && last?.files ? Math.min(0.99, live.files / last.files) : null;
  return (
    <header className="topbar">
      <span className="topbar-title">{title}</span>
      {running ? (
        <span className="scan-pill" data-state="running" style={share !== null ? { ["--share" as string]: `${share * 100}%` } : undefined} role="status">
          <span className="pulse" aria-hidden />
          <span className="num">
            Scanning{live ? ` · ${count(live.files)} files` : "…"}
            {share !== null && <span className="faint"> · about {Math.round(share * 100)}%</span>}
          </span>
          <button className="btn small ghost" disabled={stop.isPending} onClick={() => stop.mutate()} data-tip="Stops reading. Your last scan stays as it was.">
            <Square size={11} aria-hidden fill="currentColor" /> Stop
          </button>
        </span>
      ) : last ? (
        <span className="scan-pill" data-tip={`${last.root} · ${count(last.files)} files in ${Math.round(last.elapsed / 1e9)} s`}>
          Scanned {ago(last.time)}
        </span>
      ) : null}
      <span className="grow" />
      <button className="btn topbar-search" onClick={() => openPalette(true)}>
        <Search size={14} aria-hidden />
        <span>Go to, find a folder, clean…</span>
        <kbd>⌘K</kbd>
      </button>
      {!running && (
        <button className="btn primary" onClick={() => start.mutate({})}>
          <RefreshCw size={14} aria-hidden />
          {last ? "Scan again" : "Scan this Mac"}
        </button>
      )}
    </header>
  );
}
