import * as THREE from "three";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { clone } from "three/examples/jsm/utils/SkeletonUtils.js";

export type RangerCue = { id: string; clip: "Attack_Bow" | "Hit" | "Death"; target?: { x: number; y: number } };
export type RangerSpec = { position: { x: number; y: number }; userId?: number; cues?: RangerCue[] };

const MODEL_URL = "/assets/nachalo/characters/ranger/ranger.glb";
const loader = new GLTFLoader();
let cached: Promise<Awaited<ReturnType<GLTFLoader["loadAsync"]>>> | null = null;
function loadModel() {
  cached ??= loader.loadAsync(MODEL_URL).catch(error => { cached = null; throw error; });
  return cached;
}

/** One persistent visual instance per scene. Authoritative coordinates are set by the caller. */
export class AnimatedRanger {
  readonly root = new THREE.Group();
  private mixer: THREE.AnimationMixer | null = null;
  private clips = new Map<string, THREE.AnimationAction>();
  private active: THREE.AnimationAction | null = null;
  private activeName = "";
  private destination: THREE.Vector3;
  private seen = new Set<string>();
  private pending = new Map<string, RangerCue>();
  private eventEndsAt = 0;
  private dead = false;
  private disposed = false;
  private loaded = false;
  onReady?: () => void;

  constructor(spec: RangerSpec) {
    this.root.position.set(spec.position.x, 0, spec.position.y);
    this.destination = this.root.position.clone();
    void loadModel().then(gltf => {
      if (this.disposed) return;
      const model = clone(gltf.scene);
      model.scale.setScalar(0.82);
      model.traverse(object => {
        if (object instanceof THREE.Mesh) {
          object.castShadow = true;
          object.receiveShadow = true;
        }
      });
      this.root.add(model);
      this.mixer = new THREE.AnimationMixer(model);
      for (const clip of gltf.animations) {
        const action = this.mixer.clipAction(clip);
        if (clip.name === "Idle" || clip.name === "Walk") action.setLoop(THREE.LoopRepeat, Infinity);
        else { action.setLoop(THREE.LoopOnce, 1); action.clampWhenFinished = true; }
        this.clips.set(clip.name, action);
      }
      this.loaded = true;
      this.play("Idle");
      if (this.pending.size) {
        this.setSpec({ position: { x: this.destination.x, y: this.destination.z },
          cues: [...this.pending.values()] }, performance.now() * 0.001);
        this.pending.clear();
      }
      this.onReady?.();
    }).catch(error => {
      if (!this.disposed) console.warn("Ranger GLB unavailable; using the existing sprite", error);
    });
  }

  get isReady() { return this.loaded; }

  setSpec(spec: RangerSpec, now: number) {
    const { x, y } = spec.position;
    if (this.destination.x !== x || this.destination.z !== y) {
      this.destination.set(x, 0, y);
      if (!this.dead && now >= this.eventEndsAt) this.face(this.destination);
    }
    for (const cue of spec.cues ?? []) {
      if (!this.loaded) { this.pending.set(cue.id, cue); continue; }
      if (this.seen.has(cue.id)) continue;
      this.seen.add(cue.id);
      if (this.seen.size > 128) this.seen.delete(this.seen.values().next().value!);
      if (this.dead) continue;
      if (cue.target) this.face(new THREE.Vector3(cue.target.x, 0, cue.target.y));
      this.play(cue.clip);
      this.eventEndsAt = now + (this.clips.get(cue.clip)?.getClip().duration ?? 0.7);
      if (cue.clip === "Death") this.dead = true;
    }
  }

  private face(target: THREE.Vector3) {
    const dx = target.x - this.root.position.x, dz = target.z - this.root.position.z;
    if (dx * dx + dz * dz > 0.0001) this.root.rotation.y = Math.atan2(dx, dz);
  }

  private play(name: string) {
    const next = this.clips.get(name);
    if (!next || next === this.active) return;
    next.reset().setEffectiveTimeScale(name === "Walk" ? 2.5 : 1).setEffectiveWeight(1).play();
    if (this.active) next.crossFadeFrom(this.active, 0.16, false);
    this.active = next;
    this.activeName = name;
  }

  update(delta: number, now: number) {
    if (!this.dead) {
      const distance = this.root.position.distanceTo(this.destination);
      if (distance > 0.012) {
        if (now >= this.eventEndsAt) this.face(this.destination);
        // One tile takes ~0.36 s. Position changes only after a confirmed snapshot.
        const step = Math.min(distance, delta * 2.75);
        this.root.position.addScaledVector(this.destination.clone().sub(this.root.position).normalize(), step);
        if (now >= this.eventEndsAt) this.play("Walk");
      } else {
        this.root.position.copy(this.destination);
        if (now >= this.eventEndsAt && this.activeName !== "Idle") this.play("Idle");
      }
    }
    this.mixer?.update(delta);
  }

  dispose() {
    this.disposed = true;
    this.mixer?.stopAllAction();
    this.root.removeFromParent();
  }
}
