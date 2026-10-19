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

  test("claims a token once with the key from the link", async () => {
    const c = await import("./client.ts");
    c.setLinkForTests("k".repeat(4), 47341);
    responses.push(json({ token: "issued" }));
    expect(await c.claim()).toBe("claimed");
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({ key: "kkkk" });
    expect(c.token()).toBe("issued");
    expect(c.apiPort()).toBe(47341);
    expect(await c.claim()).toBe("none");
    expect(calls.length).toBe(1);
  });

  test("an expired or used link is refused", async () => {
    const c = await import("./client.ts");
    c.setLinkForTests("used", 8080);
    responses.push(json({ error: "this link has expired or was used already", code: "link-expired" }, 403));
    expect(await c.claim()).toBe("refused");
    expect(c.apiPort()).not.toBe(8080);
  });

  test("a link's port gets no token until ebb there takes the key", async () => {
    const c = await import("./client.ts");
    c.setTokenForTests("stored");
    c.setLinkForTests("k".repeat(4), 8080);
    responses.push(new Response("<!doctype html>", { status: 200 }), json({ ok: true, allowed: true }));
    const conn = await c.connect();
    expect(conn?.remote).toBe(true);
    expect(calls[1].url).toBe("http://127.0.0.1:8080/api/v1/health");
    expect(c.token()).toBe("");
    c.setLinkForTests("");
  });

  test("a link whose port has no ebb is dropped for the saved port", async () => {
    const c = await import("./client.ts");
    c.setTokenForTests("stored");
    c.setLinkForTests("k".repeat(4), 8080);
    responses.push(new Response("<!doctype html>", { status: 200 }), new Response("", { status: 502 }), json({ ok: true, allowed: true }));
    await c.connect();
    expect(calls[2].url).toBe(`http://127.0.0.1:${c.apiPort()}/api/v1/health`);
    expect(await c.claim()).toBe("none");
  });

  test("reads only well formed secrets and ports from a link", async () => {
    const { readLink } = await import("./client.ts");
    const key = "ab".repeat(24);
    expect(readLink(`#key=${key}&port=47341`)).toEqual({ token: "", key, port: 47341 });
    expect(readLink(`#token=${key}`)).toEqual({ token: key, key: "", port: 0 });
    expect(readLink("#key=<script>&port=99999")).toEqual({ token: "", key: "", port: 0 });
    expect(readLink("")).toEqual({ token: "", key: "", port: 0 });
  });
});
