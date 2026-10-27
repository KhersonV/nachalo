import { hexCacheHit } from "./performance";

export const COMMON_HEX_IMAGES = [
  "/assets/nachalo/catacombs/floor_stone.webp",
  "/assets/nachalo/catacombs/wall_base.webp",
  "/assets/nachalo/objects/barrel.png",
];
// One image cache/queue for speculative loading and scene textures. GPU textures
// remain scene-owned and can be disposed independently. HTTP caching stays native.
type Entry = { promise: Promise<HTMLImageElement>; bytes: number; loaded: boolean; signal?: AbortSignal };
const images = new Map<string, Entry>();
const queue: Array<() => void> = [];
let active = 0;
const MAX_BYTES = 16 * 1024 * 1024;
const MAX_IMAGES = 24;
function trim() {
  let bytes = [...images.values()].reduce((sum, entry) => sum + entry.bytes, 0);
  for (const [url, entry] of images) {
    if (bytes <= MAX_BYTES && images.size <= MAX_IMAGES) break;
    if (!entry.loaded) continue;
    images.delete(url);
    bytes -= entry.bytes;
  }
}
function drain() {
  while (active < 3 && queue.length) queue.shift()!();
}
export function loadHexImage(url: string, signal?: AbortSignal): Promise<HTMLImageElement> {
  if (signal?.aborted) return Promise.reject(new DOMException("Cancelled", "AbortError"));
  const found = images.get(url);
  if (found) {
    images.delete(url); images.set(url, found);
    if (!signal) found.signal = undefined; // A visible scene adopts a queued optional request.
    hexCacheHit("image");
    return found.promise;
  }
  let resolve!: (image: HTMLImageElement) => void;
  let reject!: (reason: unknown) => void;
  const entry: Entry = { bytes: 0, loaded: false, signal, promise: new Promise((yes, no) => { resolve = yes; reject = no; }) };
  images.set(url, entry);
  queue.push(() => {
    if (entry.signal?.aborted) {
      images.delete(url); reject(new DOMException("Cancelled", "AbortError")); return;
    }
    active++;
    const image = new Image();
    image.crossOrigin = "anonymous";
    const timeout = window.setTimeout(() => {
      image.onload = image.onerror = null;
      image.src = "";
      active--; images.delete(url); reject(new Error("Map image loading timed out.")); drain();
    }, 20_000);
    image.onload = () => {
      window.clearTimeout(timeout);
      active--; entry.loaded = true;
      entry.bytes = image.naturalWidth * image.naturalHeight * 4;
      resolve(image); trim(); drain();
    };
    image.onerror = () => {
      window.clearTimeout(timeout);
      active--; images.delete(url); reject(new Error("Map image could not be loaded.")); drain();
    };
    image.src = url;
  });
  drain();
  return entry.promise;
}
export async function preloadHexImages(urls: string[], signal?: AbortSignal) {
  // Optional failures never prevent entry to battle; renderers have fallbacks.
  await Promise.allSettled([...new Set(urls)].map(url => loadHexImage(url, signal)));
}
