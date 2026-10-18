import { Check as Tick, Minus } from "lucide-react";

/** A three-state checkbox: none, some (of a group) or all. */
export function Check({ state, label, onClick, tour }: { state: "none" | "some" | "all"; label: string; onClick: () => void; tour?: string }) {
  return (
    <button className="check" role="checkbox" aria-checked={state === "all" ? true : state === "some" ? "mixed" : false} aria-label={label} onClick={onClick} data-state={state} data-tour={tour}>
      {state === "all" ? <Tick size={13} strokeWidth={3} /> : state === "some" ? <Minus size={13} strokeWidth={3} /> : null}
    </button>
  );
}
