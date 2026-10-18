import { expect, test } from "bun:test";
import { isBehind, shortVersion } from "./version.ts";

test("an older release is behind the page", () => {
  expect(isBehind("0.1.1", "v0.1.2")).toBe(true);
  expect(isBehind("0.1.9", "v0.2.0")).toBe(true);
  expect(isBehind("0.9.9", "v1.0.0")).toBe(true);
});

test("the same or a newer release is not behind", () => {
  expect(isBehind("0.1.2", "v0.1.2")).toBe(false);
  expect(isBehind("0.1.3", "v0.1.2")).toBe(false);
  expect(isBehind("0.2.0", "v0.1.9")).toBe(false);
});

test("versions compare as numbers, not text", () => {
  expect(isBehind("0.1.9", "v0.1.10")).toBe(true);
  expect(isBehind("0.1.10", "v0.1.9")).toBe(false);
});

test("a development build counts as the release it was built after", () => {
  expect(isBehind("v0.1.1-6-g71342c4", "v0.1.1")).toBe(false);
  expect(isBehind("v0.1.1-6-g71342c4", "v0.1.2")).toBe(true);
});

test("an unknown version on either side never asks for an update", () => {
  expect(isBehind("dev", "v0.1.2")).toBe(false);
  expect(isBehind(undefined, "v0.1.2")).toBe(false);
  expect(isBehind("0.1.1", "")).toBe(false);
  expect(isBehind("71342c4", "v0.1.2")).toBe(false);
});

test("shortVersion drops the v and any build suffix", () => {
  expect(shortVersion("v0.1.2")).toBe("0.1.2");
  expect(shortVersion("0.1.2-6-g71342c4")).toBe("0.1.2");
  expect(shortVersion("dev")).toBe("");
});
