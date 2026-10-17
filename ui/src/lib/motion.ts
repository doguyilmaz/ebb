/** Motion for canvas and number animations; CSS uses the same curves as tokens. */
export const easeOut = (t: number) => 1 - (1 - t) ** 5;
export const easeInOut = (t: number) => (t < 0.5 ? 8 * t ** 4 : 1 - (-2 * t + 2) ** 4 / 2);

export function reducedMotion(): boolean {
  return typeof matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/** Calls fn with eased progress each frame for ms; returns a cancel function. */
export function tween(ms: number, fn: (t: number) => void, ease = easeOut): () => void {
  if (ms <= 0) {
    fn(1);
    return () => {};
  }
  const start = performance.now();
  let id = 0;
  const step = (now: number) => {
    const t = Math.min(1, (now - start) / ms);
    fn(ease(t));
    if (t < 1) id = requestAnimationFrame(step);
  };
  id = requestAnimationFrame(step);
  return () => cancelAnimationFrame(id);
}
