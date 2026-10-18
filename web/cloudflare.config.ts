import { defineConfig } from "cf/config";

export default defineConfig({
  worker: {
    name: "ebb-web",
    compatibilityDate: "2026-10-06",
    assets: { notFoundHandling: "single-page-application" },
  },
});
