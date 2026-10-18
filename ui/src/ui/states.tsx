import { ApiError } from "#/api/client.ts";
import { Welcome } from "#/app/Welcome.tsx";

export function Loading({ label = "Loading…" }: { label?: string }) {
  return (
    <p className="loading muted" role="status">
      <span className="pulse" aria-hidden /> {label}
    </p>
  );
}

/** Turns an API error into a sentence with a way forward. */
export function Problem({ error }: { error: unknown }) {
  if (error instanceof ApiError && error.code === "never-scanned") return <NeverScanned />;
  if (error instanceof ApiError && error.status === 404 && error.message === "no such endpoint")
    return (
      <p className="error" role="alert">
        This needs a newer ebb than the one on this Mac. Run <code>brew upgrade ebb</code> and start ebb again.
      </p>
    );
  const msg = error instanceof Error ? error.message : String(error);
  return <p className="error" role="alert">{msg}</p>;
}

export function NeverScanned() {
  return <Welcome />;
}
