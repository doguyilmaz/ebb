import { afterEach, beforeEach, describe, expect, test } from "bun:test";

type Call = { url: string; init?: RequestInit };
let calls: Call[] = [];
let responses: Response[] = [];
const realFetch = globalThis.fetch;

beforeEach(() => {
  calls = [];
  responses = [];
  globalThis.fetch = (async (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    return responses.shift() ?? new Response("{}", { status: 200 });
  }) as typeof fetch;
});
afterEach(() => {
  globalThis.fetch = realFetch;
});

const json = (v: unknown, status = 200) => new Response(JSON.stringify(v), { status });

describe("api client", () => {
  test("sends the bearer token and decodes errors", async () => {
    const c = await import("./client.ts");
    c.setTokenForTests("t0k");
    responses.push(json({ ok: 1 }), json({ error: "path is outside the scanned folder", code: "outside-root" }, 400));
    expect(await c.api.get<{ ok: number }>("/status")).toEqual({ ok: 1 });
    expect((calls[0].init?.headers as Record<string, string>).Authorization).toBe("Bearer t0k");
    const err = await c.api.get("/children").catch((e) => e);
    expect(err).toBeInstanceOf(c.ApiError);
    expect(err.status).toBe(400);
    expect(err.code).toBe("outside-root");
  });

  test("rejects a successful reply that is not JSON", async () => {
    const c = await import("./client.ts");
    responses.push(new Response("<!doctype html><html></html>", { status: 200, headers: { "Content-Type": "text/html" } }));
    const err = await c.api.get("/status").catch((e) => e);
    expect(err).toBeInstanceOf(c.ApiError);
  });

  test("pairing sends a stable nonce and stores the issued token", async () => {
    const c = await import("./client.ts");
    responses.push(json({ pending: true, code: "123456", confirm: "ebb web confirm 123456" }), json({ token: "issued" }));
    const first = await c.pair();
    expect(first).toEqual({ paired: false, code: "123456", confirm: "ebb web confirm 123456" });
    const second = await c.pair();
    expect(second).toEqual({ paired: true });
    const n1 = JSON.parse(String(calls[0].init?.body)).nonce;
    const n2 = JSON.parse(String(calls[1].init?.body)).nonce;
    expect(n1.length).toBeGreaterThanOrEqual(32);
    expect(n2).toBe(n1);
    expect(c.token()).toBe("issued");
  });
});
