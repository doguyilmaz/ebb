import { useNavigate, useLocation } from "@tanstack/react-router";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { reducedMotion } from "#/lib/motion.ts";
import { steps, useTour } from "./tour.ts";

const PAD = 8;
const CARD_W = 340;

interface Box {
  top: number;
  left: number;
  width: number;
  height: number;
}

/**
 * The guided tour: dims the page, lights up one element per step and explains
 * it in a card beside it. It walks across pages on its own; arrow keys move,
 * Escape ends it.
 */
export function Tour() {
  const { step, next, back, stop } = useTour();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [box, setBox] = useState<Box | null>(null);
  const [card, setCard] = useState({ top: 0, left: 0 });
  const cardEl = useRef<HTMLDivElement>(null);
  const nextBtn = useRef<HTMLButtonElement>(null);
  const s = step === null ? null : steps[step];

  useEffect(() => {
    if (s && pathname !== s.route) navigate({ to: s.route });
  }, [s, pathname, navigate]);

  // Follow the target while it loads, scrolls or resizes; give up on it after a few seconds.
  useEffect(() => {
    if (!s) return;
    setBox(null);
    let raf = 0;
    let scrolled = false;
    const started = performance.now();
    const track = () => {
      const el = document.querySelector<HTMLElement>(`[data-tour="${s.target}"]`);
      if (el) {
        if (!scrolled) {
          el.scrollIntoView({ block: "center", behavior: reducedMotion() ? "auto" : "smooth" });
          scrolled = true;
        }
        const r = el.getBoundingClientRect();
        setBox((b) => (b && b.top === r.top && b.left === r.left && b.width === r.width && b.height === r.height ? b : { top: r.top, left: r.left, width: r.width, height: r.height }));
      } else if (performance.now() - started > 4000) return setBox({ top: innerHeight / 2, left: innerWidth / 2, width: 0, height: 0 });
      raf = requestAnimationFrame(track);
    };
    raf = requestAnimationFrame(track);
    return () => cancelAnimationFrame(raf);
  }, [s]);

  useLayoutEffect(() => {
    if (!box || !cardEl.current) return;
    const h = cardEl.current.offsetHeight;
    const below = box.top + box.height + PAD + 12 + h < innerHeight - 12;
    const right = box.left + box.width + PAD + 16 + CARD_W < innerWidth - 12;
    let top: number;
    let left: number;
    if (box.height > innerHeight * 0.55 && right) {
      top = Math.max(12, Math.min(innerHeight - h - 12, box.top + 24));
      left = box.left + box.width + PAD + 16;
    } else {
      top = below ? box.top + box.height + PAD + 12 : Math.max(12, box.top - PAD - 12 - h);
      left = Math.max(12, Math.min(innerWidth - CARD_W - 12, box.left));
    }
    setCard({ top, left });
  }, [box]);

  useEffect(() => {
    if (step === null) return;
    nextBtn.current?.focus({ preventScroll: true });
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") stop();
      else if (e.key === "ArrowRight") next();
      else if (e.key === "ArrowLeft") back();
      else return;
      e.preventDefault();
    };
    addEventListener("keydown", key);
    return () => removeEventListener("keydown", key);
  }, [step, next, back, stop]);

  if (!s || step === null) return null;
  const last = step === steps.length - 1;
  return (
    <div className="tour" role="presentation">
      {box && (
        <div
          className="tour-spot"
          style={{ top: box.top - PAD, left: box.left - PAD, width: box.width + PAD * 2, height: box.height + PAD * 2, opacity: box.width ? 1 : 0 }}
          aria-hidden
        />
      )}
      <div
        ref={cardEl}
        className="tour-card"
        role="dialog"
        aria-modal="false"
        aria-labelledby="tour-title"
        aria-describedby="tour-body"
        style={{ top: card.top, left: card.left, width: CARD_W, visibility: box ? "visible" : "hidden" }}
        key={step}
      >
        <span className="faint small-text num">
          {step + 1} of {steps.length}
        </span>
        <h2 id="tour-title">{s.title}</h2>
        <p id="tour-body" className="muted">{s.body}</p>
        <div className="tour-dots" aria-hidden>
          {steps.map((_, i) => (
            <i key={i} data-on={i === step || undefined} />
          ))}
        </div>
        <div className="row">
          <button className="btn ghost small" onClick={() => stop()}>Skip tour</button>
          <span className="grow" />
          {step > 0 && <button className="btn small" onClick={back}>Back</button>}
          <button ref={nextBtn} className="btn primary small" onClick={next}>{last ? "Done" : "Next"}</button>
        </div>
      </div>
    </div>
  );
}
