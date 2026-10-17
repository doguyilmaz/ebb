import { HardDrive } from "lucide-react";

/** The scanned disk first, then each folder down to the current one. */
export function Breadcrumb({ path, root, onOpen }: { path: string; root?: string; onOpen: (p: string | undefined) => void }) {
  const inRoot = root && (path === root || path.startsWith(root + "/"));
  const rest = inRoot ? path.slice(root!.length) : path;
  const parts = rest.split("/").filter(Boolean);
  const prefix = inRoot ? root! : "";
  return (
    <nav className="crumbs" aria-label="Folder">
      <button className="crumb" onClick={() => onOpen(undefined)}>
        <HardDrive size={14} aria-hidden /> Disk
      </button>
      {parts.map((p, i) => {
        const target = `${prefix}/${parts.slice(0, i + 1).join("/")}`;
        const last = i === parts.length - 1;
        return (
          <span key={target} className="crumb-wrap">
            <span className="faint">/</span>
            <button className="crumb" aria-current={last ? "page" : undefined} onClick={() => onOpen(target)}>
              {p}
            </button>
          </span>
        );
      })}
    </nav>
  );
}
