import type { Meta } from "#/api/types.gen.ts";

export type Since = "scan" | "day" | "week" | "month";
export const spans: Record<Exclude<Since, "scan">, number> = { day: 864e5, week: 7 * 864e5, month: 30 * 864e5 };

/**
 * The snapshot to compare the newest one with: the scan before it, or the
 * newest scan of the same folder at least the span older. Null when history
 * does not reach back that far.
 */
export function baseline(snaps: Meta[], since: Since): { from: Meta; to: Meta } | null {
  const sorted = [...snaps].sort((a, b) => Date.parse(b.time) - Date.parse(a.time));
  const to = sorted[0];
  if (!to) return null;
  const same = sorted.slice(1).filter((m) => m.root === to.root);
  const from = since === "scan" ? same[0] : same.find((m) => Date.parse(to.time) - Date.parse(m.time) >= spans[since]);
  return from ? { from, to } : null;
}

/** The widest span history covers, so the card opens on the most telling comparison. */
export function widest(snaps: Meta[]): Since {
  for (const s of ["week", "day"] as const) if (baseline(snaps, s)) return s;
  return "scan";
}
