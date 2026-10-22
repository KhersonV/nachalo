import * as THREE from "three";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { clone } from "three/examples/jsm/utils/SkeletonUtils.js";
import combatMetadata from "../data/rangerCombat.json";

export type RangerCue = { id: string; clip: "Attack_Bow" | "Hit" | "Death"; target?: { x: number; y: number };
  startedAt?: number; releaseAt?: number };
export type RangerModel = "legacy" | "animated";
export type RangerSpec = { position: { x: number; y: number }; userId?: number; cues?: RangerCue[];
  model?: RangerModel; resetKey?: number; defeated?: boolean };
export type RangerStatus = { model: RangerModel; ready: boolean; failed: boolean; clip: string;
  speed: number; walkTimeScale: number; x: number; z: number; heading: number; bones: number;
  clipTime: number; arrowVisible: boolean };

const MODELS = {
  legacy: { url: "/assets/nachalo/characters/ranger/ranger.glb", scale: 0.82 },
  animated: { url: "/assets/nachalo/characters/ranger/ranger_walk_tuned.glb", scale: 0.82 },
};
const AUTHORED_WALK_SPEED = 1.25;
const MAX_WALK_TIME_SCALE = 2;
const loader = new GLTFLoader();
type ModelAsset = { users: number; promise: ReturnType<GLTFLoader["loadAsync"]> };
const assets = new Map<RangerModel, ModelAsset>();

function disposeAsset(scene: THREE.Object3D) {
  const geometries = new Set<THREE.BufferGeometry>();
  const materials = new Set<THREE.Material>();
  const textures = new Set<THREE.Texture>();
  const images = new Set<ImageBitmap>();
  scene.traverse(object => {
    if (!(object instanceof THREE.Mesh)) return;
    geometries.add(object.geometry);
    for (const material of Array.isArray(object.material) ? object.material : [object.material]) {
      materials.add(material);
      for (const value of Object.values(material)) if (value instanceof THREE.Texture) textures.add(value);
    }
  });
  for (const geometry of geometries) geometry.dispose();
  for (const material of materials) material.dispose();
  for (const texture of textures) {
    if (typeof ImageBitmap !== "undefined" && texture.image instanceof ImageBitmap) images.add(texture.image);
    texture.dispose();
  }
  for (const image of images) image.close();
}

// SkeletonUtils shares mesh resources. Keep a template alive only while one or
// more instances use it; late loads are released too when their owner unmounts.
function acquireModel(model: RangerModel) {
  let asset = assets.get(model);
  if (!asset) {
    asset = { users: 0, promise: loader.loadAsync(MODELS[model].url) };
    assets.set(model, asset);
  }
  asset.users++;
  const held = asset;
  return { promise: held.promise, release: () => {
    if (--held.users !== 0) return;
    if (assets.get(model) === held) assets.delete(model);
    void held.promise.then(gltf => disposeAsset(gltf.scene), () => {});
  } };
}

/** One persistent visual instance per scene. Authoritative coordinates are set by the caller. */
export class AnimatedRanger {
  readonly root = new THREE.Group();
  readonly model: RangerModel;
  readonly resetKey: number;
  private mixer: THREE.AnimationMixer | null = null;
  private instance: THREE.Object3D | null = null;
  private releaseAsset: () => void;
  private clips = new Map<string, THREE.AnimationAction>();
  private active: THREE.AnimationAction | null = null;
  private activeName = "";
  private destination: THREE.Vector3;
  private seen = new Set<string>();
  private pending = new Map<string, RangerCue>();
  private eventEndsAt = 0;
  private eventStartedAt: number | undefined;
  private dead = false;
  private serverDefeated = false;
  private disposed = false;
  private loaded = false;
  private failed = false;
  private speed = 0;
  private boneCount = 0;
  private movement = new THREE.Vector3();
  onReady?: () => void;

  constructor(spec: RangerSpec) {
    this.model = spec.model ?? "legacy";
    this.resetKey = spec.resetKey ?? 0;
    this.root.name = `Ranger:${this.model}`;
    this.root.position.set(spec.position.x, 0, spec.position.y);
    this.destination = this.root.position.clone();
    this.serverDefeated = spec.defeated === true;
    const asset = acquireModel(this.model);
    this.releaseAsset = asset.release;
    void asset.promise.then(gltf => {
      if (this.disposed) return;
      const model = clone(gltf.scene);
      this.instance = model;
      model.scale.setScalar(MODELS[this.model].scale);
      if (this.model === "animated") model.position.y = -0.01 - 0.002 * MODELS.animated.scale;
      model.traverse(object => {
        if (object instanceof THREE.Bone) this.boneCount++;
        if (object instanceof THREE.Mesh) {
          object.castShadow = true;
          object.receiveShadow = true;
        }
      });
      this.root.add(model);
      this.mixer = new THREE.AnimationMixer(model);
      for (const sourceClip of gltf.animations) {
        const clip = this.model === "animated" ? sourceClip.clone() : sourceClip;
        if (this.model === "animated") {
          // Blender starts at frame 1. Three.js otherwise holds that first pose
          // from t=0 to its first key on every repeat, causing a foot hitch.
          const start = Math.min(...clip.tracks.map(track => track.times[0] ?? 0));
          if (start > 0) for (const track of clip.tracks) track.shift(-start);
          clip.resetDuration();
        }
        const action = this.mixer.clipAction(clip);
        if (clip.name === "Idle" || clip.name === "Walk") action.setLoop(THREE.LoopRepeat, Infinity);
        else { action.setLoop(THREE.LoopOnce, 1); action.clampWhenFinished = true; }
        this.clips.set(clip.name, action);
      }
      this.loaded = true;
      this.play("Idle");
      this.mixer.update(0);
      const arrow = model.getObjectByName("NAC_CombatArrow");
      if (arrow) arrow.visible = false;
      this.applyDefeat();
      if (this.pending.size) {
        this.setSpec({ position: { x: this.destination.x, y: this.destination.z },
          cues: [...this.pending.values()] }, performance.now() * 0.001);
        this.pending.clear();
      }
      this.onReady?.();
    }).catch(error => {
      if (!this.disposed) {
        this.failed = true;
        console.warn("Ranger GLB unavailable; using the existing fallback", error);
      }
    });
  }

  get isReady() { return this.loaded; }

  get status(): RangerStatus {
    return { model: this.model, ready: this.loaded, failed: this.failed, clip: this.activeName,
      speed: this.speed, walkTimeScale: this.clips.get("Walk")?.getEffectiveTimeScale() ?? 0,
      x: this.root.position.x, z: this.root.position.z, heading: this.root.rotation.y, bones: this.boneCount,
      clipTime: this.active?.time ?? 0, arrowVisible: this.instance?.getObjectByName("NAC_CombatArrow")?.visible ?? false };
  }

  get projectileOrigin() {
    if (this.model !== "animated" || !this.loaded) return undefined;
    return this.instance?.getObjectByName("hand_l")?.getWorldPosition(new THREE.Vector3()).add(new THREE.Vector3(0,-.08,0));
  }

  /** Place the visual at an authoritative position after a discontinuous snapshot. */
  snapTo(position: { x: number; y: number }) {
    this.destination.set(position.x, 0, position.y);
    this.root.position.copy(this.destination);
    this.speed = 0;
    if (!this.dead && this.loaded) this.play("Idle");
  }

  setSpec(spec: RangerSpec, now: number) {
    this.serverDefeated ||= spec.defeated === true;
    this.applyDefeat();
    const { x, y } = spec.position;
    if (this.destination.x !== x || this.destination.z !== y) {
      this.destination.set(x, 0, y);
      if (!this.dead && now >= this.eventEndsAt) this.face(this.destination);
    }
    for (const cue of spec.cues ?? []) {
      if (!this.loaded) { this.pending.set(cue.id, cue); continue; }
      if (cue.startedAt !== undefined && cue.startedAt > now) continue;
      if (this.seen.has(cue.id)) continue;
      this.seen.add(cue.id);
      if (this.seen.size > 128) this.seen.delete(this.seen.values().next().value!);
      if (this.dead) continue;
      // Missing clips never block controls or change authoritative life state.
      if (!this.clips.has(cue.clip)) continue;
      if (cue.target) this.face(new THREE.Vector3(cue.target.x, 0, cue.target.y));
      this.play(cue.clip, true);
      const action = this.clips.get(cue.clip)!;
      if (this.model === "animated" && cue.clip === "Attack_Bow" && cue.releaseAt !== undefined && cue.startedAt !== undefined) {
        action.setEffectiveTimeScale(combatMetadata.attackReleaseSeconds / Math.max(.05,cue.releaseAt-cue.startedAt));
      }
      const started = cue.startedAt ?? now;
      this.eventStartedAt = started;
      const rate = action.getEffectiveTimeScale();
      action.time = Math.min(action.getClip().duration, Math.max(0,now-started)*rate);
      this.eventEndsAt = started + action.getClip().duration / rate;
      if (cue.clip === "Death") this.dead = true;
    }
  }

  private applyDefeat() {
    if (!this.serverDefeated) return;
    this.dead = true;
    if (!this.loaded) return;
    if (this.clips.has("Death")) this.play("Death");
    else {
      // The existing combat death-fade effect supplies the visual fallback.
      // Never present a live Idle/Walk after an authoritative defeat.
      this.root.visible = false;
      this.activeName = "Defeated";
      this.mixer?.stopAllAction();
    }
  }

  private face(target: THREE.Vector3) {
    const dx = target.x - this.root.position.x, dz = target.z - this.root.position.z;
    if (dx * dx + dz * dz > 0.0001) this.root.rotation.y = Math.atan2(dx, dz);
  }

  private play(name: string, restart = false) {
    const next = this.clips.get(name);
    if (!next || (next === this.active && !restart)) return;
    if (this.model === "animated") {
      for (const action of this.clips.values()) if (action !== next && action !== this.active) action.stop();
      this.active?.stopFading().setEffectiveWeight(1);
    }
    // Consecutive server tiles may have a brief Idle between them. Continue
    // the slower walk phase so every tile does not restart on the same foot.
    const walkTime = name === "Walk" && this.model === "animated" ? next.time : 0;
    next.reset().setEffectiveTimeScale(name === "Walk" && this.model === "legacy" ? 2.5 : 1).setEffectiveWeight(1).play();
    if (name === "Walk" && this.model === "animated") next.time = walkTime;
    if (this.active && this.active !== next) next.crossFadeFrom(this.active, name === "Death" && this.model === "animated" ? .08 : .16, false);
    this.active = next;
    this.activeName = name;
  }

  update(delta: number, now: number) {
    this.speed = 0;
    if (!this.dead) {
      const distance = this.root.position.distanceTo(this.destination);
      if (distance > (this.model === "animated" ? 1e-6 : 0.012)) {
        if (now >= this.eventEndsAt) this.face(this.destination);
        // One tile takes ~0.36 s. Position changes only after a confirmed snapshot.
        const step = Math.min(distance, delta * 2.75);
        this.root.position.addScaledVector(this.movement.copy(this.destination).sub(this.root.position).normalize(), step);
        this.speed = delta > 0 ? step / delta : 0;
        if (now >= this.eventEndsAt) this.play("Walk");
      } else {
        this.root.position.copy(this.destination);
        if (now >= this.eventEndsAt && this.activeName !== "Idle") this.play("Idle");
      }
    }
    if (this.model === "animated") {
      const walk = this.clips.get("Walk");
      // Longer authored Walk stride, capped at three footfalls per second.
      // At full game speed some sliding remains; avoid forcing a walking rig
      // to 11 footfalls/sec or changing the authoritative visual translation.
      walk?.setEffectiveTimeScale(Math.min(MAX_WALK_TIME_SCALE,
        this.speed / (AUTHORED_WALK_SPEED * MODELS.animated.scale)));
    }
    this.mixer?.update(delta);
    if (this.model === "animated" && this.activeName === "Attack_Bow" && this.active && this.eventStartedAt !== undefined) {
      // Use the presentation clock even when a frame stalls; the existing
      // projectile and the skinned arrow must cross release together.
      this.active.time = Math.min(this.active.getClip().duration,
        Math.max(0,now-this.eventStartedAt)*this.active.getEffectiveTimeScale());
      this.mixer?.update(0);
    }
    const arrow = this.instance?.getObjectByName("NAC_CombatArrow");
    if (arrow) arrow.visible = this.activeName === "Attack_Bow" &&
      (this.active?.time ?? 0) >= combatMetadata.attackArrowVisibleSeconds &&
      (this.active?.time ?? 0) < combatMetadata.attackReleaseSeconds;
  }

  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    this.mixer?.stopAllAction();
    if (this.instance) {
      this.mixer?.uncacheRoot(this.instance);
      const skeletons = new Set<THREE.Skeleton>();
      this.instance.traverse(object => {
        if (object instanceof THREE.SkinnedMesh) skeletons.add(object.skeleton);
      });
      for (const skeleton of skeletons) skeleton.dispose();
    }
    this.mixer = null;
    this.clips.clear();
    this.active = null;
    this.instance = null;
    this.loaded = false;
    this.onReady = undefined;
    this.pending.clear();
    this.root.clear();
    this.root.removeFromParent();
    this.releaseAsset();
  }
}
