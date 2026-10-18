import { expect, test } from "bun:test";
import type { Group } from "#/api/types.gen.ts";
import { extras, groupKey } from "./extras.ts";

const group = (...paths: string[]): Group => ({ size: 10, wasted: 10 * (paths.length - 1), copies: paths.map((path, i) => ({ path, mtime: i, bytes: 10 })) });
const a = group("/h/a/x.mov", "/h/b/x.mov", "/h/c/x.mov");
const b = group("/h/d/y.zip", "/h/e/y.zip");

test("picked groups give every copy but the kept one", () => {
  expect(extras([a, b], new Set([groupKey(a)]), new Map())).toEqual({ paths: ["/h/b/x.mov", "/h/c/x.mov"], bytes: 20 });
});

test("the person can keep another copy", () => {
  const keep = new Map([[groupKey(a), "/h/c/x.mov"]]);
  expect(extras([a, b], new Set([groupKey(a), groupKey(b)]), keep).paths).toEqual(["/h/a/x.mov", "/h/b/x.mov", "/h/e/y.zip"]);
});

test("a stale choice falls back to the first copy, never removing all", () => {
  const keep = new Map([[groupKey(b), "/gone"]]);
  expect(extras([b], new Set([groupKey(b)]), keep).paths).toEqual(["/h/e/y.zip"]);
});
