import type { View } from "#/router.tsx";

/** Choices that should survive a reload: kept in this browser only, never required to work. */
export interface Prefs {
  view?: View;
  unique?: boolean;
  color?: "folder" | "age";
  appSort?: "total" | "library" | "name";
  filesMin?: number;
  filesAge?: number;
  since?: "scan" | "day" | "week" | "month";
  sourcesTab?: "flows" | "packages";
}

const KEY = "ebb.prefs";

export function readPrefs(): Prefs {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? "{}");
    return v && typeof v === "object" ? (v as Prefs) : {};
  } catch {
    return {};
  }
}

export function savePrefs(p: Prefs) {
  try {
    localStorage.setItem(KEY, JSON.stringify({ ...readPrefs(), ...p }));
  } catch {}
}
