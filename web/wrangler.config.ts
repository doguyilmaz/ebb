import { defineWranglerConfig } from "wrangler/experimental-config";

// The same build the Go binary embeds: run `bun run build` in ui/ first.
export default defineWranglerConfig({
  assetsDirectory: "../internal/server/assets/dist",
});
