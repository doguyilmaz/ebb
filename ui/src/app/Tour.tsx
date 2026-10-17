import { useNavigate, useLocation } from "@tanstack/react-router";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { glideTo } from "#/lib/scroll.ts";
import { placeCard, steps, useTour, type Box } from "./tour.ts";

const PAD = 8;
const CARD_W = 320;

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
  const [moving, setMoving] = useState(false);
  const cardEl = useRef<HTMLDivElement>(null);
  const nextBtn = useRef<HTMLButtonElement>(null);
  const s = step === null ? null : steps[step];

  useEffect(() => {
    if (s && pathname !== s.route) navigate({ to: s.route });
  }, [s, pathname, navigate]);

  // Follow the target while it loads, scrolls or resizes; give up on it after a few seconds.
  useEffect(() => {
    if (!s) return;
    setMoving(true);
    const settle = setTimeout(() => setMoving(false), 450);
    let raf = 0;
    let stopGlide: (() => void) | null = null;
    const started = performance.now();
    const track = () => {
      const el = document.querySelector<HTMLElement>(`[data-tour="${s.target}"]`);
      if (el) {
        stopGlide ??= glideTo(el);
        const r = el.getBoundingClientRect();
        setBox((b) => (b && b.top === r.top && b.left === r.left && b.width === r.width && b.height === r.height ? b : { top: r.top, left: r.left, width: r.width, height: r.height }));
      } else if (performance.now() - started > 4000) return setBox({ top: innerHeight / 2, left: innerWidth / 2, width: 0, height: 0 });
      raf = requestAnimationFrame(track);
    };
    raf = requestAnimationFrame(track);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(settle);
      stopGlide?.();
    };
  }, [s]);

  useLayoutEffect(() => {
    if (!box || !cardEl.current) return;
    const lit = { top: box.top - PAD, left: box.left - PAD, width: box.width + PAD * 2, height: box.height + PAD * 2 };
    setCard(box.width ? placeCard(lit, { w: CARD_W, h: cardEl.current.offsetHeight }, { w: innerWidth, h: innerHeight }) : { top: innerHeight / 2 - 100, left: innerWidth / 2 - CARD_W / 2 });
  }, [box]);

  const ready = !!box;
  useEffect(() => {
    if (ready) nextBtn.current?.focus({ preventScroll: true });
  }, [ready, step]);

  useEffect(() => {
    if (step === null) return;
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
    <div className="tour" role="presentation" onClick={(e) => e.target === e.currentTarget && stop()}>
      {box && (
        <div
          className="tour-spot"
          data-moving={moving || undefined}
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
