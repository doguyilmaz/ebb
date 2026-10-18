import { useLayoutEffect, useRef, useState } from "react";
import type { Breakdown } from "#/api/types.gen.ts";
import { bytes, percent } from "#/lib/format.ts";
import { firstTime, useSchemeVersion } from "#/lib/theme.ts";
import { inkOn } from "#/pages/explore/canvas/colors.ts";
import { Info } from "#/ui/Tip.tsx";
import { segments } from "./volume.ts";

const parseRGB = (c: string): [number, number, number] | null => {
  const m = c.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?/);
  if (!m || (m[4] !== undefined && Number(m[4]) === 0)) return null;
  return [Number(m[1]), Number(m[2]), Number(m[3])];
};

/** The whole disk as one bar: used parts from the left, free space is the water at the end. */
export function VolumeBar({ volume, apps, reclaimable }: { volume: Breakdown; apps: number; reclaimable: number }) {
  const segs = segments(volume, apps, reclaimable);
  const total = segs.reduce((n, s) => n + s.bytes, 0) || 1;
  const bar = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  const [enter] = useState(() => firstTime("volume"));
  const scheme = useSchemeVersion();

  useLayoutEffect(() => {
    const ro = new ResizeObserver(([e]) => setWidth(e.contentRect.width));
    ro.observe(bar.current!);
    return () => ro.disconnect();
  }, []);

  // Labels sit on the segments' own colours, so their ink follows each fill.
  useLayoutEffect(() => {
    for (const el of bar.current!.children as HTMLCollectionOf<HTMLElement>) {
      const c = parseRGB(getComputedStyle(el).backgroundColor);
      el.style.color = c ? inkOn(c) : "";
    }
  }, [scheme, width, volume]);

  return (
    <section className="card" aria-label="Disk use" data-tour="volume">
      <div className="card-head">
        <h2>Where the space is</h2>
        <span className="grow" />
        <span className="faint small-text">Hover a part for what it means</span>
      </div>
      <div className={`vbar${enter ? " enter" : ""}`} ref={bar} role="img" aria-label={segs.map((s) => `${s.label} ${bytes(s.bytes)}`).join(", ")}>
        {segs.map((s) => {
          const px = (s.bytes / total) * width;
          return (
            <div
              key={s.key}
              className={`vseg ${s.key}`}
              style={{ flex: `${s.bytes / total} 1 0`, background: s.color }}
              data-term={`segment.${s.key}`}
              data-value={`${bytes(s.bytes)} · ${percent(s.bytes / total)}`}
            >
              {px >= 96 && (
                <span className="vseg-label">
                  <b>{s.label}</b>
                  <span className="num">{bytes(s.bytes)}</span>
                </span>
              )}
            </div>
          );
        })}
      </div>
      <ul className="legend">
        {segs.map((s) => (
          <li key={s.key}>
            <i className={`swatch ${s.key}`} style={{ background: s.color }} />
            <span>{s.label}</span>
            <span className="num faint">{bytes(s.bytes)}</span>
            <Info term={`segment.${s.key}`} />
          </li>
        ))}
      </ul>
    </section>
  );
}
