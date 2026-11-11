import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Connect } from "./app/Connect.tsx";
import { endSplash } from "./app/splash.ts";
import { TipHost } from "./ui/Tip.tsx";
import { router } from "./router.tsx";
import "./styles/tokens.css";
import "./styles/app.css";

const queryClient = new QueryClient({ defaultOptions: { queries: { staleTime: 30_000, retry: 1, refetchOnWindowFocus: false } } });

// The router resolves once it has rendered the first page, failed or not, and before that frame is painted.
router.subscribe("onResolved", endSplash);

createRoot(document.getElementById("app")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      {/* Outside the router, so no route loader asks ebb anything before the page knows where ebb is. */}
      <Connect>
        <RouterProvider router={router} context={{ queryClient }} />
      </Connect>
      <TipHost />
    </QueryClientProvider>
  </StrictMode>,
);
