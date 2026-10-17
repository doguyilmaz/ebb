import { create } from "zustand";

export interface Step {
  route: "/" | "/explore" | "/findings";
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
  { route: "/explore", target: "views", title: "Four ways to look", body: "Sunburst, treemap, icicle and list show the same folder. ebb remembers the one you like." },
  { route: "/explore", target: "metric", title: "Allocated or unique", body: "Unique leaves out space shared with clones and hardlinks, which deleting would not free." },
  { route: "/findings", target: "finding", title: "Every finding explains itself", body: "Why it is large, what removing it costs, and how safe it is. Tick it or press Clean." },
  { route: "/findings", target: "select", title: "Pick, then review", body: "Tick findings to collect them in the tray below. You always see the exact plan first; things go to the Trash, and Activity can put them back." },
  { route: "/findings", target: "help", title: "Come back any time", body: "Start this tour again from here. ⌘K jumps to any folder, page or cleanup." },
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
