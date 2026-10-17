import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { useAges } from "#/api/hooks.ts";
import { ageColor, oldShare } from "#/lib/ages.ts";
import { bytes, percent } from "#/lib/format.ts";
import { firstTime } from "#/lib/theme.ts";
import { Info } from "#/ui/Tip.tsx";

/** How long ago the data last changed, in ordered buckets: one hue, darker for older. */
export function AgeChart() {
  const ages = useAges();
  const [enter] = useState(() => firstTime("ages"));
  const b = ages.data?.buckets ?? [];
  const { old, total } = oldShare(b);
  if (!total) return null;
  const max = Math.max(1, ...b.map((x) => x.bytes));
  return (
    <section className="card" data-tour="ages">
      <div className="card-head">
        <h2>How old the data is</h2>
        <Info term="ages" />
      </div>
      <p className="insight">
        <strong>{percent(old / total)}</strong> has not changed in over a year
      </p>
      <div>
        <div className={`cols${enter ? " enter" : ""}`} style={{ ["--n" as string]: b.length }} role="img" aria-label={b.map((x) => `${x.label}: ${bytes(x.bytes)}`).join(", ")}>
          {b.map((x, i) => (
            <div key={x.label} className="col" data-tip={`Last changed: ${x.label.toLowerCase()}`} data-value={`${bytes(x.bytes)} · ${percent(x.bytes / total)}`} style={{ ["--i" as string]: i }}>
              <span className="col-value num">{bytes(x.bytes)}</span>
              <span className="col-bar" style={{ height: `${Math.max(1, (x.bytes / max) * 78)}%`, background: ageColor(i, b.length) }} />
            </div>
          ))}
        </div>
        <div className="col-labels" style={{ ["--n" as string]: b.length }} aria-hidden>
          {b.map((x) => (
            <span key={x.label}>{x.label}</span>
          ))}
        </div>
      </div>
      <div className="card-foot">
        Old, large folders are often safe to archive.
        <Link to="/explore" search={{ color: "age" }} className="btn small">
          See where they are
        </Link>
      </div>
    </section>
  );
}
