import { create } from "zustand";

/** Whether the command palette is open; ⌘K and the top bar's search both open it. */
export const usePalette = create<{ open: boolean; setOpen: (open: boolean) => void }>((set) => ({
  open: false,
  setOpen: (open) => set({ open }),
}));
