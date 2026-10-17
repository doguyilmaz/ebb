import { useNavigate } from "@tanstack/react-router";
import { ChartPie, LayoutGrid, List, Rows3 } from "lucide-react";
import { useState } from "react";
import { useSlice, useStatus } from "#/api/hooks.ts";
import { exploreRoute, type View } from "#/router.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { Breadcrumb } from "./Breadcrumb.tsx";
import { Chart, type Focus } from "./canvas/Chart.tsx";
import { ListView } from "./ListView.tsx";
import { SidePanel } from "./SidePanel.tsx";
import { Search } from "./Search.tsx";

const views: { id: View; label: string; icon: typeof ChartPie }[] = [
  { id: "sunburst", label: "Sunburst", icon: ChartPie },
  { id: "treemap", label: "Treemap", icon: LayoutGrid },
  { id: "icicle", label: "Icicle", icon: Rows3 },
  { id: "list", label: "List", icon: List },
];

export default function Explore() {
  const { path, view, unique } = exploreRoute.useSearch();
  const navigate = useNavigate({ from: "/explore" });
  const status = useStatus();
  const depth = view === "treemap" ? 3 : 5;
  const slice = useSlice(path, depth, view === "treemap" ? 2500 : 3500);
  const [hover, setHover] = useState<Focus | null>(null);
  const [picked, setPicked] = useState<Focus | null>(null);
  const go = (next: { path?: string; view?: View; unique?: boolean }) =>
    navigate({ search: (s) => ({ ...s, ...next }), replace: next.path === undefined && next.view !== undefined });
  const open = (p: string | undefined) => {
    setPicked(null);
    go({ path: p });
  };

  if (slice.error) return <Problem error={slice.error} />;
  if (!slice.data?.root) return <Loading />;
  const here = slice.data.path;
  const scanned = status.data?.last?.root;
  const up = () => {
    const parent = here.slice(0, here.lastIndexOf("/")) || "/";
    open(!scanned || here === scanned || !here.startsWith("/") ? undefined : parent === "" ? undefined : parent);
  };
  const focus = hover ?? picked ?? { node: slice.data.root, path: here };

  return (
    <div className="explore">
      <div className="explore-bar">
        <Breadcrumb path={here} root={scanned} onOpen={open} />
        <Search onOpen={open} />
        <div className="seg" role="group" aria-label="Size">
          <button aria-pressed={!unique} onClick={() => go({ unique: undefined })}>Allocated</button>
          <button aria-pressed={!!unique} onClick={() => go({ unique: true })} title="Leaves out data shared with clones and hardlinks">
            Unique
          </button>
        </div>
        <div className="seg" role="group" aria-label="View">
          {views.map(({ id, label, icon: Icon }) => (
            <button key={id} aria-pressed={view === id} onClick={() => go({ view: id })} title={label}>
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
          <Chart
            view={view}
            root={slice.data.root}
            base={here}
            metric={unique ? "unique" : "alloc"}
            onHover={setHover}
            onOpen={open}
            onSelect={setPicked}
            onUp={up}
          />
        )}
        <SidePanel focus={focus} onOpen={open} isRoot={!hover && !picked} />
      </div>
    </div>
  );
}
