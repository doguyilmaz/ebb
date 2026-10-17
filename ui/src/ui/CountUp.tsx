import { useLayoutEffect, useRef } from "react";
import { reducedMotion, tween } from "#/lib/motion.ts";

const seen = new Map<string, number>();

/**
 * A number that rolls to its new value. It counts up the first time it shows
 * and afterwards only when the value changes, so revisiting a page stays calm
 * while a cleanup visibly lowers the total.
 */
export function CountUp({ id, value, format, ms = 900, className }: { id: string; value: number; format: (n: number) => string; ms?: number; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const from = seen.get(id) ?? 0;
    seen.set(id, value);
    if (from === value || reducedMotion()) {
      el.textContent = format(value);
      return;
    }
    return tween(ms, (t) => {
      el.textContent = format(from + (value - from) * t);
    });
  }, [id, value, format, ms]);
  return <span ref={ref} className={className} />;
}
