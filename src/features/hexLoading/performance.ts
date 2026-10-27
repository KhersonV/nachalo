// Local, development-only timings. No IDs, tokens or server payloads.
export function timeHexStage(stage: string) {
  const start = typeof performance === "undefined" ? 0 : performance.now();
  let ended = false;
  return () => {
    if (ended) return;
    ended = true;
    if (process.env.NODE_ENV === "development") console.debug(`[HEX] ${stage}: ${Math.round(performance.now() - start)} ms`);
  };
}
export function hexCacheHit(kind: "image" | "model") {
  if (process.env.NODE_ENV === "development") counters[kind]++;
}
const counters = { image: 0, model: 0 };
export function reportHexCache() {
  if (process.env.NODE_ENV === "development") console.debug("[HEX] in-memory reuse", { ...counters });
}
