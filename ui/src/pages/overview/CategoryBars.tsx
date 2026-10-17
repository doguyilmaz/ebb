import { Link } from "@tanstack/react-router";
import { useState } from "react";
import type { FindingView } from "#/api/types.gen.ts";
import { categoryTitle } from "#/lib/glossary.ts";
import { bytes, percent } from "#/lib/format.ts";
import { firstTime } from "#/lib/theme.ts";
import { Info } from "#/ui/Tip.tsx";

const MAX = 7;

/** Cleanable space by kind, one bar each; the bars share the "Can be cleaned" colour of the disk bar. */
export function CategoryBars({ findings, total }: { findings: FindingView[]; total: number }) {
  const [enter] = useState(() => firstTime("categories"));
  const sums = new Map<string, number>();
  for (const f of findings) if (f.actions?.length) sums.set(f.category, (sums.get(f.category) ?? 0) + f.bytes);
  const rows = [...sums].sort((a, b) => b[1] - a[1]);
  const shown = rows.slice(0, MAX);
  const rest = rows.slice(MAX).reduce((n, [, b]) => n + b, 0);
  const max = Math.max(1, ...shown.map(([, b]) => b), rest);
  return (
    <section className="card" data-tour="categories">
      <div className="card-head">
        <h2>What can be cleaned</h2>
        <Info term="reclaimable" />
      </div>
      <p className="card-sub">By kind. Click one to see its findings.</p>
      {shown.length === 0 ? (
        <p className="muted">Nothing to clean right now.</p>
      ) : (
        <ol className={`hbars${enter ? " enter" : ""}`}>
          {shown.map(([cat, b], i) => (
            <li key={cat}>
              <Link to="/findings" hash={`cat-${cat}`} className="hbar" data-term={`category.${cat}`} data-value={`${bytes(b)} · ${percent(b / Math.max(1, total))} of what can be cleaned`} style={{ ["--i" as string]: i }}>
                <span className="ellipsis">{categoryTitle(cat)}</span>
                <span className="hbar-track">
                  <i style={{ width: `${(b / max) * 100}%`, background: "var(--s3)" }} />
                </span>
                <span className="hbar-value num">{bytes(b)}</span>
              </Link>
            </li>
          ))}
          {rest > 0 && (
            <li>
              <Link to="/findings" className="hbar" style={{ ["--i" as string]: shown.length }} data-tip="Smaller kinds of findings, together.">
                <span className="faint">Everything else</span>
                <span className="hbar-track">
                  <i style={{ width: `${(rest / max) * 100}%`, background: "var(--other)" }} />
                </span>
                <span className="hbar-value num">{bytes(rest)}</span>
              </Link>
            </li>
          )}
        </ol>
      )}
    </section>
  );
}
