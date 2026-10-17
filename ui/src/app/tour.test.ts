import { beforeEach, expect, test } from "bun:test";

const mem = new Map<string, string>();
globalThis.localStorage = {
  getItem: (k: string) => mem.get(k) ?? null,
  setItem: (k: string, v: string) => void mem.set(k, v),
  removeItem: (k: string) => void mem.delete(k),
  clear: () => mem.clear(),
  key: () => null,
  length: 0,
} as Storage;

const { steps, tourSeen, useTour } = await import("./tour.ts");

beforeEach(() => {
  mem.clear();
  useTour.getState().stop(false);
});

test("steps walk forward and back within bounds", () => {
  const t = useTour.getState();
  t.start();
  expect(useTour.getState().step).toBe(0);
  t.back();
  expect(useTour.getState().step).toBe(0);
  t.next();
  expect(useTour.getState().step).toBe(1);
  t.back();
  expect(useTour.getState().step).toBe(0);
});

test("finishing or skipping remembers the tour was seen", () => {
  expect(tourSeen()).toBe(false);
  const t = useTour.getState();
  t.start();
  for (let i = 0; i < steps.length; i++) t.next();
  expect(useTour.getState().step).toBeNull();
  expect(tourSeen()).toBe(true);
  mem.clear();
  t.start();
  t.stop();
  expect(tourSeen()).toBe(true);
});
