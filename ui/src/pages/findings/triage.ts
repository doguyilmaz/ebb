import type { FindingView } from "#/api/types.gen.ts";

export type Filter = "all" | "regenerable" | "caution" | "user-data";

/** A finding ebb can clean itself; manual ones only explain what to do. */
export const pickable = (f: FindingView) => (f.actions?.length ?? 0) > 0 && f.actions![0].kind !== "manual";

export const matches = (f: FindingView, filter: Filter) => filter === "all" || f.safety === filter || (filter === "user-data" && f.safety === "system");

/** Whether none, some or all of a group's cleanable findings are picked, for its tri-state checkbox. */
export function groupState(list: FindingView[], picked: Set<string>): "none" | "some" | "all" {
  const ids = list.filter(pickable).map((f) => f.id);
  const n = ids.filter((id) => picked.has(id)).length;
  return n === 0 ? "none" : n === ids.length ? "all" : "some";
}

/** Findings grouped by category, biggest group first, biggest finding first inside. */
export function grouped(list: FindingView[]): [string, FindingView[]][] {
  const groups = new Map<string, FindingView[]>();
  for (const f of list) groups.set(f.category, [...(groups.get(f.category) ?? []), f]);
  const size = (l: FindingView[]) => l.reduce((n, f) => n + f.bytes, 0);
  return [...groups].map(([c, l]) => [c, [...l].sort((a, b) => b.bytes - a.bytes)] as [string, FindingView[]]).sort((a, b) => size(b[1]) - size(a[1]));
}
