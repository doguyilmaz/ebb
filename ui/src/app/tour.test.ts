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

const { placeCard } = await import("./tour.ts");
const view = { w: 1440, h: 813 };
const card = { w: 320, h: 200 };
const inside = (p: { top: number; left: number }) => p.left >= 12 && p.top >= 12 && p.left + card.w <= view.w - 12 && p.top + card.h <= view.h - 12;
const overlaps = (p: { top: number; left: number }, b: { top: number; left: number; width: number; height: number }) =>
  p.left < b.left + b.width && p.left + card.w > b.left && p.top < b.top + b.height && p.top + card.h > b.top;

test("a short target gets the card below it", () => {
  const b = { top: 220, left: 262, width: 1144, height: 190 };
  const p = placeCard(b, card, view);
  expect(p.top).toBeGreaterThan(b.top + b.height);
  expect(inside(p)).toBe(true);
});

test("a tall target gets the card beside it, never on top of it", () => {
  const b = { top: 172, left: 256, width: 812, height: 580 };
  const p = placeCard(b, card, view);
  expect(inside(p)).toBe(true);
  expect(overlaps(p, b)).toBe(false);
});

test("near the bottom the card goes above", () => {
  const b = { top: 640, left: 300, width: 300, height: 60 };
  const p = placeCard(b, card, view);
  expect(p.top + card.h).toBeLessThan(b.top);
});

test("with no room around a huge target the card sits inside its corner", () => {
  const b = { top: 20, left: 20, width: 1400, height: 780 };
  const p = placeCard(b, card, view);
  expect(inside(p)).toBe(true);
});
