import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { defineConfig, type Plugin } from "vite";
import viteReact from "@vitejs/plugin-react";

// The release this build belongs to; the window compares it with the version of ebb it talks to.
process.env.VITE_EBB_VERSION ??= latestTag();

function latestTag(): string {
  try {
    return execFileSync("git", ["describe", "--tags", "--abbrev=0", "--match", "v*"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
  } catch {
    return "";
  }
}

const outDir = resolve(import.meta.dirname, "../internal/server/assets/dist");
const readmePath = resolve(outDir, "README");
const readme = readFileSync(readmePath, "utf8");

// go:embed needs at least one tracked file in dist; the build empties the directory, so put the marker back.
const keepReadme: Plugin = { name: "keep-readme", closeBundle: () => writeFileSync(readmePath, readme) };

export default defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [viteReact(), keepReadme],
  build: { outDir, emptyOutDir: true, sourcemap: false, chunkSizeWarningLimit: 900 },
  server: {
    proxy: { "/api": { target: `http://127.0.0.1:${process.env.EBB_PORT ?? "47340"}`, changeOrigin: false } },
  },
});
