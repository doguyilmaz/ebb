import { Check, Copy } from "lucide-react";
import { useState } from "react";

/** A command to run in Terminal (or, without the prompt, any text to paste), with a copy button. */
export function Command({ value, prompt = true }: { value: string; prompt?: boolean }) {
  const [done, setDone] = useState(false);
  return (
    <div className="cmd">
      {prompt && <span className="cmd-prompt" aria-hidden>$</span>}
      <code title={value}>{value}</code>
      <button
        type="button"
        className="icon-btn"
        aria-label={done ? "Copied" : `Copy ${value}`}
        data-tip={done ? undefined : "Copy"}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(value);
            setDone(true);
            setTimeout(() => setDone(false), 1400);
          } catch {
            setDone(false);
          }
        }}
      >
        {done ? <Check size={15} /> : <Copy size={15} />}
      </button>
    </div>
  );
}
