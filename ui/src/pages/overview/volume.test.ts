import { describe, expect, test } from "bun:test";
import { segments } from "./volume.ts";
import type { Breakdown } from "#/api/types.gen.ts";

const v: Breakdown = {
  capacity: 500e9, used: 460e9, free: 40e9, dataUsed: 420e9, systemUsed: 12e9,
  others: [{ name: "VM", role: "VM", device: "d", used: 10e9 }, { name: "Preboot", role: "Preboot", device: "e", used: 9e9 }],
  purgeable: 1e9, snapshots: null, scanned: 400e9, notVisible: 20e9, fullDiskAccess: false,
};

describe("volume segments", () => {
  test("add up to the disk capacity", () => {
    const s = segments(v, 54e9, 105e9);
    expect(s.reduce((n, x) => n + x.bytes, 0)).toBeCloseTo(500e9, -3);
    const by = Object.fromEntries(s.map((x) => [x.key, x.bytes]));
    expect(by.system).toBe(12e9 + 10e9 + 9e9 + 9e9);
    expect(by.apps).toBe(54e9);
    expect(by.reclaimable).toBe(105e9);
    expect(by.rest).toBe(400e9 - 54e9 - 105e9);
    expect(by.free).toBe(40e9);
  });
  test("leave out empty parts", () => {
    const s = segments({ ...v, notVisible: 0, dataUsed: 400e9 }, 0, 0);
    expect(s.find((x) => x.key === "hidden")).toBeUndefined();
    expect(s.find((x) => x.key === "apps")).toBeUndefined();
  });
  test("never go negative when parts overlap", () => {
    const s = segments(v, 300e9, 300e9);
    expect(s.every((x) => x.bytes >= 0)).toBe(true);
  });
});
