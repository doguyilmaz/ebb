import type { App } from "#/api/types.gen.ts";

export type AppSort = "total" | "library" | "name";

/** Filters by name or bundle ID and orders a copy; ties fall back to the name. */
export function sortApps(apps: App[], sort: AppSort, query: string): App[] {
  const q = query.trim().toLowerCase();
  const list = q ? apps.filter((a) => a.name.toLowerCase().includes(q) || a.bundleId.toLowerCase().includes(q)) : [...apps];
  const byName = (a: App, b: App) => a.name.localeCompare(b.name);
  const key = (a: App) => (sort === "library" ? a.leftoverBytes : a.bytes + a.leftoverBytes);
  return list.sort((a, b) => (sort === "name" ? byName(a, b) : key(b) - key(a) || byName(a, b)));
}
