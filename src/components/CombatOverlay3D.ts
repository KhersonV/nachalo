import * as THREE from "three";
import type { ActiveEffect } from "@/types/combat";
import { normalizeAvatarPath } from "@/utils/normalizeAvatarPath";

/** Project the shared combat timeline; updated by the scene's existing RAF. */
export class CombatOverlay3D {
  private layer = document.createElement("div");
  private nodes = new Map<string, HTMLElement>();
  private point = new THREE.Vector3();
  private projectileOrigins = new Map<string, THREE.Vector3>();

  constructor(host: HTMLElement) {
    this.layer.setAttribute("aria-label", "Боевые эффекты");
    Object.assign(this.layer.style, { position: "absolute", inset: "0", pointerEvents: "none", overflow: "hidden" });
    host.appendChild(this.layer);
  }

  update(effects: readonly ActiveEffect[], now: number, camera: THREE.Camera,
    width: number, height: number, visible: (p: { x: number; y: number }) => boolean,
    bow?: { cell: { x: number; y: number }; point: THREE.Vector3 }) {
    const active = new Set<string>();
    for (const effect of effects) {
      const t = (now - effect.startMs) / effect.durationMs;
      if (t < 0 || t > 1) continue;
      const cell = effect.kind === "deathFade" ? effect.actor.position : effect.kind === "projectile" ? effect.target : effect.cell;
      if (!visible(cell) || (effect.kind === "projectile" && !visible(effect.source))) continue;
      let x = cell.x, z = cell.y;
      if (effect.kind === "projectile") {
        x = THREE.MathUtils.lerp(effect.source.x, cell.x, t);
        z = THREE.MathUtils.lerp(effect.source.y, cell.y, t);
      }
      this.point.set(x, effect.kind === "floater" || effect.kind === "textFloater" ? 1.5 + t * .4 : .8, z);
      if (effect.kind === "projectile" && bow && effect.source.x === bow.cell.x && effect.source.y === bow.cell.y) {
        if (!this.projectileOrigins.has(effect.id)) this.projectileOrigins.set(effect.id,bow.point.clone());
        this.point.copy(this.projectileOrigins.get(effect.id)!).lerp(new THREE.Vector3(cell.x,.8,cell.y),t);
      }
      this.point.project(camera);
      if (this.point.z < -1 || this.point.z > 1) continue;
      active.add(effect.id);
      let node = this.nodes.get(effect.id);
      if (!node) {
        node = document.createElement(effect.kind === "deathFade" ? "img" : "div");
        node.dataset.effect = effect.kind;
        Object.assign(node.style, { position: "absolute", transform: "translate(-50%,-50%)", font: "bold 22px sans-serif",
          textShadow: "0 2px 4px black", color: "#ffd069" });
        if (effect.kind === "floater") {
          node.textContent = `${effect.isHeal ? "+" : "−"}${effect.value}`;
          node.style.color = effect.isHeal ? "#7dff9e" : "#ff8070";
        } else if (effect.kind === "textFloater") node.textContent = effect.text;
        else if (effect.kind === "deathFade") {
          const src = normalizeAvatarPath(effect.actor.image);
          if (!src.startsWith("/") || src.startsWith("//") || src.includes("..")) continue;
          (node as HTMLImageElement).src = src;
          (node as HTMLImageElement).alt = "";
          Object.assign(node.style, { width: "70px", height: "80px", objectFit: "contain" });
        } else {
          Object.assign(node.style, { width: effect.kind === "projectile" ? "12px" : "55px",
            height: effect.kind === "projectile" ? "5px" : "55px", borderRadius: "50%",
            background: effect.kind === "hitFlash" ? "#ff754d88" : "#ffd069aa", boxShadow: "0 0 12px #ffb34e" });
        }
        this.nodes.set(effect.id, node); this.layer.appendChild(node);
      }
      node.style.left = `${(this.point.x + 1) * width / 2}px`;
      node.style.top = `${(1 - this.point.y) * height / 2}px`;
      node.style.opacity = String(effect.kind === "projectile" ? 1 : 1 - t);
    }
    for (const [id, node] of this.nodes) if (!active.has(id)) { node.remove(); this.nodes.delete(id); }
    for (const id of this.projectileOrigins.keys()) if (!active.has(id)) this.projectileOrigins.delete(id);
  }

  dispose() { this.nodes.clear(); this.projectileOrigins.clear(); this.layer.remove(); }
}
