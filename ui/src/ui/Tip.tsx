import { Info as InfoIcon } from "lucide-react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { term, type TermId } from "#/lib/glossary.ts";

interface Shown {
  value?: string;
  title?: string;
  body: string;
  target: HTMLElement;
  instant: boolean;
}

const DELAY = 450;
const GAP = 8;

/** data-term names a glossary entry, data-tip is plain text, data-value adds a figure on top (a size, a share). */
function content(el: HTMLElement): { value?: string; title?: string; body: string } | null {
  const value = el.dataset.value;
  const id = el.dataset.term;
  if (id) {
    const t = term(id);
    return t ? { value, title: t.title, body: t.body } : null;
  }
  return el.dataset.tip ? { value, body: el.dataset.tip } : null;
}

/**
 * One tooltip for the whole app: any element with data-term (a glossary id) or
 * data-tip (plain text) gets it on hover, keyboard focus or tap. After one tip
 * has shown, neighbours open at once, so scanning a row of them feels instant.
 */
export function TipHost() {
  const [shown, setShown] = useState<Shown | null>(null);
  const el = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    let current: HTMLElement | null = null;
    let pinned = false;
    let lastHide = 0;
    const find = (t: EventTarget | null) => (t instanceof Element ? t.closest<HTMLElement>("[data-term],[data-tip]") : null);
    const show = (target: HTMLElement, instant: boolean) => {
      const c = content(target);
      if (!c) return;
      current?.removeAttribute("aria-describedby");
      current = target;
      target.setAttribute("aria-describedby", "ebb-tip");
      setShown({ ...c, target, instant });
    };
    const hide = () => {
      clearTimeout(timer);
      if (!current) return;
      current.removeAttribute("aria-describedby");
      current = null;
      pinned = false;
      lastHide = performance.now();
      setShown(null);
    };
    const over = (e: PointerEvent) => {
      if (e.pointerType === "touch") return;
      const t = find(e.target);
      if (t === current) return;
      clearTimeout(timer);
      if (!t) {
        if (!pinned) hide();
        return;
      }
      const warm = current !== null || performance.now() - lastHide < 400;
      if (warm) show(t, true);
      else timer = setTimeout(() => show(t, false), DELAY);
    };
    const focus = (e: FocusEvent) => {
      const t = find(e.target);
      if (t && (e.target as Element).matches(":focus-visible")) show(t, true);
    };
    const blur = (e: FocusEvent) => {
      if (find(e.target) === current && !pinned) hide();
    };
    const click = (e: MouseEvent) => {
      const t = find(e.target);
      if (t?.classList.contains("info")) {
        if (current === t && pinned) return hide();
        show(t, true);
        pinned = true;
        return;
      }
      hide();
    };
    const key = (e: KeyboardEvent) => e.key === "Escape" && hide();
    addEventListener("pointerover", over);
    addEventListener("focusin", focus);
    addEventListener("focusout", blur);
    addEventListener("click", click, true);
    addEventListener("keydown", key);
    addEventListener("scroll", hide, true);
    return () => {
      clearTimeout(timer);
      removeEventListener("pointerover", over);
      removeEventListener("focusin", focus);
      removeEventListener("focusout", blur);
      removeEventListener("click", click, true);
      removeEventListener("keydown", key);
      removeEventListener("scroll", hide, true);
    };
  }, []);

  useLayoutEffect(() => {
    const tip = el.current;
    if (!tip) return;
    if (!shown) {
      if (tip.matches(":popover-open")) tip.hidePopover();
      return;
    }
    if (!tip.matches(":popover-open")) tip.showPopover();
    const r = shown.target.getBoundingClientRect();
    const w = tip.offsetWidth;
    const h = tip.offsetHeight;
    const below = r.bottom + GAP + h <= innerHeight - 8 || r.top - GAP - h < 8;
    const left = Math.max(8, Math.min(innerWidth - w - 8, r.left + r.width / 2 - w / 2));
    const top = below ? r.bottom + GAP : r.top - GAP - h;
    tip.style.left = `${left}px`;
    tip.style.top = `${top}px`;
    tip.style.transformOrigin = `${r.left + r.width / 2 - left}px ${below ? "0" : "100%"}`;
  }, [shown]);

  return (
    <div ref={el} id="ebb-tip" role="tooltip" popover="manual" className="tip" data-instant={shown?.instant || undefined}>
      {shown?.value && <b className="tip-value num">{shown.value}</b>}
      {shown?.title && <strong>{shown.title}</strong>}
      {shown && <span>{shown.body}</span>}
    </div>
  );
}

/** A small (i) that explains one glossary term on hover, focus or tap. */
export function Info({ term: id }: { term: TermId }) {
  return (
    <button type="button" className="info" data-term={id} aria-label={`About ${term(id)?.title ?? id}`}>
      <InfoIcon size={13} strokeWidth={2.2} aria-hidden />
    </button>
  );
}
