import type { Plan, PlanResult } from "#/api/types.gen.ts";

export interface SheetState {
  phase: "planning" | "review" | "running" | "done" | "approved";
  plan: Plan | null;
  confirmed: boolean;
  result: PlanResult | null;
  error: string | null;
}

export type SheetAction =
  | { type: "planned"; plan: Plan }
  | { type: "confirm"; value: boolean }
  | { type: "run" }
  | { type: "done"; result: PlanResult }
  | { type: "approved" }
  | { type: "error"; message: string };

export const initial: SheetState = { phase: "planning", plan: null, confirmed: false, result: null, error: null };

export function reduce(s: SheetState, a: SheetAction): SheetState {
  switch (a.type) {
    case "planned":
      return { ...initial, phase: "review", plan: a.plan };
    case "confirm":
      return { ...s, confirmed: a.value };
    case "run":
      return { ...s, phase: "running", error: null };
    case "done":
      return { ...s, phase: "done", result: a.result };
    case "approved":
      return { ...s, phase: "approved", error: null };
    case "error":
      return { ...s, phase: s.plan ? "review" : "planning", error: a.message };
  }
}

export function summary(p: Plan) {
  const items = p.items ?? [];
  const runnable = items.filter((i) => !i.blocked && i.action.kind !== "manual");
  return {
    runnable: runnable.length,
    blocked: items.filter((i) => i.blocked).length,
    manual: items.filter((i) => !i.blocked && i.action.kind === "manual").length,
    trash: runnable.filter((i) => i.action.kind === "trash").length,
    bytes: p.bytes,
  };
}

/** Risky plans need the person to tick that they checked the list. */
export function canRun(s: SheetState): boolean {
  if (s.phase !== "review" || !s.plan || summary(s.plan).runnable === 0) return false;
  return !s.plan.needsApproval || s.confirmed;
}
