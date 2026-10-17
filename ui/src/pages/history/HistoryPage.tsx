import { Link } from "@tanstack/react-router";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useEffect, useState } from "react";
import { useDiff, useHistory } from "#/api/hooks.ts";
import { baseName, bytes, count } from "#/lib/format.ts";
import { PageHead } from "#/ui/PageHead.tsx";
import { Loading, Problem } from "#/ui/states.tsx";
import { Timeline } from "./Timeline.tsx";

const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });

export default function HistoryPage() {
  const history = useHistory();
  const snaps = history.data?.snapshots ?? [];
  const [from, setFrom] = useState<string>();
  const [to, setTo] = useState<string>();
  useEffect(() => {
    if (snaps.length >= 2 && !from && !to) {
      setTo(snaps[0].id);
      setFrom(snaps[1].id);
    }
  }, [snaps, from, to]);
  const diff = useDiff(from ?? "", to ?? "");
  if (history.error) return <Problem error={history.error} />;
  if (!history.data) return <Loading label="Reading the history…" />;
  const pick = (id: string) => {
    if (id === to || id === from) return;
    const t = (s: string | undefined) => snaps.find((x) => x.id === s)?.time ?? "";
    // The newer of the two stays "to"; the click replaces the end it is closer to in time.
    const clicked = t(id);
    if (clicked > t(to)) setTo(id);
    else if (clicked < t(from)) setFrom(id);
    else setFrom(id);
  };
  const rows = (diff.data?.changes ?? []).filter((c) => c.path !== diff.data?.to.root && c.delta !== 0).slice(0, 40);
  const max = Math.max(1, ...rows.map((c) => Math.abs(c.delta)));
  return (
    <>
      <PageHead title="History" sub="How the disk filled up over time. Every scan is kept; pick two to see what grew and what shrank between them." />
      {snaps.length < 2 ? (
        <p className="empty card">After a second scan, this shows how space changed. Scan again any time; it takes seconds.</p>
      ) : (
        <>
          <section className="card">
            <div className="card-head">
              <h2>Used space</h2>
              <span className="grow" />
              <span className="faint small-text">{count(snaps.length)} scans</span>
            </div>
            <Timeline snaps={snaps} from={from} to={to} onPick={pick} />
          </section>
          <section className="card">
            <div className="card-head" style={{ flexWrap: "wrap", gap: 10 }}>
              <h2>What changed</h2>
              <span className="grow" />
              <label className="row small-text muted">
                From
                <select className="input" value={from} onChange={(e) => setFrom(e.target.value)}>
                  {snaps.map((s) => (
                    <option key={s.id} value={s.id} disabled={s.id === to}>{when(s.time)}</option>
                  ))}
                </select>
              </label>
              <label className="row small-text muted">
                To
                <select className="input" value={to} onChange={(e) => setTo(e.target.value)}>
                  {snaps.map((s) => (
                    <option key={s.id} value={s.id} disabled={s.id === from}>{when(s.time)}</option>
                  ))}
                </select>
              </label>
            </div>
            {diff.isLoading ? (
              <Loading label="Comparing…" />
            ) : rows.length === 0 ? (
              <p className="muted">Nothing changed by more than 10 MB between these scans.</p>
            ) : (
              <ol className="changes">
                {rows.map((c) => {
                  const up = c.delta > 0;
                  return (
                    <li key={c.path}>
                      <Link to="/explore" search={{ path: c.path }} className="change history-change" data-tip={c.path} data-value={`${bytes(c.before)} → ${bytes(c.after)}`}>
                        <span className="ellipsis">
                          {baseName(c.path)} <span className="faint small-text">{c.path.slice(0, c.path.lastIndexOf("/"))}</span>
                        </span>
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
            )}
          </section>
        </>
      )}
    </>
  );
}
