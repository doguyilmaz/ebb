import { useState } from "react";
import { usePackages, useSources } from "#/api/hooks.ts";
import { bytes } from "#/lib/format.ts";
import { readPrefs, savePrefs } from "#/lib/prefs.ts";
import { PageHead } from "#/ui/PageHead.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { FlowChart, kindLabel } from "./FlowChart.tsx";
import { Packages } from "./Packages.tsx";

type Tab = "flows" | "packages";

export default function Sources() {
  const [tab, setTab] = useState<Tab>(() => readPrefs().sourcesTab ?? "flows");
  const flows = useSources();
  const pkgs = usePackages(tab === "packages");
  const pick = (t: Tab) => (setTab(t), savePrefs({ sourcesTab: t }));
  return (
    <>
      <PageHead title="Sources" sub="Where the space comes from: which folders hold it, which tool or app put it there, and what kind it is.">
        <div className="seg" role="tablist" aria-label="Show">
          <button role="tab" aria-selected={tab === "flows"} aria-pressed={tab === "flows"} onClick={() => pick("flows")}>Flows</button>
          <button role="tab" aria-selected={tab === "packages"} aria-pressed={tab === "packages"} onClick={() => pick("packages")}>Packages</button>
        </div>
      </PageHead>
      {tab === "flows" ? (
        flows.error ? (
          <Problem error={flows.error} />
        ) : !flows.data ? (
          <Loading />
        ) : (
          <>
            <section className="card flows-card" data-tour="flows">
              <FlowChart flows={flows.data} />
            </section>
            <details className="card flows-table">
              <summary>As a table</summary>
              <table>
                <thead>
                  <tr>
                    <th>Owner</th>
                    <th>Kind</th>
                    <th className="num">Size</th>
                  </tr>
                </thead>
                <tbody>
                  {(flows.data.links ?? [])
                    .filter((l) => l.to.startsWith("k:"))
                    .map((l) => (
                      <tr key={l.from + l.to}>
                        <td>{l.from.slice(2)}</td>
                        <td>{kindLabel(l.to.slice(2))}</td>
                        <td className="num">{bytes(l.bytes)}</td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </details>
          </>
        )
      ) : pkgs.error ? (
        <Problem error={pkgs.error} />
      ) : !pkgs.data ? (
        <Loading label="Reading the projects' packages…" />
      ) : (
        <Packages data={pkgs.data} />
      )}
    </>
  );
}
