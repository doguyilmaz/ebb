import { expect, test } from "bun:test";
import { frame, plan } from "./morph.ts";

interface S {
  key: string;
  x: number;
}
const lerp = (a: S, b: S, t: number): S => ({ key: b.key, x: a.x + (b.x - a.x) * t });
const away = (s: S): S => ({ ...s, x: s.x + 100 });

test("matched shapes interpolate, new ones fade in, gone ones fade out", () => {
  const tr = plan<S>([{ key: "a", x: 0 }, { key: "gone", x: 5 }], [{ key: "a", x: 10 }, { key: "new", x: 20 }], away, away);
  const mid = frame(tr, 0.5, lerp);
  expect(mid.find((f) => f.shape.key === "a")).toEqual({ shape: { key: "a", x: 5 }, alpha: 1 });
  expect(mid.find((f) => f.shape.key === "new")).toEqual({ shape: { key: "new", x: 70 }, alpha: 0.5 });
  expect(mid.find((f) => f.shape.key === "gone")).toEqual({ shape: { key: "gone", x: 55 }, alpha: 0.5 });
});

test("the last frame is exactly the target, without exiting shapes", () => {
  const to = [{ key: "a", x: 10 }, { key: "new", x: 20 }];
  const tr = plan<S>([{ key: "a", x: 0 }, { key: "gone", x: 5 }], to, away, away);
  expect(frame(tr, 1, lerp).map((f) => f.shape)).toEqual(to);
  expect(frame(tr, 0, lerp).filter((f) => f.alpha > 0).map((f) => f.shape.key).sort()).toEqual(["a", "gone"]);
});
