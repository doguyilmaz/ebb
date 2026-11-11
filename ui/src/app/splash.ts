/**
 * The splash in index.html is the first thing the window paints: ebb's mark on the theme's background, before any
 * script runs. It sits outside the element React renders into, so React never replaces it. The window hides it
 * while it shows a screen of its own (a gate) and ends it once the first page is on screen; both happen in layout
 * effects, in the same frame as what takes its place.
 */
const splash = () => document.getElementById("splash");

export function showSplash(on: boolean) {
  const el = splash();
  if (el) el.hidden = !on;
}

/** The first page is on screen: the splash is never needed again, not even on a later page change. */
export function endSplash() {
  splash()?.remove();
}
