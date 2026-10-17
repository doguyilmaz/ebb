import { expect, test } from "bun:test";
import type { FindingView } from "#/api/types.gen.ts";
import { groupState, grouped, matches, pickable } from "./triage.ts";

const f = (id: string, category: string, bytes: number, safety = "regenerable", kind = "trash"): FindingView =>
  ({ id, ruleId: id, title: id, category, safety, bytes, alloc: bytes, targets: [], why: "", impact: "", actions: [{ kind }] }) as FindingView;

test("manual findings are explained but never picked", () => {
  expect(pickable(f("a", "c", 1))).toBe(true);
  expect(pickable(f("b", "c", 1, "caution", "manual"))).toBe(false);
});

test("group checkbox reflects only what can be picked", () => {
  const l = [f("a", "c", 1), f("b", "c", 1), f("m", "c", 1, "caution", "manual")];
  expect(groupState(l, new Set())).toBe("none");
  expect(groupState(l, new Set(["a"]))).toBe("some");
  expect(groupState(l, new Set(["a", "b"]))).toBe("all");
});

test("groups and findings come biggest first", () => {
  const g = grouped([f("s", "small", 1), f("b1", "big", 5), f("b2", "big", 9)]);
  expect(g.map(([c]) => c)).toEqual(["big", "small"]);
  expect(g[0][1].map((x) => x.id)).toEqual(["b2", "b1"]);
});

test("the your-data filter also shows system items", () => {
  expect(matches(f("x", "c", 1, "system"), "user-data")).toBe(true);
  expect(matches(f("x", "c", 1, "caution"), "regenerable")).toBe(false);
});
