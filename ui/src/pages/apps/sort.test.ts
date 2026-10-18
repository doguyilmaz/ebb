import { expect, test } from "bun:test";
import type { App } from "#/api/types.gen.ts";
import { sortApps } from "./sort.ts";

const app = (name: string, bundleId: string, appBytes: number, lib: number) =>
  ({ name, bundleId, bytes: appBytes, leftoverBytes: lib, leftovers: [] }) as unknown as App;

const apps = [app("Zed", "dev.zed.Zed", 100, 5), app("Arc", "company.thebrowser.Browser", 50, 500), app("Notes+", "com.x.notes", 100, 5)];

test("sorts by size, library data or name without touching the input", () => {
  expect(sortApps(apps, "total", "").map((a) => a.name)).toEqual(["Arc", "Notes+", "Zed"]);
  expect(sortApps(apps, "library", "").map((a) => a.name)).toEqual(["Arc", "Notes+", "Zed"]);
  expect(sortApps(apps, "name", "").map((a) => a.name)).toEqual(["Arc", "Notes+", "Zed"]);
  expect(apps[0].name).toBe("Zed");
});

test("filters by name or bundle id", () => {
  expect(sortApps(apps, "total", "ZED").map((a) => a.name)).toEqual(["Zed"]);
  expect(sortApps(apps, "total", "thebrowser").map((a) => a.name)).toEqual(["Arc"]);
});
