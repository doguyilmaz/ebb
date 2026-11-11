import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";

const html = readFileSync(new URL("../../index.html", import.meta.url), "utf8");
const body = html.slice(html.indexOf("<body>"));

test("every script comes from a file, as the content security policy only allows those", () => {
  const scripts = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/g)];
  expect(scripts.length).toBeGreaterThan(0);
  for (const [, attrs, inner] of scripts) {
    expect(attrs).toMatch(/\bsrc="/);
    expect(inner.trim()).toBe("");
  }
});

test("the splash is painted before any script runs and stays outside what React replaces", () => {
  const splash = body.match(/<div id="splash"[^>]*>[\s\S]*?<\/div>/)?.[0] ?? "";
  expect(splash).not.toBe("");
  expect(splash).not.toMatch(/\shidden\b/);
  expect(splash).toMatch(/role="status"/);
  expect(splash).toMatch(/<img [^>]*alt=""/);
  expect(splash).toContain("ebb");
  // React empties #app when it first renders, which would put a new splash on screen.
  expect(body).toContain('<div id="app"></div>');
});

test("the splash uses the theme's colours, so it matches light, dark and a chosen theme", () => {
  const style = html.match(/<style>([\s\S]*?)<\/style>/)?.[1] ?? "";
  const rule = style.match(/\.splash \{([^}]*)\}/)?.[1] ?? "";
  expect(rule).toContain("background: var(--bg)");
  expect(rule).toContain("color: var(--text)");
  expect(style).not.toMatch(/#[0-9a-f]{3,8}\b/i);
});
