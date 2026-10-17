import type { ReactNode } from "react";

/**
 * Enough Markdown for an AI answer (paragraphs, headings, lists, code, bold,
 * italics), built as React elements: text is never parsed as HTML.
 */
export function Markdown({ text }: { text: string }) {
  const blocks: ReactNode[] = [];
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  for (let i = 0; i < lines.length; ) {
    const line = lines[i];
    if (line.startsWith("```")) {
      const code: string[] = [];
      for (i++; i < lines.length && !lines[i].startsWith("```"); i++) code.push(lines[i]);
      i++;
      blocks.push(<pre key={blocks.length}><code>{code.join("\n")}</code></pre>);
      continue;
    }
    const h = line.match(/^(#{1,4})\s+(.*)/);
    if (h) {
      blocks.push(<p key={blocks.length} className="md-h">{inline(h[2])}</p>);
      i++;
      continue;
    }
    const list = /^\s*([-*]|\d+\.)\s+/;
    if (list.test(line)) {
      const ordered = /^\s*\d+\./.test(line);
      const items: ReactNode[] = [];
      for (; i < lines.length && list.test(lines[i]); i++) items.push(<li key={items.length}>{inline(lines[i].replace(list, ""))}</li>);
      blocks.push(ordered ? <ol key={blocks.length}>{items}</ol> : <ul key={blocks.length}>{items}</ul>);
      continue;
    }
    if (!line.trim()) {
      i++;
      continue;
    }
    const para: string[] = [];
    for (; i < lines.length && lines[i].trim() && !list.test(lines[i]) && !lines[i].startsWith("```") && !/^#{1,4}\s/.test(lines[i]); i++) para.push(lines[i]);
    blocks.push(<p key={blocks.length}>{inline(para.join(" "))}</p>);
  }
  return <div className="md">{blocks}</div>;
}

function inline(s: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(`[^`]+`|\*\*[^*]+\*\*|\*[^*]+\*|_[^_]+_)/g;
  let last = 0;
  for (const m of s.matchAll(re)) {
    if (m.index > last) out.push(s.slice(last, m.index));
    const t = m[0];
    if (t.startsWith("`")) out.push(<code key={out.length}>{t.slice(1, -1)}</code>);
    else if (t.startsWith("**")) out.push(<strong key={out.length}>{t.slice(2, -2)}</strong>);
    else out.push(<em key={out.length}>{t.slice(1, -1)}</em>);
    last = m.index + t.length;
  }
  if (last < s.length) out.push(s.slice(last));
  return out;
}
