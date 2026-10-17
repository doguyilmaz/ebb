import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { defineConfig, type Plugin } from "vite";
import viteReact from "@vitejs/plugin-react";

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
