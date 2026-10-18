import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { useState } from "react";
import type { Package, Packages as P } from "#/api/types.gen.ts";
import { baseName, bytes, count } from "#/lib/format.ts";

/** Which packages fill the projects' node_modules, and which versions Bun's cache keeps. */
export function Packages({ data }: { data: P }) {
  const [query, setQuery] = useState("");
  const q = query.trim().toLowerCase();
  const pkgs = (data.packages ?? []).filter((p) => !q || p.name.toLowerCase().includes(q));
  const cache = (data.cache ?? []).filter((c) => !q || c.name.toLowerCase().includes(q));
  const max = Math.max(1, ...pkgs.map((p) => p.bytes));
  return (
    <>
      <div className="row">
        <input className="input" type="search" placeholder="Filter packages" value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Filter packages" />
        <span className="muted small-text">
          {data.projects === 1 ? "1 project" : `${count(data.projects)} projects`} with node_modules · {bytes(data.nodeModules)}
          {data.cacheBytes > 0 && ` · Bun cache ${bytes(data.cacheBytes)}`}
        </span>
      </div>
      {data.projects === 0 ? (
        <p className="empty card">No project with a node_modules folder in your home folder.</p>
      ) : (
        <section className="card pkgs">
          <div className="pkg-head small-text faint">
            <span />
            <span>Package</span>
            <span>Copies</span>
            <span>Versions</span>
            <span>In all projects</span>
          </div>
          <ul>
            {pkgs.slice(0, 120).map((p) => (
              <Row key={p.name} p={p} max={max} />
            ))}
          </ul>
          {pkgs.length === 0 && <p className="empty">No package matches “{query}”.</p>}
        </section>
      )}
      {cache.length > 0 && (
        <section className="card pkgs">
          <div className="card-head pad">
            <h2>Bun keeps</h2>
            <span className="faint small-text">every version it ever installed, for every project</span>
          </div>
          <ul className="cache-list">
            {cache.slice(0, 60).map((c) => (
              <li key={c.name}>
                <span className="mono ellipsis">{c.name}</span>
                <span className="faint small-text ellipsis">{(c.versions ?? []).length} {(c.versions ?? []).length === 1 ? "version" : "versions"}: {(c.versions ?? []).slice(-4).join(", ")}{(c.versions ?? []).length > 4 ? " …" : ""}</span>
                <span className="num">{bytes(c.bytes)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </>
  );
}

function Row({ p, max }: { p: Package; max: number }) {
  const [open, setOpen] = useState(false);
  const versions = p.versions ?? [];
  return (
    <li className="pkg" data-open={open}>
      <button className="pkg-main" onClick={() => setOpen(!open)} aria-expanded={open}>
        <ChevronRight size={14} className="chev" aria-hidden />
        <span className="mono ellipsis">{p.name}</span>
        <span className="num">{p.copies}×</span>
        <span className="ellipsis faint small-text" title={versions.join(", ")}>
          {versions.slice(0, 3).join(", ")}
          {versions.length > 3 ? ` +${versions.length - 3}` : ""}
        </span>
        <span className="pkg-size num">
          <i style={{ width: `${(p.bytes / max) * 100}%` }} />
          {bytes(p.bytes)}
        </span>
      </button>
      {open && (
        <ul className="pkg-projects">
          {(p.projects ?? []).map((proj) => (
            <li key={proj}>
              <Link to="/explore" search={{ path: `${proj}/node_modules` }} className="ellipsis" title={proj}>
                {baseName(proj)} <span className="faint mono small-text">{proj}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}
