import type { AgeBucket } from "#/api/types.gen.ts";

/** One hue, light to dark: older data is darker because it is the part worth a look. */
export const ageColor = (i: number, n: number) => `color-mix(in oklab, var(--accent) ${Math.round(26 + (74 * i) / Math.max(1, n - 1))}%, var(--surface))`;

/** Bytes untouched for over a year, and the total. */
export function oldShare(buckets: AgeBucket[]): { old: number; total: number } {
  let old = 0;
  let total = 0;
  for (const b of buckets) {
    total += b.bytes;
    if (b.days === 0 || b.days > 365) old += b.bytes;
  }
  return { old, total };
}
