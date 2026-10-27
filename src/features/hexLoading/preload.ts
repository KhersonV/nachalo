import { COMMON_HEX_IMAGES, preloadHexImages } from "./resources";
import { timeHexStage } from "./performance";

let screen: Promise<typeof import("@/components/HexGameController")> | undefined;
export function loadHexScreen() {
  if (!screen) {
    const done = timeHexStage("battle JavaScript (import to evaluation)");
    screen = import("@/components/HexGameController").then(module => { done(); return module; }).catch(error => { screen = undefined; throw error; });
  }
  return screen;
}
// Deliberately no Three.js import or GLB download on the lobby/2D path.
export function scheduleHexPreload() {
  const controller = new AbortController();
  const timer = window.setTimeout(() => {
    if (document.visibilityState !== "visible") return;
    const network = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection;
    if (network?.saveData || /(^|-)2g$/.test(network?.effectiveType ?? "")) return;
    void loadHexScreen().catch(() => {});
    void preloadHexImages(COMMON_HEX_IMAGES, controller.signal);
  }, 600);
  // Imports and an already shared Image request cannot safely be aborted;
  // queued optional images are skipped when their owner leaves the screen.
  return () => { window.clearTimeout(timer); controller.abort(); };
}
