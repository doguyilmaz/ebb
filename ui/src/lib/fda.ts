import { create } from "zustand";
import { useStartScan, useStatus } from "#/api/hooks.ts";
import type { ScanRequest } from "#/api/types.gen.ts";

const DECLINED = "ebb.fdaDeclined";

/** Someone chose to scan without Full Disk Access; macOS then asks per folder, and ebb stops asking first. */
export function declined(): boolean {
  try {
    return localStorage.getItem(DECLINED) === "1";
  } catch {
    return false;
  }
}

export function decline() {
  try {
    localStorage.setItem(DECLINED, "1");
  } catch {
    // Without storage ebb asks again next time, which is all that is lost.
  }
}

/** The scan waiting for an answer about Full Disk Access. */
export const useFdaAsk = create<{ pending: ScanRequest | null; ask: (r: ScanRequest | null) => void }>((set) => ({
  pending: null,
  ask: (pending) => set({ pending }),
}));

/** Starts a scan, asking for Full Disk Access first unless ebb has it or the person chose to scan without it. */
export function useScan() {
  const start = useStartScan();
  const status = useStatus();
  const ask = useFdaAsk((s) => s.ask);
  const run = (req: ScanRequest) => (status.data?.fullDiskAccess === false && !declined() ? ask(req) : start.mutate(req));
  return { run, start };
}
