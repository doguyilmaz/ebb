import type { Plan } from "#/api/types.gen.ts";

export function CleanupSheet(_: { planId: string; plan?: Plan; onClose: () => void }) {
  return null;
}
