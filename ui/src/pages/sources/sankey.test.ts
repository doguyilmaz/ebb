import { expect, test } from "bun:test";
import type { Flows } from "#/api/types.gen.ts";
import { LABEL_GAP, layout } from "./sankey.ts";

const flows: Flows = {
  total: 600,
  nodes: [
    { id: "p:a", label: "~/Library", column: 0, bytes: 400 },
    { id: "p:b", label: "~/Documents", column: 0, bytes: 200 },
    { id: "o:go", label: "Go", column: 1, bytes: 300 },
    { id: "o:you", label: "You", column: 1, bytes: 300 },
    { id: "k:build", label: "build-artifact", column: 2, bytes: 300 },
    { id: "k:yours", label: "yours", column: 2, bytes: 300 },
  ],
  links: [
    { from: "p:a", to: "o:go", bytes: 300 },
    { from: "p:a", to: "o:you", bytes: 100 },
    { from: "p:b", to: "o:you", bytes: 200 },
    { from: "o:go", to: "k:build", bytes: 300 },
    { from: "o:you", to: "k:yours", bytes: 300 },
  ],
};

test("bands fill each node exactly, on both sides", () => {
  const { boxes, bands } = layout(flows, 800, 400);
  for (const n of boxes) {
    const out = bands.filter((b) => b.from === n.id).reduce((s, b) => s + b.w, 0);
    const inn = bands.filter((b) => b.to === n.id).reduce((s, b) => s + b.w, 0);
    if (n.column < 2) expect(out).toBeCloseTo(n.h, 5);
    if (n.column > 0) expect(inn).toBeCloseTo(n.h, 5);
  }
});

test("one scale everywhere, inside the height, columns apart", () => {
  const { boxes } = layout(flows, 800, 400);
  const lib = boxes.find((b) => b.id === "p:a")!;
  const go = boxes.find((b) => b.id === "o:go")!;
  expect(lib.h / 400).toBeCloseTo(go.h / 300, 5);
  for (const b of boxes) {
    expect(b.y).toBeGreaterThanOrEqual(0);
    expect(b.y + b.h).toBeLessThanOrEqual(400.0001);
  }
  expect(new Set(boxes.map((b) => b.x)).size).toBe(3);
});

test("labels of tiny nodes clear each other and stay inside", () => {
  const tiny: Flows = {
    total: 1000,
    nodes: [{ id: "p:a", label: "a", column: 0, bytes: 1000 }, ...["x", "y", "z", "w"].map((k, i) => ({ id: `o:${k}`, label: k, column: 1, bytes: i === 0 ? 997 : 1 }))],
    links: ["x", "y", "z", "w"].map((k, i) => ({ from: "p:a", to: `o:${k}`, bytes: i === 0 ? 997 : 1 })),
  };
  const col = layout(tiny, 600, 200).boxes.filter((b) => b.column === 1).sort((a, b) => a.labelY - b.labelY);
  for (let i = 1; i < col.length; i++) expect(col[i].labelY - col[i - 1].labelY).toBeGreaterThanOrEqual(LABEL_GAP - 1e-9);
  expect(col.at(-1)!.labelY).toBeLessThanOrEqual(200);
});
