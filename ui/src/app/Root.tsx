import { Outlet, useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";
import { ErrorBoundary } from "./ErrorBoundary.tsx";
import { FdaAsk } from "./FdaAsk.tsx";
import { Palette } from "./Palette.tsx";
import { SheetHost } from "./SheetHost.tsx";
import { Shell } from "./Shell.tsx";
import { endSplash } from "./splash.ts";
import { Tour } from "./Tour.tsx";

export function Root() {
  const { pathname } = useLocation();
  // The router mounts this with the first page and all its data, so the splash goes in the frame they appear.
  useLayoutEffect(endSplash, []);
  return (
    <>
      <Shell>
        <ErrorBoundary resetKey={pathname}>
          <Outlet />
        </ErrorBoundary>
      </Shell>
      <Palette />
      <SheetHost />
      <FdaAsk />
      <Tour />
    </>
  );
}
