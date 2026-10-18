import { create } from "zustand";

export interface Step {
  route: "/" | "/explore" | "/findings" | "/apps" | "/duplicates" | "/sources";
  /** data-tour value of the element the step points at. */
  target: string;
  title: string;
  body: string;
}

/** A first walk through ebb: what each screen answers, in the order people need it. */
export const steps: Step[] = [
  { route: "/", target: "volume", title: "Your whole disk", body: "Each colour is one part of the disk. Hover a part to see what it is; the (i) icons explain every term." },
  { route: "/", target: "categories", title: "What can be cleaned", body: "Everything ebb's findings can free, by kind. Click a bar to see the findings behind it." },
  { route: "/", target: "ages", title: "How old it is", body: "Data nothing touched for a year is often safe to archive. Explore can colour folders by age." },
  { route: "/", target: "presets", title: "Quick cleanups", body: "Ready-made groups, like every cache that rebuilds itself. A click shows the plan; nothing runs until you approve it." },
  { route: "/explore", target: "chart", title: "Explore any folder", body: "Click a slice to go in and the middle to come back. Hover for size, share and age." },
  { route: "/explore", target: "views", title: "Five ways to look", body: "Sunburst, treemap, icicle and list show the same folder; Files lists the biggest and oldest files below it. ebb remembers your choice." },
  { route: "/explore", target: "metric", title: "Allocated or unique", body: "Unique leaves out space shared with clones and hardlinks, which deleting would not free." },
  { route: "/findings", target: "finding", title: "Every finding explains itself", body: "Why it is large, what removing it costs and how safe it is. Tick it or press Clean." },
  { route: "/findings", target: "select", title: "Pick, then review", body: "Tick findings to collect them in the tray below. You always see the exact plan first; things go to the Trash and Activity can put them back." },
  { route: "/apps", target: "apps", title: "Apps and what they leave", body: "Each app with the data it keeps in your Library. Uninstall moves both to the Trash." },
  { route: "/duplicates", target: "dupes", title: "Copies of the same file", body: "ebb compares content, not names. You choose which copy stays; the others go to the Trash." },
  { route: "/sources", target: "flows", title: "Where space comes from", body: "Folders on the left, the tool or app that filled them in the middle, the kind of data on the right." },
  { route: "/sources", target: "help", title: "Come back any time", body: "Start this tour again from here. ⌘K jumps to any folder, page or cleanup." },
];

const KEY = "ebb.tour";

export function tourSeen(): boolean {
  try {
    return localStorage.getItem(KEY) === "seen";
  } catch {
    return false;
  }
}

function remember() {
  try {
    localStorage.setItem(KEY, "seen");
  } catch {}
}

interface Tour {
  step: number | null;
  start: () => void;
  next: () => void;
  back: () => void;
  /** Ends the tour; seen (the default) keeps it from starting on its own again. */
  stop: (seen?: boolean) => void;
}

export const useTour = create<Tour>((set, get) => ({
  step: null,
  start: () => set({ step: 0 }),
  next: () => {
    const s = get().step;
    if (s === null) return;
    if (s + 1 >= steps.length) return get().stop();
    set({ step: s + 1 });
  },
  back: () => {
    const s = get().step;
    if (s !== null && s > 0) set({ step: s - 1 });
  },
  stop: (seen = true) => {
    if (seen) remember();
    set({ step: null });
  },
}));

export interface Box {
  top: number;
  left: number;
  width: number;
  height: number;
}

const MARGIN = 12;
const GAP = 16;

/**
 * Where the step's card goes: below or above a short target, beside a tall
 * one, and only inside a target that leaves no room around it. The card never
 * covers what it is talking about when there is any space left.
 */
export function placeCard(b: Box, card: { w: number; h: number }, view: { w: number; h: number }): { top: number; left: number } {
  const fits = (top: number, left: number) => top >= MARGIN && left >= MARGIN && top + card.h <= view.h - MARGIN && left + card.w <= view.w - MARGIN;
  const clampX = (x: number) => Math.max(MARGIN, Math.min(view.w - card.w - MARGIN, x));
  const clampY = (y: number) => Math.max(MARGIN, Math.min(view.h - card.h - MARGIN, y));
  const below = { top: b.top + b.height + GAP, left: clampX(b.left) };
  const above = { top: b.top - GAP - card.h, left: clampX(b.left) };
  const right = { top: clampY(b.top + 24), left: b.left + b.width + GAP };
  const left = { top: clampY(b.top + 24), left: b.left - GAP - card.w };
  const tall = b.height > view.h * 0.45;
  for (const p of tall ? [right, left, below, above] : [below, above, right, left]) if (fits(p.top, p.left)) return p;
  return { top: clampY(b.top + b.height - card.h - GAP), left: clampX(b.left + b.width - card.w - GAP) };
}
