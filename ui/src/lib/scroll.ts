import { easeInOut, reducedMotion, tween } from "./motion.ts";

/**
 * Brings el to the middle of the content pane (the app scrolls there, not the
 * window), gliding unless motion is reduced. Returns a function that stops it.
 */
export function glideTo(el: HTMLElement, ms = 480): () => void {
  const pane = el.closest<HTMLElement>(".content");
  if (!pane) {
    el.scrollIntoView({ block: "center" });
    return () => {};
  }
  const r = el.getBoundingClientRect();
  const c = pane.getBoundingClientRect();
  const offset = r.height > c.height - 48 ? r.top - c.top - 16 : r.top + r.height / 2 - (c.top + c.height / 2);
  const from = pane.scrollTop;
  const to = Math.max(0, Math.min(pane.scrollHeight - pane.clientHeight, from + offset));
  if (Math.abs(to - from) < 2) return () => {};
  if (reducedMotion()) {
    pane.scrollTop = to;
    return () => {};
  }
  return tween(ms, (t) => (pane.scrollTop = from + (to - from) * t), easeInOut);
}
