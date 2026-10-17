import { create } from "zustand";
import type { PlanEvent, Progress } from "#/api/types.gen.ts";

/** Live state pushed by ebb's event stream. */
interface Live {
  scan: Progress | null;
  planEvents: PlanEvent[];
  setScan: (p: Progress | null) => void;
  addPlanEvent: (e: PlanEvent) => void;
  resetPlan: () => void;
}

export const useLive = create<Live>((set) => ({
  scan: null,
  planEvents: [],
  setScan: (scan) => set({ scan }),
  addPlanEvent: (e) => set((s) => ({ planEvents: [...s.planEvents, e] })),
  resetPlan: () => set({ planEvents: [] }),
}));
