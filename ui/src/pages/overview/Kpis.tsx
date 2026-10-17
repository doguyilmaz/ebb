import type { Breakdown, DiffResponse, Meta } from "#/api/types.gen.ts";
import { ago, bytes, count } from "#/lib/format.ts";
import { Info } from "#/ui/Tip.tsx";

const signed = (n: number, fmt: (v: number) => string) => (n === 0 ? "no change" : `${n > 0 ? "+" : "−"}${fmt(Math.abs(n))}`);

/** The few numbers people check every time, each with how it moved since the scan before. */
export function Kpis({ volume, reclaimable, last, diff }: { volume: Breakdown; reclaimable: number; last: Meta | null; diff?: DiffResponse }) {
  const used = volume.capacity - volume.free;
  const usedDelta = diff ? diff.to.used - diff.from.used : null;
  const filesDelta = diff ? diff.to.files - diff.from.files : null;
  return (
    <section className="kpis" aria-label="Key numbers">
      <div className="kpi">
        <span className="kpi-label">Used</span>
        <span className="kpi-value num">{bytes(used)}</span>
        {usedDelta !== null && <span className="kpi-delta num" data-dir={Math.sign(usedDelta)}>{signed(usedDelta, bytes)} since the scan before</span>}
      </div>
      <div className="kpi">
        <span className="kpi-label">
          Can be cleaned <Info term="reclaimable" />
        </span>
        <span className="kpi-value num">
          <i className="swatch" style={{ background: "var(--s3)" }} /> {bytes(reclaimable)}
        </span>
        <span className="kpi-delta">Rebuilds itself or you check first</span>
      </div>
      <div className="kpi">
        <span className="kpi-label">
          Not visible <Info term="segment.hidden" />
        </span>
        <span className="kpi-value num">{bytes(volume.notVisible)}</span>
        <span className="kpi-delta">{volume.fullDiskAccess ? "Full Disk Access is on" : "Full Disk Access shows more"}</span>
      </div>
      {last && (
        <div className="kpi">
          <span className="kpi-label">Files read</span>
          <span className="kpi-value num">{count(last.files)}</span>
          <span className="kpi-delta num" data-dir={filesDelta === null ? undefined : Math.sign(filesDelta)}>
            {filesDelta !== null ? `${signed(filesDelta, count)} · ` : ""}
            {last.incremental ? `updated in ${(last.elapsed / 1e9).toFixed(1)} s` : `${Math.round(last.elapsed / 1e9)} s`}, {ago(last.time)}
          </span>
        </div>
      )}
    </section>
  );
}
