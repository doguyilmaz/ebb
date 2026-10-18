import { defineConfig } from "cf/config";

export default defineConfig({
  worker: {
    name: "ebb-site",
    compatibilityDate: "2026-10-06",
    domains: ["ebb.doguyilmaz.com"],
  },
});
