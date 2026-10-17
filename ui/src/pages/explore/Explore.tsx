import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { ChartPie, LayoutGrid, List, Rows3 } from "lucide-react";
import { useState } from "react";
import { sliceQuery, useSlice, useStatus, useVolume } from "#/api/hooks.ts";
import { baseName } from "#/lib/format.ts";
import { exploreRoute, type View } from "#/router.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { Info } from "#/ui/Tip.tsx";
import { Breadcrumb } from "./Breadcrumb.tsx";
import { Chart } from "./canvas/Chart.tsx";
import { ListView } from "./ListView.tsx";
import { Search } from "./Search.tsx";
import { SidePanel, type Focus } from "./SidePanel.tsx";

const views: { id: View; label: string; icon: typeof ChartPie; hint: string }[] = [
  { id: "sunburst", label: "Sunburst", icon: ChartPie, hint: "Click a slice to open it, the middle to go back" },
  { id: "treemap", label: "Treemap", icon: LayoutGrid, hint: "Click a box to open it" },
  { id: "icicle", label: "Icicle", icon: Rows3, hint: "Click a column to open it" },
  { id: "list", label: "List", icon: List, hint: "" },
];

const depthFor = (v: View) => (v === "treemap" ? 3 : 5);
const budgetFor = (v: View) => (v === "treemap" ? 2500 : 3500);

export default function Explore() {
  const { path, view, unique } = exploreRoute.useSearch();
  const navigate = useNavigate({ from: "/explore" });
  const qc = useQueryClient();
  const status = useStatus();
  const volume = useVolume();
  const slice = useSlice(path, depthFor(view), budgetFor(view));
  const [hoverKey, setHoverKey] = useState<string | null>(null);
  const [picked, setPicked] = useState<Focus | null>(null);
  const go = (next: { path?: string; view?: View; unique?: boolean }) =>
    navigate({ search: (s) => ({ ...s, ...next }), replace: next.path === undefined && next.view !== undefined });
  const open = (p: string | undefined) => {
    setPicked(null);
    setHoverKey(null);
    go({ path: p });
  };

  if (slice.error) return <Problem error={slice.error} />;
  if (!slice.data?.root) return <Loading label="Reading the scan…" />;
  const here = slice.data.path;
  const scanned = status.data?.last?.root;
  const atTop = !scanned || here === scanned || !here.startsWith("/");
  const parent = here.slice(0, here.lastIndexOf("/")) || "/";
  const up = () => open(atTop ? undefined : parent);
  const metric = unique ? "unique" : "alloc";
  const current = views.find((v) => v.id === view)!;
  const prefetch = (p: string) => view !== "list" && qc.prefetchQuery(sliceQuery(p, depthFor(view), budgetFor(view)));

  return (
    <div className="explore">
      <div className="explore-bar">
        <Breadcrumb path={here} root={scanned} onOpen={open} />
        <Search onOpen={open} />
        <div className="row" style={{ gap: 4 }} data-tour="metric">
          <div className="seg" role="group" aria-label="Size">
            <button aria-pressed={!unique} onClick={() => go({ unique: undefined })}>Allocated</button>
            <button aria-pressed={!!unique} onClick={() => go({ unique: true })}>Unique</button>
          </div>
          <Info term={unique ? "unique" : "allocated"} />
        </div>
        <div className="row" style={{ gap: 4 }} data-tour="views">
          <div className="seg" role="group" aria-label="View">
            {views.map(({ id, label, icon: Icon }) => (
              <button key={id} aria-pressed={view === id} onClick={() => go({ view: id })} aria-label={label} data-term={`view.${id}`}>
                <Icon size={15} aria-hidden />
                <span className="seg-label">{label}</span>
              </button>
            ))}
          </div>
        </div>
      </div>
      <div className="explore-body">
        {view === "list" ? (
          <ListView path={here} onOpen={open} onPick={setPicked} />
        ) : (
          <div className="chart-card">
            <Chart
              view={view}
              root={slice.data.root}
              base={here}
              metric={metric}
              upLabel={atTop ? undefined : baseName(parent) || parent}
              disk={volume.data?.capacity}
              hoverKey={hoverKey}
              stale={slice.isPlaceholderData}
              onHover={(s) => setHoverKey(s?.key ?? null)}
              onOpen={open}
              onPick={(s) => setPicked({ node: s.node, path: s.path })}
              onUp={up}
              onPrefetch={prefetch}
            />
            <p className="chart-hint faint">
              {current.hint}
              <Info term={`view.${view}` as "view.sunburst"} />
            </p>
          </div>
        )}
        <SidePanel
          root={slice.data.root}
          base={here}
          metric={metric}
          picked={picked}
          hoverKey={hoverKey}
          onHover={setHoverKey}
          onOpen={open}
          onPick={setPicked}
          onPrefetch={prefetch}
        />
      </div>
    </div>
  );
}
