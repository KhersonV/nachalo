"use client";

import { useCallback, useLayoutEffect, useRef, useState } from "react";
import type { PointerEvent, WheelEvent, MouseEvent } from "react";

export type CameraPoint = Readonly<{ x: number; y: number }>;
type Camera = { x: number; y: number; zoom: number };
type Drag = { x: number; y: number; camera: Camera; moved: boolean };

const MIN_ZOOM = 0.65;
const MAX_ZOOM = 2.4;
const DRAG_THRESHOLD = 6; // Preserve mouse drag behavior.
const TOUCH_DRAG_THRESHOLD = 8;
const TAP_DURATION = 500;

export function useHexCamera(hero: CameraPoint, onTouchTap?: (point: CameraPoint, svg: SVGSVGElement) => void) {
  const svgRef = useRef<SVGSVGElement>(null);
  const drag = useRef<Drag | null>(null);
  const suppressClick = useRef(false);
  const [size, setSize] = useState({ width: 1000, height: 700 });
  const [camera, setCamera] = useState<Camera>(() => ({ x: hero.x, y: hero.y, zoom: 1 }));
  const [panning, setPanning] = useState(false);
  const liveCamera = useRef(camera);
  const tapHandler = useRef(onTouchTap);
  tapHandler.current = onTouchTap;
  const touches = useRef(new Map<number, CameraPoint>());
  const touchStart = useRef<{ point: CameraPoint; camera: Camera } | null>(null);
  const pinch = useRef<{ span: number; camera: Camera; anchor: CameraPoint } | null>(null);
  const touchBlockedTap = useRef(false);
  const touchStartedAt = useRef(0);
  const ignoreTouchClickUntil = useRef(0);
  const touchFrame = useRef<number | null>(null);
  const updateCamera = useCallback((change: (current: Camera) => Camera) => {
    const next = change(liveCamera.current);
    liveCamera.current = next;
    setCamera(next);
  }, []);
  const queueTouchCamera = useCallback((next: Camera) => {
    liveCamera.current = next;
    if (touchFrame.current !== null) return;
    touchFrame.current = requestAnimationFrame(() => {
      touchFrame.current = null;
      setCamera(liveCamera.current);
    });
  }, []);
  useLayoutEffect(() => () => {
    if (touchFrame.current !== null) cancelAnimationFrame(touchFrame.current);
  }, []);
  const rebaseTouches = useCallback((svg: SVGSVGElement) => {
    const points = [...touches.current.values()];
    pinch.current = null;
    touchStart.current = points.length === 1 ? { point: points[0], camera: liveCamera.current } : null;
    if (points.length >= 2) {
      const [a, b] = points, current = liveCamera.current;
      const rect = svg.getBoundingClientRect();
      const x = (a.x + b.x) / 2 - rect.left - rect.width / 2;
      const y = (a.y + b.y) / 2 - rect.top - rect.height / 2;
      pinch.current = { span: Math.max(1, Math.hypot(a.x - b.x, a.y - b.y)), camera: current, anchor: { x: current.x + x / current.zoom, y: current.y + y / current.zoom } };
    }
  }, []);

  useLayoutEffect(() => {
    const svg = svgRef.current;
    if (!svg) return;
    const measure = () => {
      const rect = svg.getBoundingClientRect();
      const width = Math.max(1, rect.width);
      const height = Math.max(1, rect.height);
      setSize(current => current.width === width && current.height === height ? current : { width, height });
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(svg);
    return () => observer.disconnect();
  }, []);

  const recenter = useCallback(() => {
    updateCamera(current => ({ ...current, x: hero.x, y: hero.y }));
  }, [hero.x, hero.y, updateCamera]);
  const zoomBy = useCallback((factor: number) => {
    updateCamera(current => ({ ...current, zoom: Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, current.zoom * factor)) }));
  }, [updateCamera]);

  const onWheel = useCallback((event: WheelEvent<SVGSVGElement>) => {
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    const offsetX = event.clientX - rect.left - rect.width / 2;
    const offsetY = event.clientY - rect.top - rect.height / 2;
    updateCamera(current => {
      const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, current.zoom * Math.exp(-event.deltaY * 0.0012)));
      return {
        zoom,
        x: current.x + offsetX / current.zoom - offsetX / zoom,
        y: current.y + offsetY / current.zoom - offsetY / zoom,
      };
    });
  }, [updateCamera]);

  const onPointerDown = useCallback((event: PointerEvent<SVGSVGElement>) => {
    if (event.pointerType === "touch") {
      event.preventDefault();
      if (touches.current.size === 0) {
        drag.current = null;
        touchBlockedTap.current = false;
        touchStartedAt.current = performance.now();
      } else touchBlockedTap.current = true;
      touches.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
      ignoreTouchClickUntil.current = Date.now() + 1000;
      event.currentTarget.setPointerCapture(event.pointerId);
      rebaseTouches(event.currentTarget);
      return;
    }
    if (touches.current.size) return;
    ignoreTouchClickUntil.current = 0;
    if (event.button !== 0 || !(event.target instanceof SVGElement)) return;
    if (event.target.closest("[data-hex-actor], [data-hex-object]")) return;
    drag.current = { x: event.clientX, y: event.clientY, camera: liveCamera.current, moved: false };
  }, [rebaseTouches]);

  const onPointerMove = useCallback((event: PointerEvent<SVGSVGElement>) => {
    if (event.pointerType === "touch" || touches.current.has(event.pointerId)) {
      if (!touches.current.has(event.pointerId)) return;
      event.preventDefault();
      touches.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
      const points = [...touches.current.values()];
      const zoomStart = pinch.current;
      if (points.length >= 2 && zoomStart) {
        touchBlockedTap.current = true;
        const [a, b] = points, rect = event.currentTarget.getBoundingClientRect();
        const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoomStart.camera.zoom * Math.hypot(a.x - b.x, a.y - b.y) / zoomStart.span));
        queueTouchCamera({ zoom, x: zoomStart.anchor.x - ((a.x + b.x) / 2 - rect.left - rect.width / 2) / zoom, y: zoomStart.anchor.y - ((a.y + b.y) / 2 - rect.top - rect.height / 2) / zoom });
        return;
      }
      const start = touchStart.current;
      if (!start) return;
      const dx = event.clientX - start.point.x, dy = event.clientY - start.point.y;
      if (!touchBlockedTap.current && Math.hypot(dx, dy) < TOUCH_DRAG_THRESHOLD) return;
      touchBlockedTap.current = true;
      setPanning(true);
      queueTouchCamera({ ...start.camera, x: start.camera.x - dx / start.camera.zoom, y: start.camera.y - dy / start.camera.zoom });
      return;
    }
    if (touches.current.size) return;
    const start = drag.current;
    if (!start) return;
    const dx = event.clientX - start.x;
    const dy = event.clientY - start.y;
    if (!start.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
    if (!start.moved) event.currentTarget.setPointerCapture(event.pointerId);
    start.moved = true;
    setPanning(true);
    updateCamera(() => ({ ...start.camera, x: start.camera.x - dx / start.camera.zoom, y: start.camera.y - dy / start.camera.zoom }));
  }, [queueTouchCamera, updateCamera]);

  const onPointerUp = useCallback((event: PointerEvent<SVGSVGElement>) => {
    if (event.pointerType === "touch" || touches.current.has(event.pointerId)) {
      if (!touches.current.has(event.pointerId)) return;
      const start = touchStart.current;
      const tap = event.type === "pointerup" && touches.current.size === 1 && !touchBlockedTap.current && !!start &&
        Math.hypot(event.clientX - start.point.x, event.clientY - start.point.y) < TOUCH_DRAG_THRESHOLD && performance.now() - touchStartedAt.current <= TAP_DURATION;
      // Delete before release: normal lostpointercapture must not cancel a new gesture.
      touches.current.delete(event.pointerId);
      touchBlockedTap.current = true;
      ignoreTouchClickUntil.current = Date.now() + 750;
      if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
      rebaseTouches(event.currentTarget);
      if (!touches.current.size) setPanning(false);
      if (tap) tapHandler.current?.({ x: event.clientX, y: event.clientY }, event.currentTarget);
      return;
    }
    if (touches.current.size) return;
    const moved = drag.current?.moved;
    drag.current = null;
    setPanning(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
    if (moved) {
      suppressClick.current = true;
      window.setTimeout(() => { suppressClick.current = false; }, 0);
    }
  }, [rebaseTouches]);

  const onClickCapture = useCallback((event: MouseEvent<SVGSVGElement>) => {
    const native = event.nativeEvent as typeof event.nativeEvent & { pointerType?: string; sourceCapabilities?: { firesTouchEvents?: boolean } };
    const touchClick = native.pointerType === "touch" || native.sourceCapabilities?.firesTouchEvents === true;
    if (!suppressClick.current && !((event.detail > 0 || touchClick) && Date.now() < ignoreTouchClickUntil.current)) return;
    suppressClick.current = false;
    event.preventDefault();
    event.stopPropagation();
  }, []);

  return {
    svgRef,
    camera,
    panning,
    viewBox: `${camera.x - size.width / camera.zoom / 2} ${camera.y - size.height / camera.zoom / 2} ${size.width / camera.zoom} ${size.height / camera.zoom}`,
    recenter,
    zoomBy,
    onWheel,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onPointerCancel: onPointerUp,
    onLostPointerCapture: onPointerUp,
    onClickCapture,
  };
}
