import { Search as Icon } from "lucide-react";
import { useState } from "react";
import { useSearch } from "#/api/hooks.ts";
import { bytes } from "#/lib/format.ts";

/** Finds folders and large files by name (glob or text) across the whole scan. */
export function Search({ onOpen }: { onOpen: (p: string) => void }) {
  const [term, setTerm] = useState("");
  const found = useSearch(term.trim());
  const items = found.data?.items ?? [];
  return (
    <div className="search">
      <Icon size={14} aria-hidden className="search-icon" />
      <input
        className="input"
        placeholder="Find by name, e.g. node_modules or *.dmg"
        value={term}
        onChange={(e) => setTerm(e.target.value)}
        onKeyDown={(e) => e.key === "Escape" && setTerm("")}
        aria-label="Find by name"
      />
      {term.trim().length > 1 && (
        <ul className="search-results panel" role="listbox">
          {items.length === 0 && <li className="muted">{found.isFetching ? "Searching…" : "Nothing found"}</li>}
          {items.map((it) => (
            <li key={it.path}>
              <button
                onClick={() => {
                  setTerm("");
                  onOpen(it.kind === "dir" ? it.path : it.path.slice(0, it.path.lastIndexOf("/")));
                }}
              >
                <span className="num">{bytes(it.alloc)}</span>
                <span className="mono ellipsis">{it.path}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
