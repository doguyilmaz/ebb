import { useNavigate, useLocation } from "@tanstack/react-router";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { glideTo } from "#/lib/scroll.ts";
import { placeCard, steps, useTour, type Box } from "./tour.ts";

const same = (a: Box, b: Box) => a.top === b.top && a.left === b.left && a.width === b.width && a.height === b.height;

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
  // shown: the step's target has settled and its card is in place; moving: the spot glides to it once.
  const [shown, setShown] = useState(false);
  const [moving, setMoving] = useState(false);
  const cardEl = useRef<HTMLDivElement>(null);
  const nextBtn = useRef<HTMLButtonElement>(null);
  const s = step === null ? null : steps[step];

  useEffect(() => {
    // The tour overlay stays put while the page under it changes: no cross-fade.
    if (s && pathname !== s.route) navigate({ to: s.route, viewTransition: false });
  }, [s, pathname, navigate]);

  // Wait until the target exists and stops moving (its page loaded, the scroll to it ended), then move the
  // spot there once and show the card; afterwards follow scrolling and resizing directly.
  useEffect(() => {
    if (!s) return;
    setShown(false);
    let raf = 0;
    let glide: number | undefined;
    let stopGlide: (() => void) | null = null;
    let last: Box | null = null;
    let still = 0;
    let settled = false;
    const started = performance.now();
    const settle = (b: Box) => {
      settled = true;
      setMoving(true);
      setBox(b);
      setShown(true);
      glide = window.setTimeout(() => setMoving(false), 340);
    };
    const track = () => {
      const el = document.querySelector<HTMLElement>(`[data-tour="${s.target}"]`);
      if (el) {
        stopGlide ??= glideTo(el);
        const r = el.getBoundingClientRect();
        const b = { top: r.top, left: r.left, width: r.width, height: r.height };
        if (settled) setBox((p) => (p && same(p, b) ? p : b));
        else {
          still = last && same(last, b) ? still + 1 : 0;
          last = b;
          if (still >= 3) settle(b);
        }
      } else if (!settled && performance.now() - started > 4000) settle({ top: innerHeight / 2, left: innerWidth / 2, width: 0, height: 0 });
      raf = requestAnimationFrame(track);
    };
    raf = requestAnimationFrame(track);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(glide);
      stopGlide?.();
    };
  }, [s]);

  useLayoutEffect(() => {
    if (!box || !cardEl.current) return;
    const lit = { top: box.top - PAD, left: box.left - PAD, width: box.width + PAD * 2, height: box.height + PAD * 2 };
    setCard(box.width ? placeCard(lit, { w: CARD_W, h: cardEl.current.offsetHeight }, { w: innerWidth, h: innerHeight }) : { top: innerHeight / 2 - 100, left: innerWidth / 2 - CARD_W / 2 });
  }, [box, shown]);

  useEffect(() => {
    if (shown) nextBtn.current?.focus({ preventScroll: true });
  }, [shown, step]);

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
        style={{ top: card.top, left: card.left, width: CARD_W, visibility: shown ? "visible" : "hidden" }}
        data-shown={shown || undefined}
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
