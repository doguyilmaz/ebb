import type { Breakdown } from "#/api/types.gen.ts";

export interface Segment {
  key: "system" | "apps" | "reclaimable" | "rest" | "hidden" | "free";
  label: string;
  bytes: number;
  color: string;
}

/**
 * Splits the whole disk into parts that add up to its capacity. APFS keeps
 * some space for its own use; that remainder is counted with macOS.
 */
export function segments(v: Breakdown, apps: number, reclaimable: number): Segment[] {
  const others = (v.others ?? []).reduce((n, o) => n + o.used, 0);
  const scanned = v.scanned;
  const appsPart = Math.min(apps, scanned);
  const reclaimPart = Math.min(reclaimable, Math.max(0, scanned - appsPart));
  const rest = Math.max(0, scanned - appsPart - reclaimPart);
  const known = v.systemUsed + others + scanned + v.notVisible + v.free;
  const system = v.systemUsed + others + Math.max(0, v.capacity - known);
  const all: Segment[] = [
    { key: "system", label: "macOS", bytes: system, color: "var(--s7)" },
    { key: "apps", label: "Apps", bytes: appsPart, color: "var(--s1)" },
    { key: "reclaimable", label: "Can be cleaned", bytes: reclaimPart, color: "var(--s3)" },
    { key: "rest", label: "Everything else", bytes: rest, color: "var(--s4)" },
    { key: "hidden", label: "Not visible", bytes: v.notVisible, color: "var(--other)" },
    { key: "free", label: "Free", bytes: v.free, color: "transparent" },
  ];
  return all.filter((s) => s.bytes > 0);
}
