import type { QueryClient } from "@tanstack/react-query";
import { useLive } from "#/lib/live.ts";
import { subscribe } from "./client.ts";
import { resultKeys } from "./hooks.ts";
import { EVENTS, type PlanEvent, type Progress } from "./types.gen.ts";

/** Wires ebb's event stream to the query cache and the live store. */
export function listen(qc: QueryClient): () => void {
  const live = useLive.getState();
  return subscribe((name, data) => {
    switch (name) {
      case "scan.progress":
        live.setScan(data as Progress);
        break;
      case "scan.done":
        live.setScan(null);
        resultKeys.forEach((k) => qc.invalidateQueries({ queryKey: k }));
        break;
      case "findings.changed":
        ["findings", "presets", "volume", "tree", "ages", "children"].forEach((k) => qc.invalidateQueries({ queryKey: [k] }));
        break;
      case "history.changed":
        qc.invalidateQueries({ queryKey: ["history"] });
        break;
      case "plan.progress":
        live.addPlanEvent(data as PlanEvent);
        break;
    }
  }, EVENTS);
}
