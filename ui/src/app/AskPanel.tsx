import { useMutation } from "@tanstack/react-query";
import { Loader2, Sparkles } from "lucide-react";
import { useState } from "react";
import { api } from "#/api/client.ts";
import { useStatus } from "#/api/hooks.ts";
import type { AskRequest, AskResponse } from "#/api/types.gen.ts";
import { Markdown } from "#/ui/Markdown.tsx";

const quick = ["Why is it so big?", "Is it safe to delete?", "How do I keep it smaller?"];

/** Ask the person's own Claude about a path or a finding; only ebb's metadata is sent. */
export function AskPanel({ target }: { target: Omit<AskRequest, "question"> }) {
  const status = useStatus();
  const [question, setQuestion] = useState("");
  const ask = useMutation({ mutationFn: (q: string) => api.post<AskResponse>("/ask", { ...target, question: q }) });
  if (!status.data?.askAvailable) return null;
  const send = (q: string) => {
    setQuestion(q);
    ask.mutate(q);
  };
  return (
    <section className="ask stack" style={{ gap: 8 }}>
      <div className="row" style={{ gap: 6 }}>
        <Sparkles size={14} aria-hidden className="faint" />
        <h3 className="panel-label">Ask Claude</h3>
      </div>
      <form
        className="row"
        onSubmit={(e) => {
          e.preventDefault();
          if (question.trim()) send(question.trim());
        }}
      >
        <input className="input grow" value={question} placeholder="Ask about this…" onChange={(e) => setQuestion(e.target.value)} aria-label="Question for Claude" />
        <button className="btn small" disabled={ask.isPending || !question.trim()}>Ask</button>
      </form>
      <div className="row" style={{ gap: 6, flexWrap: "wrap" }}>
        {quick.map((q) => (
          <button key={q} className="btn small ghost" disabled={ask.isPending} onClick={() => send(q)}>
            {q}
          </button>
        ))}
      </div>
      {ask.isPending && (
        <p className="muted row small-text" role="status">
          <Loader2 size={14} className="spin" aria-hidden /> Thinking…
        </p>
      )}
      {ask.error && <p className="error small-text">{(ask.error as Error).message}</p>}
      {ask.data && <Markdown text={ask.data.answer} />}
      <p className="faint small-text" data-tip="Paths, sizes, dates and what ebb's rules know. File contents never leave your Mac.">
        Sends only metadata to your own Claude.
      </p>
    </section>
  );
}
