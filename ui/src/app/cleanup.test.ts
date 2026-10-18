import { describe, expect, test } from "bun:test";
import { canRun, initial, reduce, summary } from "./cleanup.ts";
import type { Plan } from "#/api/types.gen.ts";

const plan = (over: Partial<Plan> = {}): Plan => ({
  id: "p1", created: "", expires: "", source: "ui", bytes: 3e9, needsApproval: false, approved: false,
  items: [
    { path: "/a", bytes: 2e9, safety: "regenerable", action: { kind: "delete" } },
    { path: "/b", bytes: 1e9, safety: "caution", action: { kind: "trash" } },
    { path: "/c", bytes: 5e8, safety: "regenerable", action: { kind: "delete" }, blocked: "Quit Xcode first" },
    { path: "/d", bytes: 0, safety: "system", action: { kind: "manual", hint: "tmutil …" } },
  ],
  ...over,
});

describe("cleanup sheet state", () => {
  test("a plan with risky items needs the confirmation box", () => {
    let s = reduce(initial, { type: "planned", plan: plan({ needsApproval: true }) });
    expect(s.phase).toBe("review");
    expect(canRun(s)).toBe(false);
    s = reduce(s, { type: "confirm", value: true });
    expect(canRun(s)).toBe(true);
  });
  test("regenerable-only plans run without the box", () => {
    expect(canRun(reduce(initial, { type: "planned", plan: plan() }))).toBe(true);
  });
  test("a plan an assistant made is only approved here, after the box", () => {
    let s = reduce(initial, { type: "planned", plan: plan({ needsApproval: true, source: "mcp" }) });
    expect(canRun(s)).toBe(false);
    s = reduce(reduce(s, { type: "confirm", value: true }), { type: "run" });
    s = reduce(s, { type: "approved" });
    expect(s.phase).toBe("approved");
    expect(s.result).toBeNull();
  });
  test("summary counts what will happen", () => {
    expect(summary(plan())).toEqual({ runnable: 2, blocked: 1, manual: 1, trash: 1, bytes: 3e9 });
  });
  test("progress then result", () => {
    let s = reduce(initial, { type: "planned", plan: plan() });
    s = reduce(s, { type: "run" });
    expect(s.phase).toBe("running");
    s = reduce(s, { type: "done", result: { planId: "p1", freed: 2e9, done: 1, refused: 1, failed: 0, manual: 1, ops: null } });
    expect(s.phase).toBe("done");
    expect(s.result?.freed).toBe(2e9);
  });
  test("a failure keeps the plan visible with the error", () => {
    let s = reduce(initial, { type: "planned", plan: plan() });
    s = reduce(reduce(s, { type: "run" }), { type: "error", message: "plan expired" });
    expect(s.phase).toBe("review");
    expect(s.error).toBe("plan expired");
  });
});
