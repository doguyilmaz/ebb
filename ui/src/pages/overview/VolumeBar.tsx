import type { Breakdown } from "#/api/types.gen.ts";
import { bytes, percent } from "#/lib/format.ts";
import { segments } from "./volume.ts";

/** The whole disk as one waterline: used parts fill from the left, free space is the water. */
export function VolumeBar({ volume, apps, reclaimable }: { volume: Breakdown; apps: number; reclaimable: number }) {
  const segs = segments(volume, apps, reclaimable);
  const total = segs.reduce((n, s) => n + s.bytes, 0) || 1;
  return (
    <section className="volume" aria-label="Disk use">
      <div className="volume-head">
        <h2>{bytes(volume.free)} free of {bytes(volume.capacity)}</h2>
        {volume.purgeable > 0 && <span className="muted">plus {bytes(volume.purgeable)} macOS frees on its own when needed</span>}
      </div>
      <div className="volume-bar" role="img" aria-label={segs.map((s) => `${s.label} ${bytes(s.bytes)}`).join(", ")}>
        {segs.map((s) => (
          <div
            key={s.key}
            className={`vseg ${s.key}`}
            style={{ flexGrow: s.bytes / total, background: s.color }}
            title={`${s.label}: ${bytes(s.bytes)} (${percent(s.bytes / total)}) — ${s.hint}`}
          />
        ))}
      </div>
      <ul className="legend">
        {segs.map((s) => (
          <li key={s.key}>
            <i className={`swatch ${s.key}`} style={{ background: s.color }} />
            <span>{s.label}</span>
            <span className="num muted">{bytes(s.bytes)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
