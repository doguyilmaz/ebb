import { Link, useLocation } from "@tanstack/react-router";
import { Activity, AppWindow, ChartLine, CircleHelp, Compass, Gauge, ListChecks, RefreshCw, Search, Settings, Square } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";
import { useStartScan, useStatus, useStopScan } from "#/api/hooks.ts";
import { ago, bytes, count, percent } from "#/lib/format.ts";
import { useLive } from "#/lib/live.ts";
import { usePalette } from "#/lib/palette.ts";
import { SelectionTray } from "./SelectionTray.tsx";
import { tourSeen, useTour } from "./tour.ts";

let offered = false;

const nav = [
  { to: "/", label: "Overview", icon: Gauge },
  { to: "/explore", label: "Explore", icon: Compass },
  { to: "/findings", label: "Findings", icon: ListChecks },
  { to: "/apps", label: "Apps", icon: AppWindow },
  { to: "/history", label: "History", icon: ChartLine },
  { to: "/activity", label: "Activity", icon: Activity },
  { to: "/settings", label: "Settings", icon: Settings },
] as const;

export function Shell({ children }: { children: ReactNode }) {
  const { pathname, hash } = useLocation();
  const wide = pathname.startsWith("/explore");
  const content = useRef<HTMLDivElement>(null);
  // A new page starts at its top; one opened at an anchor scrolls there itself.
  useEffect(() => {
    if (!hash && content.current) content.current.scrollTop = 0;
  }, [pathname]);
  const hasScan = !!useStatus().data?.last;
  const startTour = useTour((s) => s.start);
  // The tour starts by itself once, on the overview, as soon as there is a scan to show; it never pulls
  // someone away from the page they opened.
  useEffect(() => {
    if (hasScan && pathname === "/" && !offered && !tourSeen()) {
      offered = true;
      startTour();
    }
  }, [hasScan, pathname, startTour]);
  return (
    <div className="shell">
      <aside className="side">
        <Link to="/" className="brand" aria-label="ebb, go to the overview">
          <img src="/favicon.svg" width={26} height={26} alt="" />
          ebb
        </Link>
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
        <div className="content" ref={content}>
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
  const startTour = useTour((s) => s.start);
  const running = status.data?.scan.running || start.isPending || !!live;
  const last = status.data?.last;
  // A rescan reads about as many files as the last one did; that is the best estimate there is.
  const share = live && last?.files && !(status.data?.changes?.ready && !status.data.changes.lost) ? Math.min(0.99, live.files / last.files) : null;
  const ch = status.data?.changes;
  const changed = ch && ch.ready && !ch.lost ? ch.folders : 0;
  const quick = !!ch && ch.ready && !ch.lost;
  return (
    <header className="topbar">
      <span className="topbar-title">{title}</span>
      {running ? (
        <span className="scan-pill" data-state="running" role="status" aria-label={live ? `Scanning, ${count(live.files)} files read` : "Scanning"}>
          <span className="pulse" aria-hidden />
          <span>Scanning</span>
          <span className="pill-count num">{count(live?.files ?? 0)}</span>
          <span className="faint">files</span>
          <span className={`pill-track${share === null ? "" : " known"}`} aria-hidden>
            <i style={share === null ? undefined : { transform: `scaleX(${share})` }} />
          </span>
          <span className="pill-pct num faint">{share === null ? "" : `${Math.round(share * 100)}%`}</span>
          <button className="btn small ghost" disabled={stop.isPending} onClick={() => stop.mutate()} data-tip="Stops reading. Your last scan stays as it was.">
            <Square size={10} aria-hidden fill="currentColor" /> Stop
          </button>
        </span>
      ) : last ? (
        <span className="scan-pill" data-tip={`${last.root} · ${count(last.files)} files${last.incremental ? `, updated in ${(last.elapsed / 1e9).toFixed(1)} s` : ` in ${Math.round(last.elapsed / 1e9)} s`}`}>
          {last.incremental ? "Updated" : "Scanned"} {ago(last.time)}
          {changed > 0 && <span className="faint num"> · {count(changed)} {changed === 1 ? "folder" : "folders"} changed</span>}
        </span>
      ) : null}
      <span className="grow" />
      <button className="btn topbar-search" onClick={() => openPalette(true)}>
        <Search size={14} aria-hidden />
        <span>Go to, find a folder, clean…</span>
        <kbd>⌘K</kbd>
      </button>
      <button className="icon-btn" onClick={startTour} aria-label="Take the tour" data-tip="Take the tour" data-tour="help">
        <CircleHelp size={17} />
      </button>
      {!running && (
        <button
          className="btn primary"
          onClick={() => start.mutate({})}
          data-tip={last ? (quick ? "Reads again only the folders that changed since the last scan, usually in seconds. ⌘K has “Scan everything again”." : "Reads every folder; ebb does not know yet what changed.") : undefined}
        >
          <RefreshCw size={14} aria-hidden />
          {last ? "Scan again" : "Scan this Mac"}
        </button>
      )}
    </header>
  );
}
