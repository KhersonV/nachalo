import * as THREE from "three";

export type FireProp = {
  group: THREE.Group;
  flame: THREE.Group;
  sparks: THREE.Mesh[];
  light: THREE.PointLight;
  phase: number;
  cellIndex: number;
};

type BuildContext = {
  stone: THREE.Material;
  darkStone: THREE.Material;
  geometries: THREE.BufferGeometry[];
  materials: THREE.Material[];
  sharedGeometries: Map<string, THREE.BufferGeometry>;
};

function mesh(parent: THREE.Group, geometry: THREE.BufferGeometry, material: THREE.Material,
  x: number, y: number, z: number) {
  const part = new THREE.Mesh(geometry, material);
  part.position.set(x, y, z);
  // Small decorative parts are numerous; their shadow passes cost more than they add.
  part.castShadow = false;
  part.receiveShadow = true;
  parent.add(part);
  return part;
}

export function makeAtmosphereContext(stone: THREE.Material, darkStone: THREE.Material): BuildContext {
  return { stone, darkStone, geometries: [], materials: [], sharedGeometries: new Map() };
}

function geometry<T extends THREE.BufferGeometry>(context: BuildContext, value: T): T {
  context.geometries.push(value);
  return value;
}

function material<T extends THREE.Material>(context: BuildContext, value: T): T {
  context.materials.push(value);
  return value;
}

function sharedGeometry<T extends THREE.BufferGeometry>(context: BuildContext, key: string, create: () => T): T {
  const existing = context.sharedGeometries.get(key) as T | undefined;
  if (existing) return existing;
  const value = geometry(context, create());
  context.sharedGeometries.set(key, value);
  return value;
}

export function disposeAtmosphere(context: BuildContext) {
  context.geometries.forEach(item => item.dispose());
  context.materials.forEach(item => item.dispose());
}

// The front of each wall prop is local +Z. The mounting group is rotated to the open face.
export function makeWallTorch(context: BuildContext, x: number, z: number, rotation: number,
  cellIndex: number, phase: number): FireProp {
  const group = new THREE.Group();
  group.position.set(x, 0, z);
  group.rotation.y = rotation;
  const iron = material(context, new THREE.MeshStandardMaterial({ color: 0x332d2a, metalness: 0.65, roughness: 0.66 }));
  const ironEdge = material(context, new THREE.MeshStandardMaterial({ color: 0x76604a, metalness: 0.55, roughness: 0.68 }));
  const ember = material(context, new THREE.MeshStandardMaterial({ color: 0x7a2810, emissive: 0xf35c16, emissiveIntensity: 1.15 }));
  mesh(group, geometry(context, new THREE.BoxGeometry(0.3, 0.57, 0.075)), context.stone, 0, 1.05, 0.51);
  for (const y of [0.86, 1.23]) {
    const pin = mesh(group, geometry(context, new THREE.IcosahedronGeometry(0.045, 0)), ironEdge, 0, y, 0.564);
    pin.scale.z = 0.5;
  }
  mesh(group, geometry(context, new THREE.BoxGeometry(0.075, 0.075, 0.36)), iron, 0, 1.02, 0.73);
  const cup = mesh(group, geometry(context, new THREE.CylinderGeometry(0.15, 0.105, 0.19, 8, 1, true)), iron, 0, 1.04, 0.88);
  cup.castShadow = false;
  for (const y of [0.96, 1.13]) {
    const band = mesh(group, geometry(context, new THREE.TorusGeometry(y === 0.96 ? 0.115 : 0.153, 0.025, 5, 8)), ironEdge, 0, y, 0.88);
    band.rotation.x = Math.PI / 2;
  }
  for (let i = 0; i < 8; i++) {
    const angle = i * Math.PI / 4;
    const tine = mesh(group, geometry(context, new THREE.CylinderGeometry(0.018, 0.025, 0.27, 5)), iron, Math.sin(angle) * 0.14, 1.19, 0.88 + Math.cos(angle) * 0.14);
    tine.rotation.z = Math.sin(angle) * 0.15;
    tine.rotation.x = -Math.cos(angle) * 0.15;
  }
  mesh(group, geometry(context, new THREE.IcosahedronGeometry(0.095, 1)), ember, 0, 1.16, 0.88);
  return addFire(context, group, 0, 1.32, 0.88, cellIndex, phase, 2.6);
}

export function makeFloorBrazier(context: BuildContext, x: number, z: number,
  cellIndex: number, phase: number): FireProp {
  const group = new THREE.Group();
  group.position.set(x, 0, z);
  const iron = material(context, new THREE.MeshStandardMaterial({ color: 0x302d2b, metalness: 0.55, roughness: 0.75 }));
  const rim = material(context, new THREE.MeshStandardMaterial({ color: 0x77624b, metalness: 0.5, roughness: 0.7 }));
  const coals = material(context, new THREE.MeshStandardMaterial({ color: 0x682413, emissive: 0xdc4815, emissiveIntensity: 0.9 }));
  mesh(group, geometry(context, new THREE.BoxGeometry(0.63, 0.12, 0.63)), context.stone, 0, 0.06, 0);
  mesh(group, geometry(context, new THREE.BoxGeometry(0.5, 0.1, 0.5)), context.darkStone, 0, 0.16, 0);
  mesh(group, geometry(context, new THREE.CylinderGeometry(0.29, 0.18, 0.25, 10)), iron, 0, 0.34, 0);
  const lip = mesh(group, geometry(context, new THREE.TorusGeometry(0.29, 0.035, 6, 10)), rim, 0, 0.48, 0);
  lip.rotation.x = Math.PI / 2;
  for (let i = 0; i < 5; i++) {
    const angle = i * Math.PI * 2 / 5;
    mesh(group, geometry(context, new THREE.IcosahedronGeometry(0.09, 0)), coals,
      Math.cos(angle) * 0.12, 0.48, Math.sin(angle) * 0.12);
  }
  for (let i = 0; i < 8; i++) {
    const angle = i * Math.PI / 4;
    mesh(group, geometry(context, new THREE.ConeGeometry(0.035, 0.16, 4)), iron,
      Math.sin(angle) * 0.27, 0.57, Math.cos(angle) * 0.27);
  }
  return addFire(context, group, 0, 0.55, 0, cellIndex, phase, 3);
}

export function makeColumn(context: BuildContext, x: number, z: number, variant: number) {
  const group = new THREE.Group();
  group.position.set(x, 0, z);
  group.rotation.y = (variant % 4) * Math.PI / 2;
  const base = mesh(group, sharedGeometry(context, "column-base", () => new THREE.BoxGeometry(0.94, 0.16, 0.94)),
    context.stone, 0, 0.08, 0);
  base.castShadow = true;
  mesh(group, sharedGeometry(context, "column-foot", () => new THREE.BoxGeometry(0.82, 0.18, 0.82)),
    context.darkStone, 0, 0.25, 0);
  const shaft = mesh(group, sharedGeometry(context, "column-shaft", () => new THREE.CylinderGeometry(0.29, 0.34, 1.16, 12)),
    context.stone, 0, 0.93, 0);
  shaft.castShadow = true;
  for (let i = 0; i < 8; i++) {
    const angle = i * Math.PI / 4;
    const flute = mesh(group, sharedGeometry(context, "column-flute", () => new THREE.BoxGeometry(0.045, 1.04, 0.055)),
      context.darkStone, Math.sin(angle) * 0.30, 0.93, Math.cos(angle) * 0.30);
    flute.rotation.y = angle;
  }
  const collar = mesh(group, sharedGeometry(context, "column-collar", () => new THREE.TorusGeometry(0.33, 0.045, 6, 12)),
    context.darkStone, 0, 0.43, 0);
  collar.rotation.x = Math.PI / 2;
  mesh(group, sharedGeometry(context, "column-cap-lower", () => new THREE.BoxGeometry(0.78, 0.14, 0.78)),
    context.darkStone, 0, 1.57, 0);
  mesh(group, sharedGeometry(context, "column-cap-upper", () => new THREE.BoxGeometry(0.92, 0.18, 0.92)),
    context.stone, 0, 1.73, 0);
  return { group, shaft };
}

export function makeRubble(context: BuildContext, x: number, z: number, variant: number) {
  const group = new THREE.Group();
  group.position.set(x, 0, z);
  const count = 3 + variant % 4;
  for (let i = 0; i < count; i++) {
    const angle = i * 2.399 + variant;
    const radius = 0.06 + (i % 3) * 0.055;
    const size = 0.065 + ((variant + i * 3) % 5) * 0.018;
    const rock = mesh(group, geometry(context, new THREE.IcosahedronGeometry(size, 0)),
      i % 3 ? context.stone : context.darkStone,
      Math.sin(angle) * radius, size * 0.54, Math.cos(angle) * radius);
    rock.scale.set(1.2, 0.56 + i % 2 * 0.25, 0.8);
    rock.rotation.y = angle;
  }
  return group;
}

function addFire(context: BuildContext, group: THREE.Group, x: number, y: number, z: number,
  cellIndex: number, phase: number, distance: number): FireProp {
  const flame = new THREE.Group();
  flame.position.set(x, y, z);
  group.add(flame);
  const outer = material(context, new THREE.MeshBasicMaterial({ color: 0xff7625, transparent: true, opacity: 0.75, depthWrite: false }));
  const inner = material(context, new THREE.MeshBasicMaterial({ color: 0xffdb75, transparent: true, opacity: 0.9, depthWrite: false }));
  for (const [radius, height, offset, fireMaterial] of [
    [0.115, 0.37, 0, outer], [0.073, 0.27, 0.035, inner], [0.058, 0.23, -0.055, outer],
  ] as const) {
    const cone = mesh(flame, geometry(context, new THREE.ConeGeometry(radius, height, 7)), fireMaterial, offset, height * 0.38, 0);
    cone.castShadow = false;
  }
  const sparkMaterial = material(context, new THREE.MeshBasicMaterial({ color: 0xffab4f, transparent: true, opacity: 0.85, depthWrite: false }));
  const sparkGeometry = geometry(context, new THREE.IcosahedronGeometry(0.017, 0));
  const sparks = Array.from({ length: 3 }, (_, i) => {
    const spark = mesh(flame, sparkGeometry, sparkMaterial, 0, 0, 0);
    spark.castShadow = false;
    spark.userData.sparkIndex = i;
    return spark;
  });
  const light = new THREE.PointLight(0xff8b43, 2.4, distance, 2);
  light.position.set(x, y + 0.13, z);
  // Point-light shadows multiply draw calls and reveal architecture beyond the fog.
  light.castShadow = false;
  group.add(light);
  return { group, flame, sparks, light, phase, cellIndex };
}

export function animateFire(fire: FireProp, time: number) {
  const t = time + fire.phase;
  fire.flame.scale.set(0.94 + 0.08 * Math.sin(t * 13), 0.86 + 0.16 * Math.sin(t * 9.1), 0.94 + 0.07 * Math.sin(t * 11));
  fire.flame.rotation.z = 0.1 * Math.sin(t * 6.7);
  fire.light.intensity = 2.15 + 0.22 * Math.sin(t * 8.3) + 0.12 * Math.sin(t * 15.7);
  fire.sparks.forEach((spark, i) => {
    const age = (t * (0.55 + i * 0.08) + i / 3) % 1;
    spark.position.set(Math.sin(t * 3 + i * 2) * age * 0.16, 0.1 + age * 0.53, Math.cos(t * 2.3 + i) * age * 0.12);
    spark.scale.setScalar(1 - age * 0.72);
  });
}
