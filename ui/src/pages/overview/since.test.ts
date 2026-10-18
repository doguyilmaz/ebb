import { expect, test } from "bun:test";
import type { Meta } from "#/api/types.gen.ts";
import { baseline, widest } from "./since.ts";

const at = (id: string, daysAgo: number, root = "/data"): Meta =>
  ({ id, root, time: new Date(Date.parse("2026-10-18T12:00:00Z") - daysAgo * 864e5).toISOString() }) as Meta;

const snaps = [at("now", 0), at("1h", 0.04), at("2d", 2), at("9d", 9), at("other", 20, "/Volumes/Backup")];

test("compares with the scan before, or the newest scan at least the span older", () => {
  expect(baseline(snaps, "scan")?.from.id).toBe("1h");
  expect(baseline(snaps, "day")?.from.id).toBe("2d");
  expect(baseline(snaps, "week")?.from.id).toBe("9d");
  expect(baseline(snaps, "month")).toBeNull();
});

test("only scans of the same folder count, and the widest covered span is the default", () => {
  expect(widest(snaps)).toBe("week");
  expect(widest([at("now", 0), at("other", 20, "/Volumes/Backup")])).toBe("scan");
  expect(baseline([], "scan")).toBeNull();
});
