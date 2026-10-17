import { Outlet } from "@tanstack/react-router";
import { Connect } from "./Connect.tsx";
import { Shell } from "./Shell.tsx";

export function Root() {
  return (
    <Connect>
      <Shell>
        <Outlet />
      </Shell>
    </Connect>
  );
}
