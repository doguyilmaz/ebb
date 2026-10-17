import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, createRoute, createRouter, lazyRouteComponent } from "@tanstack/react-router";
import { Root } from "./app/Root.tsx";

export type View = "sunburst" | "treemap" | "icicle" | "list";
const views: View[] = ["sunburst", "treemap", "icicle", "list"];

const rootRoute = createRootRouteWithContext<{ queryClient: QueryClient }>()({ component: Root });


export const exploreRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/explore",
  validateSearch: (s: Record<string, unknown>): { path?: string; view: View; unique?: boolean } => ({
    path: typeof s.path === "string" ? s.path : undefined,
    view: views.includes(s.view as View) ? (s.view as View) : "sunburst",
    unique: s.unique === true || s.unique === "true" ? true : undefined,
  }),
  component: lazyRouteComponent(() => import("./pages/explore/Explore.tsx")),
});

const overviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: lazyRouteComponent(() => import("./pages/overview/Overview.tsx")),
});
const findingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/findings",
  component: lazyRouteComponent(() => import("./pages/findings/Findings.tsx")),
});
const activityRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/activity",
  component: lazyRouteComponent(() => import("./pages/activity/Activity.tsx")),
});
const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  component: lazyRouteComponent(() => import("./pages/settings/Settings.tsx")),
});

const routeTree = rootRoute.addChildren([overviewRoute, exploreRoute, findingsRoute, activityRoute, settingsRoute]);

export const router = createRouter({ routeTree, context: { queryClient: undefined! }, defaultPreload: "intent" });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
