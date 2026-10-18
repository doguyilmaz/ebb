import type { View } from "#/router.tsx";

/** How deep and how many shapes each view asks for; shared by the page and the route's preload. */
export const depthFor = (v: View) => (v === "sunburst" ? 5 : 6);
export const budgetFor = (v: View) => (v === "treemap" ? 6000 : v === "icicle" ? 4000 : 3500);
