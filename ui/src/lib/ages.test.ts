import { expect, test } from "bun:test";
import { oldShare } from "./ages.ts";

test("old share counts buckets beyond a year", () => {
  const b = [
    { label: "Past week", days: 7, bytes: 10 },
    { label: "Past year", days: 365, bytes: 20 },
    { label: "1–2 years", days: 730, bytes: 30 },
    { label: "Older", days: 0, bytes: 40 },
  ];
  expect(oldShare(b)).toEqual({ old: 70, total: 100 });
});
