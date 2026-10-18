const units = ["kB", "MB", "GB", "TB", "PB"];

/** Decimal units, as Finder shows them. */
export function bytes(n: number): string {
  if (n < 1000) return `${Math.round(n)} B`;
  let v = n / 1000;
  let i = 0;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v.toFixed(1).replace(/\.0$/, "")} ${units[i]}`;
}

export function percent(f: number): string {
  return `${(f * 100).toFixed(1).replace(/\.0$/, "")}%`;
}

export function ago(iso: string | number, now = Date.now()): string {
  const t = typeof iso === "number" ? iso * 1000 : new Date(iso).getTime();
  const s = (now - t) / 1000;
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 48 * 3600) return `${Math.floor(s / 3600)} h ago`;
  if (s < 60 * 86400) return `${Math.floor(s / 86400)} days ago`;
  return new Date(t).toLocaleDateString();
}

export function count(n: number): string {
  return n.toLocaleString("en-US");
}

/** The last path component, for labels. */
export const baseName = (p: string) => p.slice(p.lastIndexOf("/") + 1) || p;

/** The disk a scan covers, as people call it: an external disk's name, or this Mac. */
export const diskName = (root: string | undefined) => (root?.startsWith("/Volumes/") ? root.split("/")[2] : "This Mac");
