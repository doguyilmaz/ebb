import type { FetchQueryOptions, QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, createRoute, createRouter, lazyRouteComponent } from "@tanstack/react-router";
import { appsQuery, diffQuery, duplicatesQuery, findingsQuery, historyQuery, latestDiffQuery, opsQuery, settingsQuery, sliceQuery, statusQuery, volumeQuery } from "./api/hooks.ts";
import { Root } from "./app/Root.tsx";
import { readPrefs } from "./lib/prefs.ts";
import { budgetFor, depthFor } from "./pages/explore/params.ts";
import { baseline, widest } from "./pages/overview/since.ts";

export type View = "sunburst" | "treemap" | "icicle" | "list" | "files";
const views: View[] = ["sunburst", "treemap", "icicle", "list", "files"];

const rootRoute = createRootRouteWithContext<{ queryClient: QueryClient }>()({ component: Root });

/**
 * A page switches once its data is there (hovering a link already fetches it), so it appears whole instead of
 * flashing "Loading…" and shifting. Failures are left to the page, which explains them.
 */
const preload =
  (...queries: FetchQueryOptions<any, any, any, any>[]) =>
  ({ context }: { context: { queryClient: QueryClient } }) =>
    Promise.all(queries.map((q) => context.queryClient.ensureQueryData(q).catch(() => undefined))).then(() => undefined);

export const exploreRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/explore",
  validateSearch: (s: Record<string, unknown>): { path?: string; view?: View; unique?: boolean; color?: "age" | "folder" } => ({
    path: typeof s.path === "string" ? s.path : undefined,
    view: views.includes(s.view as View) ? (s.view as View) : undefined,
    unique: s.unique === true || s.unique === "true" ? true : s.unique === false || s.unique === "false" ? false : undefined,
    color: s.color === "age" || s.color === "folder" ? s.color : undefined,
  }),
  loaderDeps: ({ search }) => ({ path: search.path, view: search.view }),
  loader: ({ context, deps }) => {
    const view = deps.view ?? readPrefs().view ?? "sunburst";
    const qs: FetchQueryOptions<any, any, any, any>[] = [statusQuery, volumeQuery];
    if (view !== "list" && view !== "files") qs.push(sliceQuery(deps.path, depthFor(view), budgetFor(view)));
    return preload(...qs)({ context });
  },
  component: lazyRouteComponent(() => import("./pages/explore/Explore.tsx")),
});

const overviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  loader: async ({ context }) => {
    await preload(statusQuery, volumeQuery, findingsQuery(), sliceQuery(undefined, 1, 60), latestDiffQuery, historyQuery)({ context });
    // The "What changed" card's comparison too, so it does not arrive after the page.
    const snaps = context.queryClient.getQueryData(historyQuery.queryKey)?.snapshots ?? [];
    const pair = baseline(snaps, readPrefs().since ?? widest(snaps));
    if (pair) await preload(diffQuery(pair.from.id, pair.to.id))({ context });
  },
  component: lazyRouteComponent(() => import("./pages/overview/Overview.tsx")),
});
const findingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/findings",
  loader: preload(findingsQuery()),
  component: lazyRouteComponent(() => import("./pages/findings/Findings.tsx")),
});
const appsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/apps",
  loader: preload(appsQuery),
  component: lazyRouteComponent(() => import("./pages/apps/Apps.tsx")),
});
const duplicatesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/duplicates",
  loader: preload(duplicatesQuery),
  component: lazyRouteComponent(() => import("./pages/duplicates/Duplicates.tsx")),
});
const historyRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/history",
  loader: preload(historyQuery),
  component: lazyRouteComponent(() => import("./pages/history/HistoryPage.tsx")),
});
const activityRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/activity",
  loader: preload(opsQuery),
  component: lazyRouteComponent(() => import("./pages/activity/Activity.tsx")),
});
const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  loader: preload(settingsQuery, statusQuery),
  component: lazyRouteComponent(() => import("./pages/settings/Settings.tsx")),
});

const routeTree = rootRoute.addChildren([overviewRoute, exploreRoute, findingsRoute, appsRoute, duplicatesRoute, historyRoute, activityRoute, settingsRoute]);

export const router = createRouter({
  routeTree,
  context: { queryClient: undefined! },
  defaultPreload: "intent",
  // Loaders only fill the query cache, which decides freshness; the router never caches on its own.
  defaultPreloadStaleTime: 0,
  // Pages cross-fade as a whole; a change of search params only (Explore zooming, filters) keeps its own animation.
  defaultViewTransition: { types: ({ pathChanged }) => (pathChanged ? ["page"] : false) },
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
