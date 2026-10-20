"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { PointerEvent, WheelEvent, MouseEvent } from "react";

export type CameraPoint = Readonly<{ x: number; y: number }>;
type Camera = { x: number; y: number; zoom: number };
type Drag = { x: number; y: number; camera: Camera; moved: boolean };

const MIN_ZOOM = 0.65;
const MAX_ZOOM = 2.4;
const DRAG_THRESHOLD = 6;

export function useHexCamera(hero: CameraPoint) {
  const svgRef = useRef<SVGSVGElement>(null);
  const drag = useRef<Drag | null>(null);
  const suppressClick = useRef(false);
  const [size, setSize] = useState({ width: 1000, height: 700 });
  const [camera, setCamera] = useState<Camera>(() => ({ x: hero.x, y: hero.y, zoom: 1 }));
  const [panning, setPanning] = useState(false);

  useEffect(() => {
    const svg = svgRef.current;
    if (!svg) return;
    const observer = new ResizeObserver(() => {
      const rect = svg.getBoundingClientRect();
      setSize({ width: Math.max(1, rect.width), height: Math.max(1, rect.height) });
    });
    observer.observe(svg);
    return () => observer.disconnect();
  }, []);

  const recenter = useCallback(() => {
    setCamera(current => ({ ...current, x: hero.x, y: hero.y }));
  }, [hero.x, hero.y]);
  const zoomBy = useCallback((factor: number) => {
    setCamera(current => ({ ...current, zoom: Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, current.zoom * factor)) }));
  }, []);

  const onWheel = useCallback((event: WheelEvent<SVGSVGElement>) => {
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    const offsetX = event.clientX - rect.left - rect.width / 2;
    const offsetY = event.clientY - rect.top - rect.height / 2;
    setCamera(current => {
      const zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, current.zoom * Math.exp(-event.deltaY * 0.0012)));
      return {
        zoom,
        x: current.x + offsetX / current.zoom - offsetX / zoom,
        y: current.y + offsetY / current.zoom - offsetY / zoom,
      };
    });
  }, []);

  const onPointerDown = useCallback((event: PointerEvent<SVGSVGElement>) => {
    if (event.button !== 0 || !(event.target instanceof SVGElement)) return;
    if (event.target.closest("[data-hex-actor], [data-hex-object]")) return;
    drag.current = { x: event.clientX, y: event.clientY, camera, moved: false };
  }, [camera]);

  const onPointerMove = useCallback((event: PointerEvent<SVGSVGElement>) => {
    const start = drag.current;
    if (!start) return;
    const dx = event.clientX - start.x;
    const dy = event.clientY - start.y;
    if (!start.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
    if (!start.moved) event.currentTarget.setPointerCapture(event.pointerId);
    start.moved = true;
    setPanning(true);
    setCamera({ ...start.camera, x: start.camera.x - dx / start.camera.zoom, y: start.camera.y - dy / start.camera.zoom });
  }, []);

  const onPointerUp = useCallback((event: PointerEvent<SVGSVGElement>) => {
    const moved = drag.current?.moved;
    drag.current = null;
    setPanning(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
    if (moved) {
      suppressClick.current = true;
      window.setTimeout(() => { suppressClick.current = false; }, 0);
    }
  }, []);

  const onClickCapture = useCallback((event: MouseEvent<SVGSVGElement>) => {
    if (!suppressClick.current) return;
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
    onClickCapture,
  };
}
