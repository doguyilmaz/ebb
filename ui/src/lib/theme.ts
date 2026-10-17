import { useEffect, useState } from "react";

/** Changes whenever the colour scheme does (system setting or the in-app theme), for code that reads colours. */
export function useSchemeVersion(): number {
  const [n, setN] = useState(0);
  useEffect(() => {
    const bump = () => setN((v) => v + 1);
    const mq = matchMedia("(prefers-color-scheme: dark)");
    mq.addEventListener("change", bump);
    const mo = new MutationObserver(bump);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    return () => {
      mq.removeEventListener("change", bump);
      mo.disconnect();
    };
  }, []);
  return n;
}

const seen = new Set<string>();

/** True the first time key is asked for in this session, so entrance animations play once rather than on every visit. */
export function firstTime(key: string): boolean {
  if (seen.has(key)) return false;
  seen.add(key);
  return true;
}
