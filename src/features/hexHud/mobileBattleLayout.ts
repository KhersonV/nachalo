"use client";

import { useEffect, useState, type RefObject } from "react";

// Keep the CSS media queries in sync. Wide, short touch screens include phones
// in landscape without changing the desktop HUD on short desktop windows.
export const mobileBattleQuery = "(max-width: 767px), (orientation: landscape) and (max-height: 500px) and (pointer: coarse)";
export const mobileOverlayEvent = "nachalo:hex-mobile-overlay";
export function openMobileOverlay(id: string) {
  window.dispatchEvent(new CustomEvent(mobileOverlayEvent, { detail: id }));
}

export function useMobileBattleHud() {
  const [mobile, setMobile] = useState(false);
  useEffect(() => {
    const query = window.matchMedia(mobileBattleQuery);
    const update = () => setMobile(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  return mobile;
}

// Presentation only: both map renderers already observe their container size.
// VisualViewport also covers iOS keyboard/browser chrome without page zoom locks.
export function useMobileBattleViewport(mobile: boolean, field: RefObject<HTMLElement | null>) {
  useEffect(() => {
    if (!mobile) return;
    const root = document.documentElement;
    const keys = ["--hex-mobile-height", "--hex-mobile-top", "--hex-mobile-field-top"];
    const previous = keys.map(key => root.style.getPropertyValue(key));
    const viewport = window.visualViewport;
    let frame = 0;
    const measure = () => {
      const values = [viewport?.height ?? window.innerHeight, viewport?.offsetTop ?? 0, field.current?.getBoundingClientRect().top ?? 0];
      keys.forEach((key, index) => {
        const value = `${values[index]}px`;
        if (root.style.getPropertyValue(key) !== value) root.style.setProperty(key, value);
      });
    };
    const schedule = () => { cancelAnimationFrame(frame); frame = requestAnimationFrame(measure); };
    const observer = new ResizeObserver(schedule);
    if (field.current) observer.observe(field.current);
    viewport?.addEventListener("resize", schedule);
    viewport?.addEventListener("scroll", schedule);
    window.addEventListener("resize", schedule);
    measure();
    return () => {
      cancelAnimationFrame(frame); observer.disconnect();
      viewport?.removeEventListener("resize", schedule);
      viewport?.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
      keys.forEach((key, index) => previous[index] ? root.style.setProperty(key, previous[index]) : root.style.removeProperty(key));
    };
  }, [mobile, field]);
}
