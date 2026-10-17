import { Link } from "@tanstack/react-router";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useLatestDiff } from "#/api/hooks.ts";
import { ago, baseName, bytes } from "#/lib/format.ts";

/** What grew and what shrank since the scan before: growth is how a disk fills up unnoticed. */
export function Changes() {
  const diff = useLatestDiff();
  const d = diff.data;
  const rows = (d?.changes ?? []).filter((c) => c.path !== d?.to.root && c.delta !== 0).slice(0, 6);
  const max = Math.max(1, ...rows.map((c) => Math.abs(c.delta)));
  return (
    <section className="card">
      <div className="card-head">
        <h2>Changed since the scan before</h2>
      </div>
      {!d ? (
        <p className="muted">{diff.isLoading ? "Comparing…" : "After your next scan, this shows what grew and what shrank."}</p>
      ) : rows.length === 0 ? (
        <p className="muted">Nothing changed by more than 10 MB since {ago(d.from.time)}.</p>
      ) : (
        <>
          <p className="card-sub">Compared with the scan {ago(d.from.time)}.</p>
          <ol className="changes">
            {rows.map((c) => {
              const up = c.delta > 0;
              return (
                <li key={c.path}>
                  <Link to="/explore" search={{ path: c.path }} className="change" data-tip={c.path} data-value={`${bytes(c.before)} → ${bytes(c.after)}`}>
                    <span className="ellipsis">{baseName(c.path)}</span>
                    <span className="change-track" aria-hidden>
                      <i data-up={up || undefined} style={{ width: `${(Math.abs(c.delta) / max) * 100}%` }} />
                    </span>
                    <span className="change-value num" data-up={up || undefined}>
                      {up ? <ArrowUp size={12} aria-hidden /> : <ArrowDown size={12} aria-hidden />}
                      {bytes(Math.abs(c.delta))}
                    </span>
                  </Link>
                </li>
              );
            })}
          </ol>
        </>
      )}
    </section>
  );
}
