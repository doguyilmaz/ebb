import { create } from "zustand";
import type { PlanRequest } from "#/api/types.gen.ts";

/**
 * The one cleanup sheet of the app. It lives above every screen, so a finding
 * that disappears after it is cleaned does not take the result away with it.
 */
/** A cleanup to plan here, or a plan made elsewhere (an assistant, the CLI) to review and approve. */
export type SheetRequest = PlanRequest | { planId: string };

interface Sheet {
  request: SheetRequest | null;
  clearSelection: boolean;
  open: (request: SheetRequest, opts?: { clearSelection?: boolean }) => void;
  close: () => void;
}

export const useSheet = create<Sheet>((set) => ({
  request: null,
  clearSelection: false,
  open: (request, opts) => set({ request, clearSelection: !!opts?.clearSelection }),
  close: () => set({ request: null, clearSelection: false }),
}));
