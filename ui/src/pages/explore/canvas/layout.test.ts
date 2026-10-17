import { describe, expect, test } from "bun:test";
import type { SliceNode } from "#/api/types.gen.ts";
import { hitArc, hitRect, icicle, sunburst, treemap } from "./layout.ts";

const leaf = (name: string, alloc: number, kind = 1): SliceNode => ({ id: 1, name, kind, alloc, unique: alloc, files: 1, children: null });
const root: SliceNode = {
  id: 0, name: "/r", kind: 0, alloc: 100, unique: 100, files: 4,
  children: [
    { id: 2, name: "a", kind: 0, alloc: 60, unique: 60, files: 2, children: [leaf("a1", 40), leaf("a2", 20)] },
    leaf("b", 30),
    { id: 9, name: "other", kind: 3, alloc: 10, unique: 10, files: 1, other: 3, children: null },
  ],
};

describe("layout", () => {
  test("sunburst angles fill the circle at depth 1 and nest", () => {
    const arcs = sunburst(root, 200, "alloc");
    const d1 = arcs.filter((a) => a.depth === 1);
    expect(d1.reduce((n, a) => n + (a.a1 - a.a0), 0)).toBeCloseTo(Math.PI * 2, 5);
    const a = arcs.find((x) => x.node.name === "a")!;
    const kids = arcs.filter((x) => x.depth === 2);
    expect(kids.every((k) => k.a0 >= a.a0 - 1e-9 && k.a1 <= a.a1 + 1e-9)).toBe(true);
    expect(a.path).toBe("/r/a");
    expect(kids.find((k) => k.node.name === "a1")!.path).toBe("/r/a/a1");
  });

  test("treemap areas are proportional to size", () => {
    const rects = treemap(root, 400, 300, "alloc").filter((r) => r.depth === 1);
    const area = (n: string) => {
      const r = rects.find((x) => x.node.name === n)!;
      return (r.x1 - r.x0) * (r.y1 - r.y0);
    };
    expect(area("a") / area("b")).toBeCloseTo(2, 1);
  });

  test("icicle columns stack by depth", () => {
    const rects = icicle(root, 400, 300, "alloc");
    const a = rects.find((r) => r.node.name === "a")!;
    const a1 = rects.find((r) => r.node.name === "a1")!;
    expect(a1.x0).toBeGreaterThanOrEqual(a.x1 - 1e-6);
    expect(a1.y0).toBeGreaterThanOrEqual(a.y0 - 1e-6);
  });

  test("hit testing", () => {
    const arcs = sunburst(root, 200, "alloc");
    expect(hitArc(arcs, 0, 0, 200)).toBe("center");
    const a = arcs.find((x) => x.node.name === "a")!;
    const mid = (a.a0 + a.a1) / 2;
    const r = (a.r0 + a.r1) / 2;
    expect(hitArc(arcs, Math.sin(mid) * r, -Math.cos(mid) * r, 200)).toBe(a);
    const rects = treemap(root, 400, 300, "alloc");
    const b = rects.find((x) => x.node.name === "b")!;
    expect(hitRect(rects, (b.x0 + b.x1) / 2, (b.y0 + b.y1) / 2)?.node.name).toBe("b");
  });
});
