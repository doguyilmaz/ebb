import { ApiError } from "#/api/client.ts";
import { useStartScan } from "#/api/hooks.ts";
import { ScanSearch } from "lucide-react";

export function Loading({ label = "Loading…" }: { label?: string }) {
  return <p className="muted" role="status">{label}</p>;
}

/** Turns an API error into a sentence with a way forward. */
export function Problem({ error }: { error: unknown }) {
  if (error instanceof ApiError && error.code === "never-scanned") return <NeverScanned />;
  const msg = error instanceof Error ? error.message : String(error);
  return <p className="error" role="alert">{msg}</p>;
}

export function NeverScanned() {
  const start = useStartScan();
  return (
    <div className="empty">
      <ScanSearch size={34} aria-hidden />
      <h1>See where your space went</h1>
      <p>ebb reads the whole disk once (about a minute) and explains what is large and whether it is safe to remove.</p>
      <button className="btn primary" disabled={start.isPending} onClick={() => start.mutate({})}>
        Scan this Mac
      </button>
    </div>
  );
}
