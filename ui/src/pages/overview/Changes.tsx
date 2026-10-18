import { Link } from "@tanstack/react-router";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useState } from "react";
import { useDiff, useHistory } from "#/api/hooks.ts";
import { ago, baseName, bytes } from "#/lib/format.ts";
import { readPrefs, savePrefs } from "#/lib/prefs.ts";
import { baseline, widest, type Since } from "./since.ts";

const labels: Record<Since, string> = { scan: "Last scan", day: "A day", week: "A week", month: "A month" };

/** What grew and what shrank over a span: growth is how a disk fills up unnoticed. */
export function Changes() {
  const history = useHistory();
  const snaps = history.data?.snapshots ?? [];
  const [chosen, setChosen] = useState<Since | undefined>(() => readPrefs().since);
  const since = chosen ?? widest(snaps);
  const pair = baseline(snaps, since);
  const diff = useDiff(pair?.from.id ?? "", pair?.to.id ?? "");
  const d = pair ? diff.data : undefined;
  const rows = (d?.changes ?? []).filter((c) => c.path !== d?.to.root && c.delta !== 0).slice(0, 6);
  const max = Math.max(1, ...rows.map((c) => Math.abs(c.delta)));
  return (
    <section className="card">
      <div className="card-head">
        <h2>What changed</h2>
        <div className="seg small" role="group" aria-label="Over">
          {(Object.keys(labels) as Since[]).map((s) => (
            <button key={s} aria-pressed={since === s} disabled={!baseline(snaps, s)} onClick={() => (setChosen(s), savePrefs({ since: s }))}>
              {labels[s]}
            </button>
          ))}
        </div>
      </div>
      {!pair ? (
        <p className="muted">{history.isLoading ? "Comparing…" : since === "scan" ? "After your next scan, this shows what grew and what shrank." : `History does not reach back ${labels[since].toLowerCase()} yet.`}</p>
      ) : !d?.from ? (
        <p className="muted">Comparing…</p>
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
