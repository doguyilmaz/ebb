import { expect, test } from "bun:test";
import { niceTicks, scale } from "./timeline.ts";

test("ticks are round byte steps covering the data", () => {
  const t = niceTicks(463e9, 478e9, 4);
  expect(t[0]).toBeLessThanOrEqual(463e9);
  expect(t.at(-1)!).toBeGreaterThanOrEqual(478e9);
  const step = t[1] - t[0];
  expect([1e9, 2e9, 2.5e9, 5e9, 10e9]).toContain(step);
});

test("a flat series still gets a range", () => {
  const t = niceTicks(470e9, 470e9, 4);
  expect(t.length).toBeGreaterThanOrEqual(2);
  expect(t[0]).toBeLessThan(470e9);
});

test("scale maps the domain onto the range, inverted for y", () => {
  const y = scale([0, 100], [200, 0]);
  expect(y(0)).toBe(200);
  expect(y(100)).toBe(0);
  expect(y(50)).toBe(100);
});
