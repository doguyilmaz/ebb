import { ExternalLink } from "lucide-react";
import type { Access } from "#/api/types.gen.ts";
import { Command } from "#/ui/Command.tsx";

export const FDA_SETTINGS = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles";

/** Exactly what to turn on: macOS asks about the app or binary the service runs, or the terminal ebb runs in. */
export function FdaSteps({ access }: { access: Access }) {
  if (access.kind === "app")
    return (
      <p className="muted">
        Turn on <b>ebb</b> in the list. Not there? Press +, choose ebb in Applications and turn it on.
      </p>
    );
  if (access.kind === "binary")
    return (
      <>
        <p className="muted">Press +, then ⌘⇧G and paste this path:</p>
        <Command value={access.path ?? ""} prompt={false} />
      </>
    );
  return <p className="muted">Turn on {access.name || "the terminal app you run ebb from"} in the list.</p>;
}

export function OpenFda({ primary }: { primary?: boolean }) {
  return (
    <a className={primary ? "btn primary" : "btn"} href={FDA_SETTINGS}>
      Open Full Disk Access <ExternalLink size={13} aria-hidden />
    </a>
  );
}
