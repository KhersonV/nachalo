"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import styles from "./DraggableHudPanel.module.css";

export type HudMode = "expanded" | "collapsed" | "closed";
type Layout = { mode: HudMode; x?: number; y?: number };
const prefix = "nachalo.hex.hud.v1.";
const resetEvent = "nachalo:reset-hex-hud";
export function resetHudLayout() {
  try { Object.keys(localStorage).filter(key => key.startsWith(prefix)).forEach(key => localStorage.removeItem(key)); } catch { /* Storage may be disabled. */ }
  window.dispatchEvent(new Event(resetEvent));
}
export function useDesktopHud() {
  const [desktop, setDesktop] = useState(false);
  useEffect(() => {
    const query = window.matchMedia("(min-width: 1024px)");
    const update = () => setDesktop(query.matches);
    update(); query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  return desktop;
}
export function useHudPanel(id: string, initialMode: HudMode = "collapsed") {
  const [layout, setLayout] = useState<Layout>({ mode: initialMode });
  const [ready, setReady] = useState(false);
  useEffect(() => {
    try {
      const raw: unknown = JSON.parse(localStorage.getItem(prefix + id) || "null");
      if (raw && typeof raw === "object" && "mode" in raw && (raw.mode === "expanded" || raw.mode === "collapsed" || raw.mode === "closed")) {
        setLayout({ mode: raw.mode, x: "x" in raw && typeof raw.x === "number" && Number.isFinite(raw.x) ? raw.x : undefined, y: "y" in raw && typeof raw.y === "number" && Number.isFinite(raw.y) ? raw.y : undefined });
      }
    } catch { /* Ignore unavailable storage and older invalid layouts. */ }
    setReady(true);
    const reset = () => setLayout({ mode: initialMode });
    window.addEventListener(resetEvent, reset);
    return () => window.removeEventListener(resetEvent, reset);
  }, [id, initialMode]);
  useEffect(() => {
    if (!ready) return;
    try { localStorage.setItem(prefix + id, JSON.stringify(layout)); } catch { /* Layout persistence is optional. */ }
  }, [id, layout, ready]);
  const setMode = useCallback((mode: HudMode) => setLayout(current => ({ ...current, mode })), []);
  return { layout, setLayout, setMode };
}
type PanelState = ReturnType<typeof useHudPanel>;
type Props = { panel: PanelState; title: ReactNode; children: ReactNode; compact?: ReactNode; anchor?: "hero" | "target" | "quest" | "chat" | "participants"; width?: number; closable?: boolean; enabled?: boolean; onModeChange?: (mode: HudMode) => void };

/** One pointer implementation for every floating HUD window. Only the header starts a drag. */
export default function DraggableHudPanel({ panel, title, children, compact, anchor = "target", width = 280, closable = true, enabled = true, onModeChange }: Props) {
  const ref = useRef<HTMLElement>(null);
  const drag = useRef<{ id: number; x: number; y: number; left: number; top: number } | null>(null);
  const { layout, setLayout, setMode } = panel;
  const mode = !closable && layout.mode === "closed" ? "collapsed" : layout.mode;
  const change = (mode: HudMode) => { setMode(mode); onModeChange?.(mode); };
  const clampPosition = useCallback((x: number, y: number) => {
    const rect = ref.current?.getBoundingClientRect();
    const minY = Math.min(76, Math.max(8, window.innerHeight - 110));
    return { x: Math.max(8, Math.min(x, window.innerWidth - (rect?.width ?? width) - 8)), y: Math.max(minY, Math.min(y, window.innerHeight - 80 - (rect?.height ?? 38))) };
  }, [width]);
  useEffect(() => {
    if (!enabled || mode === "closed") return;
    const clamp = () => {
      const rect = ref.current?.getBoundingClientRect();
      if (!rect) return;
      const position = clampPosition(rect.left, rect.top);
      // Keep default anchors responsive until the user actually moves a panel.
      if (layout.x === undefined && layout.y === undefined && position.x === rect.left && position.y === rect.top) return;
      setLayout(current => current.x === position.x && current.y === position.y ? current : { ...current, ...position });
    };
    const observer = new ResizeObserver(clamp);
    if (ref.current) observer.observe(ref.current);
    window.addEventListener("resize", clamp); clamp();
    return () => { observer.disconnect(); window.removeEventListener("resize", clamp); };
  }, [enabled, mode, layout.x, layout.y, setLayout, clampPosition]);
  if (!enabled) return <>{children}</>;
  if (mode === "closed") return <button type="button" className={`${styles.reopen} ${styles[anchor]}`} onClick={() => change("collapsed")}>{title} ↗</button>;
  return <section ref={ref} className={`${styles.panel} ${styles[anchor]} ${mode === "collapsed" ? styles.collapsed : ""}`} style={{ width: mode === "collapsed" ? anchor === "chat" ? 140 : Math.min(width, 272) : width, ...(layout.x !== undefined ? { left: layout.x, right: "auto" } : {}), ...(layout.y !== undefined ? { top: layout.y, bottom: "auto" } : {}) }}
    onPointerDown={event => event.stopPropagation()} onClick={event => event.stopPropagation()} onDoubleClick={event => event.stopPropagation()} onWheel={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()}>
    <header className={styles.header}
      onPointerDown={event => {
        if (event.button !== 0 || (event.target instanceof Element && event.target.closest("button,a,input,textarea,select,[contenteditable]"))) return;
        const rect = ref.current?.getBoundingClientRect(); if (!rect) return;
        event.preventDefault(); event.currentTarget.setPointerCapture(event.pointerId);
        drag.current = { id: event.pointerId, x: event.clientX, y: event.clientY, left: rect.left, top: rect.top };
      }}
      onPointerMove={event => { const start = drag.current; if (!start || start.id !== event.pointerId) return; const position = clampPosition(start.left + event.clientX - start.x, start.top + event.clientY - start.y); setLayout(current => ({ ...current, ...position })); }}
      onPointerUp={event => { drag.current = null; if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId); }}
      onPointerCancel={() => { drag.current = null; }} onLostPointerCapture={() => { drag.current = null; }}>
      <span className={styles.title} title={typeof title === "string" ? title : undefined}>{title}</span>
      <button type="button" aria-label={mode === "expanded" ? "Свернуть / Collapse" : "Развернуть / Expand"} aria-expanded={mode === "expanded"} onClick={() => change(mode === "expanded" ? "collapsed" : "expanded")}>{mode === "expanded" ? "−" : "+"}</button>
      {closable && <button type="button" aria-label="Закрыть / Close" onClick={() => change("closed")}>×</button>}
    </header>
    {compact && mode === "collapsed" && <div className={styles.compact}>{compact}</div>}
    <div className={styles.body} hidden={mode !== "expanded"}>{children}</div>
  </section>;
}
