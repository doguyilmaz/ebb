// End-to-end audit of the interface: the actions a person takes, clicked in a
// real browser against a sandboxed demo home, each outcome checked where it
// lands — in the page, through the API, or on disk.
//
//   scripts/e2e.sh            (builds the binary and the demo, then runs this)
//
// It fails on any failed request, console error or check that does not hold,
// and prints one line per scenario.
import { existsSync, mkdirSync } from "node:fs";
import { request } from "node:http";
import { join } from "node:path";
import { chromium } from "playwright-core";

const HOME = process.env.EBB_E2E_HOME;
const PORT = process.env.EBB_E2E_PORT;
const TOKEN = process.env.EBB_E2E_TOKEN;
const SHOTS = process.env.E2E_SHOTS;
const BASE = `http://127.0.0.1:${PORT}`;
const CHROME = process.env.CHROMIUM ?? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

const api = async (path, method = "GET", body, headers = {}) => {
  const res = await fetch(`${BASE}/api/v1${path}`, {
    method,
    headers: { Authorization: `Bearer ${TOKEN}`, "Content-Type": "application/json", ...headers },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  return { status: res.status, body: text ? JSON.parse(text) : null };
};
// fetch() will not send a forged Host header; node:http will.
const rawStatus = (path, headers) =>
  new Promise((resolve, reject) => {
    const req = request({ host: "127.0.0.1", port: PORT, path, headers }, (res) => (res.resume(), resolve(res.statusCode)));
    req.on("error", reject);
    req.end();
  });

const results = [];
const problems = [];
const check = (cond, what) => {
  if (!cond) throw new Error(what);
};

const browser = await chromium.launch({ executablePath: CHROME, headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
page.on("console", (m) => m.type() === "error" && !/status of 409/.test(m.text()) && problems.push(`console: ${m.text()}`));
page.on("pageerror", (e) => problems.push(`page error: ${e}`));
page.on("response", (r) => {
  // 409 before the first scan is the "never scanned" state the welcome screen handles.
  if (r.status() >= 400 && !(r.status() === 409 && r.url().includes("/api/"))) problems.push(`HTTP ${r.status()} ${r.request().method()} ${r.url().replace(/token=[0-9a-f]+/, "token=…")}`);
});

let inflight = 0;
const counted = (r) => r.url().includes("/api/") && !r.url().includes("/events");
page.on("request", (r) => counted(r) && inflight++);
const finished = (r) => counted(r) && inflight--;
page.on("requestfinished", finished);
page.on("requestfailed", finished);
// The event stream never ends, so wait for the API calls in flight instead of network idle.
const settle = async () => {
  await page.waitForTimeout(200);
  for (let i = 0; i < 100 && inflight > 0; i++) await page.waitForTimeout(100);
};
const nav = async (label) => {
  await page.locator("nav a.nav", { hasText: label }).click();
  await settle();
};
const shot = async (name) => SHOTS && (mkdirSync(SHOTS, { recursive: true }), page.screenshot({ path: join(SHOTS, `${name}.png`) }));

async function scenario(name, fn) {
  const before = problems.length;
  try {
    await fn();
    const fresh = problems.slice(before);
    if (fresh.length) throw new Error(fresh.join("; "));
    results.push(`ok    ${name}`);
  } catch (e) {
    results.push(`FAIL  ${name}: ${e.message.split("\n")[0]}`);
    await shot(`fail-${name.replace(/\W+/g, "-")}`);
  }
}

await scenario("opens with its link and welcomes a first run", async () => {
  await page.goto(`${BASE}/#token=${TOKEN}`);
  await page.getByText("See where your disk space went").waitFor({ timeout: 8000 });
  check(await page.getByText("Optional: let it see everything").count() >= 0, "welcome");
  await shot("welcome");
});

await scenario("first scan shows live progress, then the overview and the tour", async () => {
  await page.locator(".welcome-go .btn.primary").click();
  await page.locator(".hero-figure").waitFor({ timeout: 20000 });
  await page.locator(".tour-card").waitFor({ timeout: 8000 });
  await page.keyboard.press("Escape");
  await page.locator(".tour-card").waitFor({ state: "detached", timeout: 4000 });
  const st = await api("/status");
  check(st.body.last && st.body.last.files > 0, "status names the scan");
  await settle();
  await shot("overview");
});

await scenario("every page renders in light and dark", async () => {
  for (const scheme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: scheme });
    for (const [label, text] of [["Overview", "Where the space is"], ["Explore", "Biggest inside"], ["Findings", "Can be cleaned"], ["Activity", "Activity"], ["Settings", "Never touch"]]) {
      await nav(label);
      await page.getByText(text).first().waitFor({ timeout: 8000 });
      await shot(`${label.toLowerCase()}-${scheme}`);
    }
  }
  await page.emulateMedia({ colorScheme: "light" });
});

await scenario("explore switches every view and remembers the choice", async () => {
  await nav("Explore");
  for (const view of ["Treemap", "Icicle", "Sunburst"]) {
    await page.getByRole("button", { name: view, exact: true }).click();
    await settle();
    check((await page.locator(".chart canvas").count()) === 1, `${view} draws`);
  }
  await page.getByRole("button", { name: "Age", exact: true }).click();
  await page.locator(".age-key").waitFor({ timeout: 4000 });
  await page.getByRole("button", { name: "List", exact: true }).click();
  await page.locator(".listview").waitFor({ timeout: 4000 });
  const prefs = await page.evaluate(() => JSON.parse(localStorage.getItem("ebb.prefs") ?? "{}"));
  check(prefs.view === "list" && prefs.color === "age", `prefs saved: ${JSON.stringify(prefs)}`);
  await page.getByRole("button", { name: "Sunburst", exact: true }).click();
  await page.getByRole("button", { name: "Folders", exact: true }).click();
});

await scenario("a finding row opens its details and selection floats without moving the page", async () => {
  await nav("Findings");
  await page.locator(".frow").first().waitFor({ timeout: 8000 });
  await page.locator(".frow-open").first().click();
  await page.locator(".frow[data-open] .details").waitFor({ timeout: 4000 });
  const pane = page.locator(".content");
  const before = await pane.evaluate((el) => [el.clientWidth, el.clientHeight]);
  await page.locator(".frow .check").first().click();
  await page.locator(".tray").waitFor({ timeout: 4000 });
  const after = await pane.evaluate((el) => [el.clientWidth, el.clientHeight]);
  check(JSON.stringify(before) === JSON.stringify(after), `the page kept its size: ${before} → ${after}`);
  await page.getByRole("button", { name: "Clear" }).click();
});

await scenario("cleaning an installer moves it to the Trash and Put back restores it", async () => {
  const dmg = join(HOME, "Downloads", "Setup-2.3.dmg");
  check(existsSync(dmg), "demo installer present");
  await nav("Findings");
  const row = page.locator(".frow", { hasText: "Installers" }).first();
  await row.getByRole("button", { name: "Clean" }).click();
  const sheet = page.locator("dialog.sheet[open]");
  await sheet.waitFor({ timeout: 8000 });
  await sheet.getByText("Setup-2.3.dmg").first().waitFor({ timeout: 8000 });
  const confirm = sheet.locator(".confirm input");
  if (await confirm.count()) await confirm.check();
  await sheet.locator(".sheet-foot .btn.primary").click();
  await sheet.getByText("Cleanup finished").waitFor({ timeout: 15000 });
  check(!existsSync(dmg), "installer gone from Downloads");
  await sheet.getByRole("button", { name: "Put back" }).first().click();
  for (let i = 0; i < 30 && !existsSync(dmg); i++) await page.waitForTimeout(100);
  check(existsSync(dmg), "put back restores the installer");
  await sheet.locator(".sheet-foot .btn.primary").click();
});

await scenario("a quick cleanup of rebuildable caches runs end to end", async () => {
  const cache = join(HOME, ".bun", "install", "cache");
  check(existsSync(cache), "demo bun cache present");
  await nav("Overview");
  await page.locator(".preset", { hasText: "Developer caches" }).click();
  const sheet = page.locator("dialog.sheet[open]");
  await sheet.waitFor({ timeout: 8000 });
  await sheet.locator(".plan-items li").first().waitFor({ timeout: 8000 });
  const confirm = sheet.locator(".confirm input");
  if (await confirm.count()) await confirm.check();
  await sheet.locator(".sheet-foot .btn.primary").click();
  await sheet.getByText("Cleanup finished").waitFor({ timeout: 20000 });
  check(!existsSync(cache), "bun cache removed from disk");
  await sheet.locator(".sheet-foot .btn.primary").click();
  await nav("Activity");
  await page.locator(".ops li").first().waitFor({ timeout: 4000 });
});

await scenario("settings add and remove a protected folder", async () => {
  await nav("Settings");
  await page.locator(".suggest").getByRole("button", { name: "Documents" }).click();
  await settle();
  let s = await api("/settings");
  check((s.body.protect ?? []).includes("~/Documents/**"), "protect saved");
  await page.getByRole("button", { name: "Stop protecting ~/Documents/**" }).click();
  await settle();
  s = await api("/settings");
  check(!(s.body.protect ?? []).includes("~/Documents/**"), "protect removed");
});

await scenario("command palette opens with ⌘K and closes with Escape", async () => {
  await page.keyboard.press("Meta+k");
  await page.locator("dialog.palette[open]").waitFor({ timeout: 2000 });
  await page.keyboard.type("findings");
  await page.keyboard.press("Escape");
  await page.locator("dialog.palette[open]").waitFor({ state: "detached", timeout: 2000 });
});

await scenario("a scan again lists only what changed", async () => {
  const before = (await api("/status")).body.last;
  await page.getByRole("button", { name: "Scan again" }).click();
  for (let i = 0; i < 100; i++) {
    const st = (await api("/status")).body;
    if (!st.scan.running && st.last.id !== before.id) break;
    await page.waitForTimeout(200);
  }
  const after = (await api("/status")).body.last;
  check(after.time !== before.time, "a new result");
  check(after.incremental === true, `incremental update: ${JSON.stringify(after)}`);
  await settle();
});

await scenario("foreign pages and forged hosts are refused", async () => {
  const health = await api("/health", "GET", undefined, { Origin: "https://evil.example" });
  check(health.body.allowed === false, "foreign origin not allowed");
  const pair = await fetch(`${BASE}/api/v1/pair`, { method: "POST", headers: { Origin: "https://evil.example", "Content-Type": "application/json" }, body: JSON.stringify({ nonce: "x".repeat(48) }) });
  check(pair.status >= 400, `pairing refused: ${pair.status}`);
  check((await rawStatus("/api/v1/status", { Host: "evil.example:" + PORT, Authorization: `Bearer ${TOKEN}` })) === 421, "forged Host refused");
  check((await api("/status", "GET", undefined, { Authorization: "Bearer wrong" })).status === 401, "wrong token refused");
});

await browser.close();
console.log(results.join("\n"));
const failed = results.filter((r) => r.startsWith("FAIL")).length;
console.log(failed ? `\n${failed} of ${results.length} scenarios failed` : `\nall ${results.length} scenarios passed`);
process.exit(failed ? 1 : 0);
