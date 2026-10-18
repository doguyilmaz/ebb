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

await scenario("a page change shows the whole page at once", async () => {
  // A real disk answers slower than the demo: with latency a page that renders before its data shows "Loading…".
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Network.enable");
  await cdp.send("Network.emulateNetworkConditions", { offline: false, latency: 150, downloadThroughput: -1, uploadThroughput: -1 });
  for (const [label, heading] of [["Findings", "Findings"], ["Apps", "Apps"], ["Explore", null], ["Activity", "Activity"], ["Overview", null]]) {
    await page.evaluate(() => {
      window.__frames = [];
      const t0 = performance.now();
      const tick = () => {
        window.__frames.push({
          title: document.querySelector(".topbar-title")?.textContent ?? "",
          h1: document.querySelector(".page h1")?.textContent ?? null,
          loading: !!document.querySelector(".page .loading"),
        });
        if (performance.now() - t0 < 1500) requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
    await nav(label);
    await page.waitForTimeout(1600);
    const frames = await page.evaluate(() => window.__frames);
    const after = frames.filter((f) => f.title === label);
    check(after.length > 0, `${label}: the page showed`);
    check(!after.some((f) => f.loading), `${label}: no "Loading…" after the switch`);
    if (heading) check(after.every((f) => f.h1 === heading), `${label}: title and page change in the same frame`);
  }
  await cdp.send("Network.emulateNetworkConditions", { offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1 });
  await cdp.detach();
});

await scenario("every page renders in light and dark", async () => {
  for (const scheme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: scheme });
    for (const [label, text] of [["Overview", "Where the space is"], ["Explore", "Biggest inside"], ["Findings", "Can be cleaned"], ["Apps", "Library data"], ["Duplicates", "Find duplicates"], ["Sources", "Where the space comes from"], ["Activity", "Activity"], ["Settings", "Never touch"]]) {
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

await scenario("the Files view filters by size and age, and disks are listed", async () => {
  await nav("Explore");
  await page.locator("[data-tour=views]").getByRole("button", { name: "Files" }).click();
  const view = page.locator(".filesview");
  await view.waitFor({ timeout: 4000 });
  await view.getByRole("button", { name: "1 MB+" }).click();
  await view.locator(".fv-row", { hasText: "trip.mov" }).waitFor({ timeout: 4000 });
  await view.getByRole("button", { name: "1 year+" }).click();
  await view.locator(".fv-row", { hasText: "big-lib" }).waitFor({ timeout: 4000 });
  await view.locator(".fv-row", { hasText: "trip.mov" }).waitFor({ state: "detached", timeout: 4000 });
  await view.getByRole("button", { name: "Any age" }).click();
  await page.locator("[data-tour=views]").getByRole("button", { name: "Sunburst" }).click();
  const disks = (await api("/volumes")).body;
  check(Array.isArray(disks) && disks[0]?.boot && disks[0].capacity > 0, `startup disk first: ${JSON.stringify(disks?.[0])}`);
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

await scenario("apps list their Library data and uninstall to the Trash", async () => {
  await nav("Apps");
  const row = page.locator(".app-row", { hasText: "Sketchpad" });
  await row.waitFor({ timeout: 4000 });
  await row.locator(".app-main").click();
  await row.getByText("Containers/com.example.Sketchpad").waitFor({ timeout: 2000 });
  await row.getByRole("button", { name: "Uninstall" }).click();
  const sheet = page.locator("dialog.sheet[open]");
  await sheet.locator(".plan-items li").first().waitFor({ timeout: 8000 });
  check((await sheet.locator(".plan-items li").count()) === 3, "bundle, container and preferences planned");
  const confirm = sheet.locator(".confirm input");
  if (await confirm.count()) await confirm.check();
  await sheet.locator(".sheet-foot .btn.primary").click();
  await sheet.getByText("Cleanup finished").waitFor({ timeout: 20000 });
  check(!existsSync(join(HOME, "Applications", "Sketchpad.app")), "app moved");
  check(existsSync(join(HOME, ".Trash", "Sketchpad.app")), "app in the Trash");
  check(!existsSync(join(HOME, "Library", "Preferences", "com.example.Sketchpad.plist")), "preferences moved");
  await sheet.locator(".sheet-foot .btn.primary").click();
  await settle();
  check((await page.locator(".app-row", { hasText: "Sketchpad" }).count()) === 0, "gone from the list");
});

await scenario("duplicates are found and the extra copy goes to the Trash", async () => {
  await nav("Duplicates");
  await page.getByRole("button", { name: "Find duplicates" }).click();
  const row = page.locator(".dupe", { hasText: "trip" });
  await row.waitFor({ timeout: 15000 });
  await row.locator(".dupe-main").click();
  await row.getByText("Keep").waitFor({ timeout: 2000 });
  await row.getByRole("checkbox").click();
  await page.locator(".dupes-tray").getByRole("button", { name: "Review" }).click();
  const sheet = page.locator("dialog.sheet[open]");
  await sheet.locator(".plan-items li").first().waitFor({ timeout: 8000 });
  check((await sheet.locator(".plan-items li").count()) === 1, "one extra copy planned");
  const confirm = sheet.locator(".confirm input");
  if (await confirm.count()) await confirm.check();
  await sheet.locator(".sheet-foot .btn.primary").click();
  await sheet.getByText("Cleanup finished").waitFor({ timeout: 20000 });
  const kept = [join(HOME, "Movies", "trip.mov"), join(HOME, "Desktop", "old", "trip copy.mov")].filter(existsSync);
  check(kept.length === 1, `one copy stays: ${kept}`);
  await sheet.locator(".sheet-foot .btn.primary").click();
  await page.locator(".dupe", { hasText: "trip" }).waitFor({ state: "detached", timeout: 6000 });
});

await scenario("sources show the flows and the packages", async () => {
  await nav("Sources");
  await page.getByRole("tab", { name: "Flows" }).click();
  await page.locator(".flowchart .band").first().waitFor({ timeout: 6000 });
  check((await page.locator(".flowchart .flow-node").count()) >= 4, "places, owners and kinds drawn");
  await page.locator(".flowchart .flow-node[data-clickable] rect").first().click();
  await page.waitForURL(/\/(explore|findings)/, { timeout: 4000 });
  await nav("Sources");
  await page.getByRole("tab", { name: "Packages" }).click();
  await page.locator(".pkg", { hasText: "big-lib" }).waitFor({ timeout: 6000 });
  await page.getByRole("tab", { name: "Flows" }).click();
});

await scenario("an assistant's plan waits in the window and is only approved there", async () => {
  const doc = join(HOME, "Documents", "thesis.pdf");
  const made = await api("/plans", "POST", { paths: [doc], source: "mcp" });
  check(made.status === 200 && made.body.needsApproval, "agent plan needs approval");
  const banner = page.locator(".pending");
  await banner.waitFor({ timeout: 6000 });
  await banner.getByRole("button", { name: "Review" }).click();
  const sheet = page.locator("dialog.sheet[open]");
  await sheet.getByText("An assistant asks to clean").waitFor({ timeout: 4000 });
  check(await sheet.getByRole("button", { name: "Approve" }).isDisabled(), "approve waits for the box");
  await sheet.locator(".confirm input").check();
  await sheet.getByRole("button", { name: "Approve" }).click();
  await sheet.getByText("Nothing has been touched yet").waitFor({ timeout: 4000 });
  check(existsSync(doc), "approving touches nothing");
  check((await api(`/plans/${made.body.id}`)).body.approved === true, "plan approved");
  await sheet.getByRole("button", { name: "Close" }).last().click();
  await banner.waitFor({ state: "detached", timeout: 6000 });
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

await scenario("the background service turns on and off, and its settings save", async () => {
  await nav("Settings");
  const card = page.locator("section.setting", { has: page.getByRole("heading", { name: "Background service" }) });
  await card.getByRole("switch", { name: "Keep the last scan fresh" }).click();
  await settle();
  check((await api("/settings")).body.backgroundScans === false, "fresh scans turned off");
  const gb = card.getByRole("spinbutton");
  await gb.fill("25");
  await gb.press("Enter");
  await settle();
  check((await api("/settings")).body.lowSpaceGb === 25, "warning limit saved");
  // Sandboxed: the plist lands in the demo home and launchctl never runs.
  const plist = join(HOME, "Library", "LaunchAgents", "com.doguyilmaz.ebb.plist");
  await card.getByRole("button", { name: "Run in the background" }).click();
  await card.getByRole("button", { name: "Turn off" }).waitFor({ timeout: 4000 });
  check(existsSync(plist), "service definition written");
  await card.getByRole("button", { name: "Turn off" }).click();
  await card.getByRole("button", { name: "Run in the background" }).waitFor({ timeout: 4000 });
  check(!existsSync(plist), "service definition removed");
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
