import { Outlet, useLocation } from "@tanstack/react-router";
import { Connect } from "./Connect.tsx";
import { ErrorBoundary } from "./ErrorBoundary.tsx";
import { Palette } from "./Palette.tsx";
import { SheetHost } from "./SheetHost.tsx";
import { Shell } from "./Shell.tsx";

export function Root() {
  const { pathname } = useLocation();
  return (
    <Connect>
      <Shell>
        <ErrorBoundary resetKey={pathname}>
          <Outlet />
        </ErrorBoundary>
      </Shell>
      <Palette />
      <SheetHost />
    </Connect>
  );
}
