export type OS = "mac" | "linux" | "windows" | "mobile";

/** The visitor's system, as far as the browser says. */
export function detectOS(): OS {
  const nav = navigator as Navigator & { userAgentData?: { platform?: string; mobile?: boolean } };
  const p = `${nav.userAgentData?.platform ?? ""} ${navigator.platform} ${navigator.userAgent}`.toLowerCase();
  if (nav.userAgentData?.mobile || /iphone|ipad|android/.test(p)) return "mobile";
  if (p.includes("win")) return "windows";
  if (p.includes("mac")) return "mac";
  return "linux";
}

export const isSafari = () => /^((?!chrome|android|crios|fxios|edg).)*safari/i.test(navigator.userAgent);
