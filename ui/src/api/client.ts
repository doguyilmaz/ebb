const TOKEN_KEY = "ebb.token";
const PORT_KEY = "ebb.port";
const NONCE_KEY = "ebb.nonce";
const DEFAULT_PORT = 47340;

function store(kind: "local" | "session"): Storage | null {
  try {
    return kind === "local" ? globalThis.localStorage : globalThis.sessionStorage;
  } catch {
    return null;
  }
}

// The local link carries the token in the hash, which never reaches the server.
function tokenFromURL(): string | null {
  if (typeof location === "undefined") return null;
  const m = location.hash.match(/token=([0-9a-f]+)/);
  if (!m) return null;
  history.replaceState(null, "", location.pathname + location.search);
  store("session")?.setItem(TOKEN_KEY, m[1]);
  return m[1];
}

const urlToken =
  tokenFromURL() ?? (import.meta.env?.VITE_EBB_TOKEN as string | undefined) ?? store("session")?.getItem(TOKEN_KEY) ?? "";

let base = "";
let tok = urlToken;

export const apiBase = () => base;
/** True when this page is hosted elsewhere and reaches ebb on this machine across origins. */
export const isRemote = () => base !== "";
export const token = () => tok;
export const setTokenForTests = (t: string) => void (tok = t);

export function apiPort(): number {
  return Number(store("local")?.getItem(PORT_KEY) ?? "") || DEFAULT_PORT;
}

export function setApiPort(port: number) {
  store("local")?.setItem(PORT_KEY, String(port));
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string,
  ) {
    super(message);
  }
}

export interface Health {
  ok: boolean;
  version?: string;
  allowed: boolean;
  device?: string;
  port?: number;
}

async function healthAt(at: string): Promise<Health | null> {
  try {
    const res = await fetch(`${at}/api/v1/health`, { cache: "no-store" });
    if (!res.ok) return null;
    const h = (await res.json()) as Health;
    return h?.ok ? h : null;
  } catch {
    return null;
  }
}

export interface Connection {
  remote: boolean;
  health: Health;
}

const EMBEDDED_KEY = "ebb.embedded";

/** True when ebb itself served this page earlier in this tab, so a failed check means it stopped. */
export const wasEmbedded = () => store("session")?.getItem(EMBEDDED_KEY) === "1";

/**
 * Finds ebb: same origin when it serves this page (or a dev server proxies to
 * it), else on loopback at the chosen port. Health names ebb's real port, which
 * is what the page shows, never the port the page itself came from.
 */
export async function connect(): Promise<Connection | null> {
  const here = await healthAt("");
  if (here) {
    base = "";
    tok = urlToken;
    if (here.port) {
      store("session")?.setItem(EMBEDDED_KEY, "1");
      setApiPort(here.port);
    }
    return { remote: false, health: here };
  }
  const at = `http://127.0.0.1:${apiPort()}`;
  const health = await healthAt(at);
  if (!health) return null;
  base = at;
  tok = store("local")?.getItem(TOKEN_KEY) ?? "";
  return { remote: true, health };
}

// A pairing request is bound to this nonce: ebb releases the token only to the
// page that asked, after someone at the machine confirmed the code it shows.
let memNonce = "";

function nonce(): string {
  let n = store("local")?.getItem(NONCE_KEY) ?? memNonce;
  if (!n) {
    const b = new Uint8Array(24);
    crypto.getRandomValues(b);
    n = Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
    store("local")?.setItem(NONCE_KEY, n);
  }
  memNonce = n;
  return n;
}

export type PairResult = { paired: true } | { paired: false; code: string; confirm: string };

export async function pair(): Promise<PairResult> {
  const res = await fetch(`${apiBase()}/api/v1/pair`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ nonce: nonce() }),
  });
  const body = (await res.json().catch(() => null)) as { token?: string; pending?: boolean; code?: string; confirm?: string; error?: string } | null;
  if (!res.ok) throw new ApiError(res.status, body?.error ?? res.statusText);
  if (body?.pending) return { paired: false, code: body.code ?? "", confirm: body.confirm ?? "ebb web confirm" };
  if (!body?.token) throw new ApiError(500, "ebb returned no token");
  tok = body.token;
  store("local")?.setItem(TOKEN_KEY, tok);
  store("local")?.removeItem(NONCE_KEY);
  memNonce = "";
  return { paired: true };
}

const rejected = new Set<() => void>();
/** Called when ebb no longer accepts this page's token. */
export const onAuthRejected = (f: () => void) => (rejected.add(f), () => void rejected.delete(f));

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${apiBase()}/api/v1${path}`, {
    method,
    headers: { Authorization: `Bearer ${tok}`, ...(body !== undefined ? { "Content-Type": "application/json" } : {}) },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) rejected.forEach((f) => f());
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { error: text };
  }
  if (!res.ok) {
    const e = data as { error?: string; code?: string } | null;
    throw new ApiError(res.status, e?.error ?? res.statusText, e?.code);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => call<T>("GET", path),
  post: <T>(path: string, body: unknown = {}) => call<T>("POST", path, body),
  del: <T>(path: string) => call<T>("DELETE", path),
};

/** Listens to ebb's server-sent events. */
export function subscribe(on: (name: string, data: unknown) => void, names: readonly string[]): () => void {
  const es = new EventSource(`${apiBase()}/api/v1/events?token=${tok}`);
  for (const n of names) {
    es.addEventListener(n, (e) => {
      let data: unknown = null;
      try {
        data = JSON.parse((e as MessageEvent).data ?? "null");
      } catch {
        data = null;
      }
      on(n, data);
    });
  }
  return () => es.close();
}
