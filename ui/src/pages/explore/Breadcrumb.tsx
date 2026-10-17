import { HardDrive } from "lucide-react";
import { useLayoutEffect, useRef, useState } from "react";

interface Crumb {
  label: string;
  target: string | undefined;
}

const SEP = 14;
const PAD = 14;
const ICON = 20;
let ctx: CanvasRenderingContext2D | null = null;
const textWidth = (s: string, bold: boolean, mono: boolean) => {
  ctx ??= document.createElement("canvas").getContext("2d");
  if (!ctx) return s.length * 8;
  ctx.font = mono ? "12.5px 'Plex Mono', monospace" : `${bold ? 600 : 400} 13.5px Plex, system-ui, sans-serif`;
  return ctx.measureText(s).width;
};

/** ".bun" → ".b", "Library" → "L": the way fish shows a long working directory. */
export const initial = (name: string) => (name.startsWith(".") ? name.slice(0, 2) : [...name][0] ?? name);

/**
 * How many leading folders to shorten so the trail fits in width: the last
 * two stay whole, the ones before them shrink to initials from the left.
 */
export function shorten(widths: { full: number; short: number }[], width: number): number {
  let total = widths.reduce((n, w) => n + w.full, 0);
  let k = 0;
  while (total > width && k < widths.length - 2) {
    total -= widths[k].full - widths[k].short;
    k++;
  }
  return k;
}

/** The scanned disk first, then each folder down to the current one, always on one line. */
export function Breadcrumb({ path, root, onOpen }: { path: string; root?: string; onOpen: (p: string | undefined) => void }) {
  const nav = useRef<HTMLElement>(null);
  const [width, setWidth] = useState(0);
  useLayoutEffect(() => {
    const ro = new ResizeObserver(([e]) => setWidth(e.contentRect.width));
    ro.observe(nav.current!);
    return () => ro.disconnect();
  }, []);

  const inRoot = root && (path === root || path.startsWith(root + "/"));
  const rest = inRoot ? path.slice(root!.length) : path;
  const parts = rest.split("/").filter(Boolean);
  const prefix = inRoot ? root! : "";
  const crumbs: Crumb[] = parts.map((p, i) => ({ label: p, target: `${prefix}/${parts.slice(0, i + 1).join("/")}` }));
  const disk = ICON + textWidth("Disk", false, false) + PAD;
  const widths = crumbs.map((c, i) => {
    const last = i === crumbs.length - 1;
    return { full: textWidth(c.label, last, false) + PAD + SEP, short: textWidth(initial(c.label), false, true) + 8 + SEP };
  });
  const cut = width ? shorten(widths, width - disk) : 0;

  return (
    <nav className="crumbs" aria-label="Folder" ref={nav}>
      <button className="crumb" onClick={() => onOpen(undefined)}>
        <HardDrive size={14} aria-hidden /> Disk
      </button>
      {crumbs.map((c, i) => {
        const last = i === crumbs.length - 1;
        const short = i < cut;
        return (
          <span key={c.target} className="row" style={{ gap: 1, minWidth: last ? 0 : undefined }}>
            <span className="crumb-sep" aria-hidden>/</span>
            <button
              className={short ? "crumb short" : last ? "crumb ellipsis" : "crumb"}
              aria-current={last ? "page" : undefined}
              aria-label={short ? c.label : undefined}
              data-tip={short ? c.label : undefined}
              onClick={() => onOpen(c.target)}
            >
              {short ? initial(c.label) : c.label}
            </button>
          </span>
        );
      })}
    </nav>
  );
}
