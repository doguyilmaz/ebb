/** Linear scale from a domain onto a range (a reversed range flips y). */
export function scale([d0, d1]: [number, number], [r0, r1]: [number, number]) {
  const k = d1 === d0 ? 0 : (r1 - r0) / (d1 - d0);
  return (v: number) => r0 + (v - d0) * k;
}

/** Round tick values (1, 2, 2.5 or 5 times a power of ten) that cover lo..hi in about n steps. */
export function niceTicks(lo: number, hi: number, n: number): number[] {
  if (hi <= lo) {
    const pad = Math.max(Math.abs(lo) * 0.01, 1e9);
    lo -= pad;
    hi += pad;
  }
  const raw = (hi - lo) / Math.max(1, n);
  const pow = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * pow).find((s) => s >= raw) ?? 10 * pow;
  const out: number[] = [];
  for (let v = Math.floor(lo / step) * step; v <= hi + step * 1e-9; v += step) out.push(v);
  if (out.at(-1)! < hi) out.push(out.at(-1)! + step);
  return out;
}
