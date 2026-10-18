/** The release this page was built from; empty when git had no tags to tell. */
export const pageVersion = (import.meta.env?.VITE_EBB_VERSION as string | undefined) ?? "";

/** "0.1.2" from "v0.1.2", "0.1.2" or a development build like "v0.1.2-6-g71342c4". */
export const shortVersion = (v: string) => /^v?(\d+\.\d+\.\d+)/.exec(v)?.[1] ?? "";

/** True when ebb on this Mac is an older release than the page, so parts of the page may not work with it. */
export function isBehind(local: string | undefined, page = pageVersion): boolean {
  const a = shortVersion(local ?? "").split(".").map(Number);
  const b = shortVersion(page).split(".").map(Number);
  if (a.length !== 3 || b.length !== 3) return false;
  const i = a.findIndex((n, k) => n !== b[k]);
  return i >= 0 && a[i] < b[i];
}
