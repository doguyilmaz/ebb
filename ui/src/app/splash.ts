/**
 * The splash in index.html is the first thing the window paints: ebb's mark on the theme's background, before any
 * script runs. It sits outside the element React renders into, so React never replaces it. The window hides it
 * while it shows a screen of its own (a gate, in a layout effect) and ends it when the router has rendered the
 * first page (main.tsx); both happen before the next paint, so it never shares a frame with what takes its place.
 */
const splash = () => document.getElementById("splash");
let waiting: ReturnType<typeof setTimeout> | undefined;

export function showSplash(on: boolean) {
  clearTimeout(waiting);
  const el = splash();
  if (!el) return;
  el.hidden = !on;
  el.setAttribute("aria-busy", "true");
  const recovery = document.getElementById("splash-recovery");
  if (recovery) recovery.hidden = true;
  if (!on) return;
  const reload = document.getElementById("splash-reload");
  if (reload) reload.onclick = () => location.reload();
  waiting = setTimeout(() => {
    if (recovery) recovery.hidden = false;
    el.setAttribute("aria-busy", "false");
  }, 15_000);
}

/** The first page is on screen: the splash is never needed again, not even on a later page change. */
export function endSplash() {
  clearTimeout(waiting);
  splash()?.remove();
}
