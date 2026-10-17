import { create } from "zustand";
import type { PlanRequest } from "#/api/types.gen.ts";

/**
 * The one cleanup sheet of the app. It lives above every screen, so a finding
 * that disappears after it is cleaned does not take the result away with it.
 */
interface Sheet {
  request: PlanRequest | null;
  clearSelection: boolean;
  open: (request: PlanRequest, opts?: { clearSelection?: boolean }) => void;
  close: () => void;
}

export const useSheet = create<Sheet>((set) => ({
  request: null,
  clearSelection: false,
  open: (request, opts) => set({ request, clearSelection: !!opts?.clearSelection }),
  close: () => set({ request: null, clearSelection: false }),
}));
