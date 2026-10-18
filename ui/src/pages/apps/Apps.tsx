import { ChevronRight, FolderSearch, Trash2 } from "lucide-react";
import { useState } from "react";
import { useApps, useReveal } from "#/api/hooks.ts";
import type { App } from "#/api/types.gen.ts";
import { bytes } from "#/lib/format.ts";
import { readPrefs, savePrefs } from "#/lib/prefs.ts";
import { useSheet } from "#/lib/sheet.ts";
import { PageHead } from "#/ui/PageHead.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { sortApps, type AppSort } from "./sort.ts";

const library = (p: string) => p.replace(/^.*?\/Library\//, "");

export default function Apps() {
  const apps = useApps();
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<AppSort>(readPrefs().appSort ?? "total");
  if (apps.error) return <Problem error={apps.error} />;
  if (!apps.data) return <Loading />;
  const all = apps.data.apps ?? [];
  const list = sortApps(all, sort, query);
  const inApps = all.reduce((s, a) => s + a.bytes, 0);
  const inLibrary = all.reduce((s, a) => s + a.leftoverBytes, 0);
  const scale = Math.max(1, ...all.map((a) => a.bytes + a.leftoverBytes));
  const pick = (s: AppSort) => (setSort(s), savePrefs({ appSort: s }));
  return (
    <>
      <PageHead title="Apps" sub="Every app on this Mac with what it keeps in your Library. Uninstalling moves the app and all of that to the Trash.">
        <input className="input" type="search" placeholder="Filter apps" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Filter apps" />
        <div className="seg" role="group" aria-label="Sort">
          {(["total", "library", "name"] as const).map((s) => (
            <button key={s} aria-pressed={sort === s} onClick={() => pick(s)}>
              {s === "total" ? "Size" : s === "library" ? "Library data" : "Name"}
            </button>
          ))}
        </div>
      </PageHead>
      {apps.data.note && <p className="empty card">{apps.data.note}</p>}
      {all.length > 0 && (
        <p className="apps-sum muted">
          {all.length === 1 ? "1 app" : `${all.length} apps`} · <span className="key app" /> {bytes(inApps)} in the apps · <span className="key lib" /> {bytes(inLibrary)} in their Library data
        </p>
      )}
      {list.length > 0 ? (
        <ul className="apps card">
          {list.map((a) => (
            <Row key={a.path} app={a} scale={scale} />
          ))}
        </ul>
      ) : (
        all.length > 0 && <p className="empty card">No app matches “{query}”.</p>
      )}
    </>
  );
}

function Row({ app, scale }: { app: App; scale: number }) {
  const [open, setOpen] = useState(false);
  const openSheet = useSheet((s) => s.open);
  const reveal = useReveal();
  const leftovers = app.leftovers ?? [];
  const why = app.apple ? "Comes with macOS or from Apple: ebb leaves it alone." : undefined;
  return (
    <li className="app-row" data-open={open}>
      <button className="app-main" onClick={() => setOpen(!open)} aria-expanded={open} disabled={leftovers.length === 0}>
        <ChevronRight size={14} className="chev" aria-hidden />
        <span className="app-mark" aria-hidden>{app.name.slice(0, 1).toUpperCase()}</span>
        <span className="app-name">
          <span className="ellipsis">{app.name}</span>
          <span className="faint mono ellipsis small-text">{app.bundleId}{app.version ? ` · ${app.version}` : ""}</span>
        </span>
        <span className="app-bar" aria-hidden>
          <span className="app" style={{ width: `${(app.bytes / scale) * 100}%` }} />
          <span className="lib" style={{ width: `${(app.leftoverBytes / scale) * 100}%` }} />
        </span>
        <span className="num" data-tip={`App ${bytes(app.bytes)} · Library data ${bytes(app.leftoverBytes)}`}>{bytes(app.bytes + app.leftoverBytes)}</span>
      </button>
      <button
        className="btn small"
        disabled={app.apple}
        data-tip={why ?? (app.rootOwned ? "Installed for every user: ebb trashes its Library data and shows how to remove the app in Finder." : "Shows exactly what goes before anything moves.")}
        onClick={() => openSheet({ app: app.bundleId, source: "ui" })}
      >
        <Trash2 size={13} aria-hidden /> Uninstall
      </button>
      {open && (
        <ul className="app-leftovers">
          {leftovers.map((t) => (
            <li key={t.path}>
              <span className="mono ellipsis" title={t.path}>{library(t.path)}</span>
              <span className="num faint">{bytes(t.bytes)}</span>
              <button className="icon-btn" aria-label={`Show ${library(t.path)} in Finder`} data-tip="Show in Finder" onClick={() => reveal.mutate(t.path)}>
                <FolderSearch size={14} />
              </button>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}
