import * as THREE from "three";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { clone } from "three/examples/jsm/utils/SkeletonUtils.js";
import type { HexActor } from "@/types/HexGameTypes";

type ClipName = "Idle" | "Walk" | "Attack" | "Hit" | "Death";
type ModelConfig = { url: string; height: number; maxWidth: number; facingOffset: number };
const MODELS: Record<string, ModelConfig> = {
  orc: { url: "/assets/nachalo/hex3d/monsters/orc_brute.glb", height: 1.7, maxWidth: 1.08, facingOffset: 0 },
};
const REQUIRED_CLIPS: ClipName[] = ["Idle", "Walk", "Attack", "Hit", "Death"];
const loader = new GLTFLoader();
type Loaded = Awaited<ReturnType<GLTFLoader["loadAsync"]>>;
const assets = new Map<string, { promise: Promise<Loaded>; users: number }>();
const unavailable = new Set<string>();

export function monsterModelConfig(actor: HexActor): ModelConfig | null {
  if (actor.kind !== "monster") return null;
  // monsterRefId is a database SERIAL, while `type` is the generic "monster".
  return MODELS[actor.name?.trim().toLowerCase() ?? ""] ?? null;
}

function disposeTemplate(scene: THREE.Object3D) {
  const geometries = new Set<THREE.BufferGeometry>();
  const materials = new Set<THREE.Material>();
  const textures = new Set<THREE.Texture>();
  scene.traverse(object => {
    if (!(object instanceof THREE.Mesh)) return;
    geometries.add(object.geometry);
    for (const material of Array.isArray(object.material) ? object.material : [object.material]) {
      materials.add(material);
      for (const value of Object.values(material)) if (value instanceof THREE.Texture) textures.add(value);
    }
  });
  geometries.forEach(geometry => geometry.dispose());
  materials.forEach(material => material.dispose());
  textures.forEach(texture => texture.dispose());
}

function acquire(config: ModelConfig) {
  let asset = assets.get(config.url);
  if (!asset) {
    asset = { promise: loader.loadAsync(config.url), users: 0 };
    assets.set(config.url, asset);
  }
  asset.users++;
  const held = asset;
  return { promise: held.promise, release: () => {
    if (--held.users !== 0) return;
    if (assets.get(config.url) === held) assets.delete(config.url);
    void held.promise.then(gltf => disposeTemplate(gltf.scene), () => unavailable.add(config.url));
  } };
}

export class AnimatedMonster {
  readonly root = new THREE.Group();
  readonly config: ModelConfig;
  private releaseAsset: () => void = () => {};
  private instance: THREE.Object3D | null = null;
  private mixer: THREE.AnimationMixer | null = null;
  private clips = new Map<ClipName, THREE.AnimationAction>();
  private active: ClipName | null = null;
  private route: THREE.Vector3[] = [];
  private cues = new Set<string>();
  private lockedUntil = 0;
  private dead = false;
  private disposed = false;
  private ready = false;
  onReady?: () => void;

  constructor(config: ModelConfig, x: number, z: number) {
    this.config = config;
    this.root.position.set(x, 0, z);
    if (unavailable.has(config.url)) { this.onReady?.(); return; }
    const asset = acquire(config);
    this.releaseAsset = asset.release;
    void asset.promise.then(gltf => {
      if (this.disposed) return;
      const clipNames = new Set(gltf.animations.map(clip => clip.name));
      let bones = 0;
      gltf.scene.traverse(object => { if (object instanceof THREE.Bone) bones++; });
      if (!bones || REQUIRED_CLIPS.some(name => !clipNames.has(name))) {
        unavailable.add(config.url);
        this.onReady?.();
        return;
      }
      const model = clone(gltf.scene);
      const box = new THREE.Box3().setFromObject(model);
      const size = box.getSize(new THREE.Vector3());
      const center = box.getCenter(new THREE.Vector3());
      if (size.y <= 0 || Math.max(size.x, size.z) <= 0) { unavailable.add(config.url); this.onReady?.(); return; }
      const scale = Math.min(config.height / size.y, config.maxWidth / Math.max(size.x, size.z));
      model.scale.setScalar(scale);
      model.position.set(-center.x * scale, -box.min.y * scale, -center.z * scale);
      model.traverse(object => { if (object instanceof THREE.Mesh) { object.castShadow = true; object.receiveShadow = true; } });
      this.root.add(model);
      this.instance = model;
      this.mixer = new THREE.AnimationMixer(model);
      for (const name of REQUIRED_CLIPS) {
        const action = this.mixer.clipAction(gltf.animations.find(clip => clip.name === name)!);
        if (name === "Idle" || name === "Walk") action.setLoop(THREE.LoopRepeat, Infinity);
        else { action.setLoop(THREE.LoopOnce, 1); action.clampWhenFinished = true; }
        this.clips.set(name, action);
      }
      this.mixer.addEventListener("finished", () => {
        if (!this.dead) { this.lockedUntil = 0; this.play(this.route.length ? "Walk" : "Idle"); }
      });
      this.ready = true;
      this.play("Idle");
      this.onReady?.();
    }).catch(() => { unavailable.add(config.url); if (!this.disposed) this.onReady?.(); });
  }

  get isReady() { return this.ready; }

  snapTo(x: number, z: number) {
    this.route.length = 0;
    this.root.position.set(x, 0, z);
    if (this.ready && !this.dead) this.play("Idle");
  }

  /** Call only with a server-confirmed route. */
  follow(path: readonly { x: number; z: number }[]) {
    if (this.dead) return;
    this.route = path.map(point => new THREE.Vector3(point.x, 0, point.z));
  }

  cue(id: string, clip: "Attack" | "Hit" | "Death", now: number, target?: { x: number; z: number }) {
    if (!this.ready || this.cues.has(id) || this.dead) return;
    this.cues.add(id);
    if (this.cues.size > 96) this.cues.delete(this.cues.values().next().value!);
    if (target) this.face(new THREE.Vector3(target.x, 0, target.z));
    this.play(clip, true);
    this.lockedUntil = now + (this.clips.get(clip)?.getClip().duration ?? 0);
    if (clip === "Death") { this.dead = true; this.route.length = 0; }
  }

  update(delta: number, now: number) {
    if (!this.dead && this.route.length) {
      const target = this.route[0];
      const distance = this.root.position.distanceTo(target);
      if (distance > .001) {
        if (now >= this.lockedUntil) { this.face(target); this.play("Walk"); }
        this.root.position.addScaledVector(target.clone().sub(this.root.position).normalize(), Math.min(distance, delta * 1.9));
      }
      if (this.root.position.distanceTo(target) <= .001) {
        this.root.position.copy(target);
        this.route.shift();
        if (!this.route.length && now >= this.lockedUntil) this.play("Idle");
      }
    }
    this.mixer?.update(delta);
  }

  private face(target: THREE.Vector3) {
    const dx = target.x - this.root.position.x, dz = target.z - this.root.position.z;
    if (dx * dx + dz * dz > .0001) this.root.rotation.y = Math.atan2(dx, dz) + this.config.facingOffset;
  }

  private play(name: ClipName, restart = false) {
    const next = this.clips.get(name);
    if (!next || this.active === name && !restart) return;
    const current = this.active && this.clips.get(this.active);
    next.reset().play();
    if (current && current !== next) next.crossFadeFrom(current, .14, false);
    this.active = name;
  }

  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    this.mixer?.stopAllAction();
    if (this.instance) {
      this.mixer?.uncacheRoot(this.instance);
      this.instance.traverse(object => { if (object instanceof THREE.SkinnedMesh) object.skeleton.dispose(); });
    }
    this.root.removeFromParent();
    this.root.clear();
    this.releaseAsset();
  }
}
