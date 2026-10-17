import { create } from "zustand";

/** What the person picked for cleanup, across Explore and Findings. */
interface Selection {
  findings: Set<string>;
  paths: Set<string>;
  toggleFinding: (id: string) => void;
  togglePath: (p: string) => void;
  /** Adds or removes several findings at once (a group, or everything that rebuilds itself). */
  setFindings: (ids: string[], on: boolean) => void;
  clear: () => void;
}

const toggled = (s: Set<string>, v: string) => {
  const next = new Set(s);
  if (!next.delete(v)) next.add(v);
  return next;
};

export const useSelection = create<Selection>((set) => ({
  findings: new Set(),
  paths: new Set(),
  toggleFinding: (id) => set((s) => ({ findings: toggled(s.findings, id) })),
  togglePath: (p) => set((s) => ({ paths: toggled(s.paths, p) })),
  setFindings: (ids, on) =>
    set((s) => {
      const next = new Set(s.findings);
      for (const id of ids) on ? next.add(id) : next.delete(id);
      return { findings: next };
    }),
  clear: () => set({ findings: new Set(), paths: new Set() }),
}));
