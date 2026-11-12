import { expect, test } from "bun:test";
import { QueryClient } from "@tanstack/react-query";
import { listen } from "./events.ts";

test("a cleanup from another client refreshes every affected view and the operation log", async () => {
  const fetchBefore = globalThis.fetch;
  const sourceBefore = Object.getOwnPropertyDescriptor(globalThis, "EventSource");
  const handlers = new Map<string, (event: { data: string }) => void>();
  let opened!: () => void;
  const ready = new Promise<void>((resolve) => (opened = resolve));
  Object.defineProperty(globalThis, "EventSource", { configurable: true, value: class {
    constructor() { opened(); }
    addEventListener(name: string, handler: (event: { data: string }) => void) { handlers.set(name, handler); }
    close() {}
  } });
  globalThis.fetch = (async () => new Response(JSON.stringify({ ticket: "test-ticket" }))) as typeof fetch;
  const qc = new QueryClient();
  const keys = ["files", "packages", "sources", "apps", "duplicates", "status", "ops"];
  for (const key of keys) qc.setQueryData([key], {});
  const off = listen(qc);
  try {
    await ready;
    handlers.get("plan.done")?.({ data: "{}" });
    for (const key of keys) expect(qc.getQueryState([key])?.isInvalidated).toBe(true);
  } finally {
    off();
    qc.clear();
    globalThis.fetch = fetchBefore;
    if (sourceBefore) Object.defineProperty(globalThis, "EventSource", sourceBefore);
    else Reflect.deleteProperty(globalThis, "EventSource");
  }
});
