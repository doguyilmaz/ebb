const TOKEN_KEY = "ebb.token";
const PORT_KEY = "ebb.port";
const DEFAULT_PORT = 47340;

function store(kind: "local" | "session"): Storage | null {
  try {
    return kind === "local" ? globalThis.localStorage : globalThis.sessionStorage;
  } catch {
    return null;
  }
}

export interface Link {
  token: string;
  key: string;
  port: number;
}

const secret = (v: string | null) => (v && /^[0-9a-f]{32,}$/.test(v) ? v : "");

/**
 * Reads the hash of a link ebb opened, which never reaches a server: the token for ebb's own window or a one-time
 * key a hosted page claims a token with, plus the port ebb listens on.
 */
export function readLink(hash: string): Link {
  const p = new URLSearchParams(hash.replace(/^#/, ""));
  const port = Number(p.get("port"));
  return { token: secret(p.get("token")), key: secret(p.get("key")), port: port > 0 && port < 65536 ? port : 0 };
}

function linkFromURL(): Link {
  if (typeof location === "undefined") return { token: "", key: "", port: 0 };
  const link = readLink(location.hash);
  if (!link.token && !link.key) return link;
  history.replaceState(null, "", location.pathname + location.search);
  if (link.token) store("session")?.setItem(TOKEN_KEY, link.token);
  if (link.port) setApiPort(link.port);
  return link;
}

const link = linkFromURL();
const urlToken = link.token || (import.meta.env?.VITE_EBB_TOKEN as string | undefined) || store("session")?.getItem(TOKEN_KEY) || "";
let linkKey = link.key;

let base = "";
let tok = urlToken;

export const apiBase = () => base;
/** True when this page is hosted elsewhere and reaches ebb on this machine across origins. */
export const isRemote = () => base !== "";
export const token = () => tok;
export const setTokenForTests = (t: string) => void (tok = t);
export const setLinkKeyForTests = (k: string) => void (linkKey = k);
/** True when this page connected to ebb before, so a failed check means ebb stopped rather than was never set up. */
export const pairedBefore = () => !!store("local")?.getItem(TOKEN_KEY);

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

/**
 * Trades the key from the link `ebb open` opened for a token of this page's own. The key works once, so it is
 * dropped after ebb answers; "refused" means the link expired or was used already.
 */
export async function claim(): Promise<"none" | "claimed" | "refused"> {
  if (!linkKey) return "none";
  const res = await fetch(`${apiBase()}/api/v1/pair`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key: linkKey }),
  });
  linkKey = "";
  const body = (await res.json().catch(() => null)) as { token?: string } | null;
  if (!res.ok || !body?.token) return "refused";
  tok = body.token;
  store("local")?.setItem(TOKEN_KEY, tok);
  return "claimed";
}

/** Calls f when another tab of this page stores a token, so a tab still waiting connects at once. */
export function onTokenFromOtherTab(f: () => void): () => void {
  const h = (e: StorageEvent) => void (e.key === TOKEN_KEY && e.newValue && f());
  addEventListener("storage", h);
  return () => removeEventListener("storage", h);
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
    // A 2xx page instead of JSON means something other than ebb answered, such as the hosted page's own server.
    if (res.ok) throw new ApiError(res.status, "The reply did not come from ebb");
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

/**
 * Listens to ebb's server-sent events. EventSource cannot send the token in a
 * header and it must not go in the URL, so each connection asks for a
 * one-time ticket first, and asks again when the stream drops.
 */
export function subscribe(on: (name: string, data: unknown) => void, names: readonly string[]): () => void {
  let es: EventSource | null = null;
  let retry: ReturnType<typeof setTimeout> | undefined;
  let closed = false;
  const open = async () => {
    try {
      const { ticket } = await api.post<{ ticket: string }>("/events/ticket");
      if (closed) return;
      es = new EventSource(`${apiBase()}/api/v1/events?ticket=${encodeURIComponent(ticket)}`);
    } catch {
      if (!closed) retry = setTimeout(open, 3000);
      return;
    }
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
    es.onerror = () => {
      es?.close();
      if (!closed) retry = setTimeout(open, 2000);
    };
  };
  void open();
  return () => {
    closed = true;
    clearTimeout(retry);
    es?.close();
  };
}
