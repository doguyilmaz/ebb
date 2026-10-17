import { expect, test } from "bun:test";
import { initial, shorten } from "./Breadcrumb.tsx";

test("initials keep the dot of hidden folders", () => {
  expect(initial("Library")).toBe("L");
  expect(initial(".bun")).toBe(".b");
});

test("leading folders shorten first and the last two stay whole", () => {
  const w = [{ full: 100, short: 20 }, { full: 100, short: 20 }, { full: 100, short: 20 }, { full: 100, short: 20 }];
  expect(shorten(w, 400)).toBe(0);
  expect(shorten(w, 330)).toBe(1);
  expect(shorten(w, 250)).toBe(2);
  expect(shorten(w, 10)).toBe(2);
});
