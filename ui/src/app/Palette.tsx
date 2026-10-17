import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { usePresets, useSearch, useStartScan } from "#/api/hooks.ts";
import { bytes } from "#/lib/format.ts";
import { useSheet } from "#/lib/sheet.ts";

interface Command {
  id: string;
  label: string;
  hint?: string;
  run: () => void;
}

/** ⌘K: go anywhere, find any folder, start a scan or a preset cleanup. */
export function Palette() {
  const [open, setOpen] = useState(false);
  const [term, setTerm] = useState("");
  const [active, setActive] = useState(0);
  const openSheet = useSheet((s) => s.open);
  const dialog = useRef<HTMLDialogElement>(null);
  const navigate = useNavigate();
  const scan = useStartScan();
  const presets = usePresets();
  const found = useSearch(open ? term.trim() : "");

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(true);
      }
    };
    addEventListener("keydown", onKey);
    return () => removeEventListener("keydown", onKey);
  }, []);
  useEffect(() => {
    if (open) dialog.current?.showModal();
    else dialog.current?.close();
  }, [open]);

  const commands = useMemo<Command[]>(() => {
    const close = () => (setOpen(false), setTerm(""));
    const go = (to: "/" | "/explore" | "/findings" | "/activity" | "/settings") => () => (close(), navigate({ to }));
    const base: Command[] = [
      { id: "overview", label: "Overview", run: go("/") },
      { id: "explore", label: "Explore the disk", run: go("/explore") },
      { id: "findings", label: "Findings", run: go("/findings") },
      { id: "activity", label: "Activity", run: go("/activity") },
      { id: "settings", label: "Settings", run: go("/settings") },
      { id: "scan", label: "Scan again", run: () => (close(), scan.mutate({})) },
      ...(presets.data ?? [])
        .filter((p) => p.bytes > 0)
        .map((p) => ({ id: `preset-${p.id}`, label: `Clean: ${p.title}`, hint: bytes(p.bytes), run: () => (close(), openSheet({ preset: p.id })) })),
    ];
    const t = term.trim().toLowerCase();
    const matching = t ? base.filter((c) => c.label.toLowerCase().includes(t)) : base;
    const paths = (found.data?.items ?? []).map((it) => ({
      id: `path-${it.path}`,
      label: it.path,
      hint: bytes(it.alloc),
      run: () => (close(), navigate({ to: "/explore", search: { path: it.kind === "dir" ? it.path : it.path.slice(0, it.path.lastIndexOf("/")), view: "sunburst" } })),
    }));
    return [...matching, ...paths];
  }, [term, presets.data, found.data, navigate, scan, openSheet]);

  useEffect(() => setActive(0), [term]);

  return (
    <>
      <dialog ref={dialog} className="palette" onClose={() => setOpen(false)} aria-label="Command palette">
        <input
          autoFocus
          className="palette-input"
          placeholder="Go to, find a folder, or clean…"
          value={term}
          onChange={(e) => setTerm(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") (e.preventDefault(), setActive((a) => Math.min(commands.length - 1, a + 1)));
            if (e.key === "ArrowUp") (e.preventDefault(), setActive((a) => Math.max(0, a - 1)));
            if (e.key === "Enter") commands[active]?.run();
          }}
          aria-activedescendant={commands[active] ? `cmd-${active}` : undefined}
        />
        <ul className="palette-list" role="listbox">
          {commands.map((c, i) => (
            <li key={c.id} id={`cmd-${i}`} role="option" aria-selected={i === active}>
              <button onMouseEnter={() => setActive(i)} onClick={c.run}>
                <span className={c.id.startsWith("path-") ? "mono ellipsis" : "ellipsis"}>{c.label}</span>
                {c.hint && <span className="num muted">{c.hint}</span>}
              </button>
            </li>
          ))}
          {commands.length === 0 && <li className="muted palette-empty">Nothing matches</li>}
        </ul>
      </dialog>
    </>
  );
}
