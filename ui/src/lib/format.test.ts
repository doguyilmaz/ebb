import { describe, expect, test } from "bun:test";
import { ago, bytes, percent } from "./format.ts";

describe("format", () => {
  test("bytes use decimal units like Finder", () => {
    expect(bytes(0)).toBe("0 B");
    expect(bytes(1500)).toBe("1.5 kB");
    expect(bytes(35_200_000_000)).toBe("35.2 GB");
    expect(bytes(2_000_000)).toBe("2 MB");
  });
  test("percent", () => {
    expect(percent(0.1234)).toBe("12.3%");
    expect(percent(0)).toBe("0%");
  });
  test("ago", () => {
    const now = new Date("2026-10-17T12:00:00Z").getTime();
    expect(ago("2026-10-17T11:59:30Z", now)).toBe("just now");
    expect(ago("2026-10-17T09:00:00Z", now)).toBe("3 h ago");
    expect(ago("2026-10-10T12:00:00Z", now)).toBe("7 days ago");
  });
});
