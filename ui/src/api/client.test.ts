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
    c.setLinkKeyForTests("k".repeat(4));
    responses.push(json({ token: "issued" }));
    expect(await c.claim()).toBe("claimed");
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({ key: "kkkk" });
    expect(c.token()).toBe("issued");
    expect(await c.claim()).toBe("none");
    expect(calls.length).toBe(1);
  });

  test("an expired or used link is refused", async () => {
    const c = await import("./client.ts");
    c.setLinkKeyForTests("used");
    responses.push(json({ error: "this link has expired or was used already", code: "link-expired" }, 403));
    expect(await c.claim()).toBe("refused");
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
