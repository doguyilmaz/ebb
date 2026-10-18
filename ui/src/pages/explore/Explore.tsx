import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { ChartPie, LayoutGrid, List, Rows3 } from "lucide-react";
import { useMemo, useState } from "react";
import { sliceQuery, useAges, useSlice, useStatus, useVolume } from "#/api/hooks.ts";
import { ageColor } from "#/lib/ages.ts";
import { readPrefs, savePrefs } from "#/lib/prefs.ts";
import { budgetFor, depthFor } from "./params.ts";
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


export default function Explore() {
  const search = exploreRoute.useSearch();
  const [prefs] = useState(readPrefs);
  const { path } = search;
  const view = search.view ?? prefs.view ?? "sunburst";
  const unique = search.unique ?? prefs.unique ?? false;
  const color = search.color ?? prefs.color ?? "folder";
  const navigate = useNavigate({ from: "/explore" });
  const qc = useQueryClient();
  const status = useStatus();
  const volume = useVolume();
  const slice = useSlice(path, depthFor(view), budgetFor(view));
  const ages = useAges(slice.data?.path);
  const buckets = ages.data?.buckets;
  const cuts = useMemo(() => (buckets ?? []).filter((b) => b.days > 0).map((b) => b.days), [buckets]);
  const [hoverKey, setHoverKey] = useState<string | null>(null);
  const [picked, setPicked] = useState<Focus | null>(null);
  const go = (next: { path?: string; view?: View; unique?: boolean; color?: "age" | "folder" }) =>
    navigate({ search: (s) => ({ ...s, ...next }), replace: next.path === undefined && next.view !== undefined });
  const choose = (next: { view?: View; unique?: boolean; color?: "age" | "folder" }) => {
    savePrefs(next);
    go(next);
  };
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
      <Breadcrumb path={here} root={scanned} onOpen={open} />
      <div className="explore-bar">
        <Search onOpen={open} />
        <span className="grow" />
        <div className="row" style={{ gap: 2 }} data-tour="metric">
          <div className="seg" role="group" aria-label="Size">
            <button aria-pressed={!unique} onClick={() => choose({ unique: false })} data-term="allocated">Allocated</button>
            <button aria-pressed={unique} onClick={() => choose({ unique: true })} data-term="unique">Unique</button>
          </div>
        </div>
        {view !== "list" && (
          <div className="seg" role="group" aria-label="Colour">
            <button aria-pressed={color === "folder"} onClick={() => choose({ color: "folder" })} data-term="color.folder">Folders</button>
            <button aria-pressed={color === "age"} onClick={() => choose({ color: "age" })} data-term="color.age">Age</button>
          </div>
        )}
        <div className="seg" role="group" aria-label="View" data-tour="views">
          {views.map(({ id, label, icon: Icon }) => (
            <button key={id} aria-pressed={view === id} onClick={() => choose({ view: id })} aria-label={label} data-term={`view.${id}`}>
              <Icon size={15} aria-hidden />
              <span className="seg-label">{label}</span>
            </button>
          ))}
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
              color={color}
              now={status.data?.last ? Math.floor(new Date(status.data.last.time).getTime() / 1000) : Math.floor(Date.now() / 1000)}
              cuts={cuts}
              hoverKey={hoverKey}
              stale={slice.isPlaceholderData}
              onHover={(s) => setHoverKey(s?.key ?? null)}
              onOpen={open}
              onPick={(s) => setPicked({ node: s.node, path: s.path })}
              onUp={up}
              onPrefetch={prefetch}
            />
            <div className="chart-hint faint">
              {current.hint}
              <Info term={`view.${view}` as "view.sunburst"} />
              {color === "age" && buckets && (
                <span className="age-key" aria-label="Colour key">
                  Changed
                  {buckets.map((b, i) => (
                    <i key={b.label} style={{ background: ageColor(i, buckets.length) }} data-tip={b.label} />
                  ))}
                  long ago
                </span>
              )}
            </div>
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
