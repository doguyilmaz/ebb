// Renders the brand SVGs to PNG with Chrome and builds the macOS .icns files.
//   python3 assets/brand/brand.py && node assets/brand/export.mjs
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "../../ui/node_modules/playwright-core/index.mjs";

const dir = dirname(fileURLToPath(import.meta.url));
const chrome = process.env.CHROMIUM ?? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const browser = await chromium.launch({ executablePath: chrome, headless: true });
const page = await browser.newPage({ viewport: { width: 1100, height: 1100 } });
const render = async (svg, size, out) => {
  const uri = "data:image/svg+xml;base64," + Buffer.from(readFileSync(join(dir, svg))).toString("base64");
  await page.setContent(`<body style="margin:0;background:transparent"><img src="${uri}" width="${size}" height="${size}" style="display:block"></body>`);
  const img = page.locator("img");
  await img.evaluate((el) => el.decode());
  await img.screenshot({ path: out, omitBackground: true });
};
const iconset = [16, 32, 128, 256, 512];
for (const variant of ["spectrum", "tide", "age"]) {
  const set = join(dir, "png", `ebb-${variant}.iconset`);
  rmSync(set, { recursive: true, force: true });
  mkdirSync(set, { recursive: true });
  for (const s of iconset) {
    await render(`icon-${variant}.svg`, s, join(set, `icon_${s}x${s}.png`));
    await render(`icon-${variant}.svg`, s * 2, join(set, `icon_${s}x${s}@2x.png`));
  }
  execFileSync("iconutil", ["-c", "icns", set, "-o", join(dir, `ebb-${variant}.icns`)]);
  await render(`icon-${variant}.svg`, 1024, join(dir, "png", `icon-${variant}-1024.png`));
  rmSync(set, { recursive: true, force: true });
}
await render("mark-spectrum.svg", 180, join(dir, "..", "..", "ui", "public", "apple-touch-icon.png"));
await render("mark-spectrum.svg", 512, join(dir, "png", "mark-spectrum-512.png"));
await browser.close();
