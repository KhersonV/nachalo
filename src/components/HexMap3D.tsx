"use client";

import { useEffect, useRef } from "react";
import * as THREE from "three";
import { createProfessionNodeAssets } from "./hex3d/ProfessionNodes3D";
import { axialToWorld, canonicalHexEdge, hexDistance, hexKey, hexNeighbor, type HexCoord } from "@/utils/hexGrid";
import { AnimatedRanger, type RangerCue } from "./AnimatedRanger";
import { animateFire, disposeAtmosphere, makeAtmosphereContext, makeColumn, makeFloorBrazier, makeRubble, makeWallTorch, type FireProp } from "./catacombsAtmosphere";
import type { HexActor, HexCell, HexCombatIndicator, HexPlayer, HexRoute, HexSnapshot, HexVisualMove, HexVisualNotice } from "@/types/HexGameTypes";
import type { HexGameMessages } from "@/i18n/hexGame";
import styles from "@/styles/HexMap3D.module.css";
import { resourceImage } from "@/utils/resourceAssets";
import { createBarrelAssets, type Barrel3D } from "./hex3d/Barrel3D";
import { createPortalAssets, type Portal3D } from "./hex3d/Portal3D";
import { createQuestArtifactAssets, type QuestArtifact3D } from "./hex3d/QuestArtifact3D";

type Props = {
  snapshot: Pick<HexSnapshot, "instanceId" | "cells" | "edges" | "actors" | "questArtifact"> & { player?: Pick<HexPlayer, "userId" | "position" | "characterType" | "health" | "isRanged"> };
  visualNotices?: HexVisualNotice[];
  visualMove?: HexVisualMove | null;
  openedBarrel?: { commandId: string; instanceId: string; position: HexCoord } | null;
  connection?: string;
  selected?: HexCoord | null;
  route?: HexRoute | null;
  selectedTargetId?: number | null;
  selectedTargetKind?: "player" | "monster";
  indicators?: HexCombatIndicator[];
  busy?: boolean;
  onCellClick: (position: HexCoord) => void;
  onMonsterClick: (actor: HexActor) => void;
  onUnavailable: () => void;
  labels?: HexGameMessages;
};

const TILE_RADIUS = 1;
const hexWorld = (position: HexCoord) => axialToWorld(position, TILE_RADIUS);
const tileTop = (_cell: HexCell) => 0;
const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

type Runtime = {
  professionNodes: ReturnType<typeof createProfessionNodeAssets>;
  renderer: THREE.WebGLRenderer;
  scene: THREE.Scene;
  camera: THREE.PerspectiveCamera;
  world: THREE.Group;
  marks: THREE.Group;
  barrelAssets: ReturnType<typeof createBarrelAssets>;
  barrelModels: Map<string, Barrel3D>;
  portalAssets: ReturnType<typeof createPortalAssets>;
  portalModels: Map<string, Portal3D>;
  artifactAssets: ReturnType<typeof createQuestArtifactAssets>;
  artifactModels: Map<string, { image: string; model: QuestArtifact3D }>;
  barrelAnimation: { barrel: Barrel3D; started: number } | null;
  lastBarrelOpenId: string | null;
  gemGeometry: THREE.IcosahedronGeometry;
  resourceShadowGeometry: THREE.CircleGeometry;
  resourceLoader: THREE.TextureLoader;
  resourceVisuals: Map<string, { texture: THREE.Texture; material: THREE.SpriteMaterial }>;
  disposed: boolean;
  markerGeometry: THREE.CapsuleGeometry;
  ringGeometry: THREE.TorusGeometry;
  wallGeometry: THREE.BoxGeometry;
  wallCapGeometry: THREE.BoxGeometry;
  brazierEdgeGeometry: THREE.BoxGeometry;
  rockGeometries: THREE.BufferGeometry[];
  structureGeometry: THREE.BoxGeometry;
  discGeometry: THREE.CylinderGeometry;
  materials: Record<string, THREE.MeshStandardMaterial | THREE.MeshBasicMaterial | THREE.LineBasicMaterial>;
  tileHits: THREE.Mesh[];
  tileLookup: Map<THREE.Object3D, HexCell[]>;
  floorGeometries: THREE.BufferGeometry[];
  textures: THREE.Texture[];
  atmosphere: ReturnType<typeof makeAtmosphereContext> | null;
  fires: FireProp[];
  ranger: AnimatedRanger | null;
  rangerUserId: number | null;
  rangerPosition: HexCoord | null;
  movement: { commandId: string; position: HexCoord; points: THREE.Vector3[]; index: number } | null;
  lastMoveId: string | null;
  instanceId: string | null;
  heroFill: THREE.PointLight;
  heroRim: THREE.PointLight;
  hover: THREE.LineLoop;
  objectHits: THREE.Object3D[];
  actorHits: THREE.Object3D[];
  bounds: { minX: number; maxX: number; minZ: number; maxZ: number };
  focus: THREE.Vector3;
  distance: number;
  yaw: number;
  pitch: number;
  frame: number | null;
  lastFrame: number;
  invalidate: () => void;
};

function pointCamera(rt: Runtime) {
  rt.focus.x = clamp(rt.focus.x, rt.bounds.minX - 2, rt.bounds.maxX + 2);
  rt.focus.z = clamp(rt.focus.z, rt.bounds.minZ - 2, rt.bounds.maxZ + 2);
  const horizontal = rt.distance * Math.cos(rt.pitch);
  rt.camera.position.set(rt.focus.x + Math.sin(rt.yaw) * horizontal, rt.distance * Math.sin(rt.pitch), rt.focus.z + Math.cos(rt.yaw) * horizontal);
  rt.camera.lookAt(rt.focus);
  rt.invalidate();
}

function clearMarks(rt: Runtime) {
  for (const child of rt.marks.children) {
    if (child.userData.ownedGeometry) (child as THREE.Mesh).geometry.dispose();
    if (child.userData.ownedMaterial) {
      const material = (child as THREE.Sprite).material as THREE.SpriteMaterial;
      material.map?.dispose();
      material.dispose();
    }
  }
  rt.marks.clear();
}

function populateWorld(rt: Runtime, snapshot: Props["snapshot"], visualMove?: HexVisualMove | null, connection?: string) {
  rt.world.clear();
  rt.floorGeometries.forEach(geometry => geometry.dispose());
  rt.floorGeometries = [];
  if (rt.atmosphere) disposeAtmosphere(rt.atmosphere);
  rt.atmosphere = makeAtmosphereContext(rt.materials.wall, rt.materials["explored-rock"]);
  rt.fires = [];
  rt.tileHits = []; rt.objectHits = []; rt.actorHits = []; rt.tileLookup.clear();
  const cellsByKey = new Map(snapshot.cells.map(cell => [hexKey(cell.position), cell]));
  const groups = new Map<string, HexCell[]>();
  for (const cell of snapshot.cells) {
    if (cell.visibility === "unexplored") continue;
    const variant = cell.terrain === "rock" ? "stone" : String(Math.abs((cell.position.q * 92821 ^ cell.position.r * 68917) % 41) < 3
      ? 1 + Math.abs((cell.position.q * 17 + cell.position.r * 31) % 3) : 0);
    const kind = `${cell.visibility ?? "visible"}-${variant}`;
    const group = groups.get(kind) ?? [];
    group.push(cell); groups.set(kind, group);
  }
  const outline: number[] = [];
  const outlinedEdges = new Set<string>();
  for (const [kind, cells] of groups) {
    const positions: number[] = [], uvs: number[] = [], local: number[] = [];
    for (const cell of cells) {
      const { x, z } = hexWorld(cell.position);
      for (let side = 0; side < 6; side++) {
        const first = Math.PI / 6 + side * Math.PI / 3;
        const next = first + Math.PI / 3;
        // Reverse winding in X/Z so the surface normal faces the camera.
        for (const [px, pz] of [[x, z], [x + Math.cos(next), z + Math.sin(next)], [x + Math.cos(first), z + Math.sin(first)]]) {
          positions.push(px, 0, pz); uvs.push(px * .25, pz * .25); local.push(px - x, pz - z);
        }
      }
      if (cell.visibility === "visible") {
        for (let side = 0; side < 6; side++) {
          const a = Math.PI / 6 + side * Math.PI / 3, b = a + Math.PI / 3;
          const ax = x + Math.cos(a), az = z + Math.sin(a), bx = x + Math.cos(b), bz = z + Math.sin(b);
          const ends = [`${ax.toFixed(4)},${az.toFixed(4)}`, `${bx.toFixed(4)},${bz.toFixed(4)}`].sort();
          const key = ends.join(":");
          if (outlinedEdges.has(key)) continue;
          outlinedEdges.add(key);
          outline.push(ax, .012, az, bx, .012, bz);
        }
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
    geometry.setAttribute("uv", new THREE.Float32BufferAttribute(uvs, 2));
    geometry.setAttribute("hexLocal", new THREE.Float32BufferAttribute(local, 2));
    geometry.computeVertexNormals();
    const mesh = new THREE.Mesh(geometry, rt.materials[`floor-${kind}`]);
    mesh.receiveShadow = true;
    rt.floorGeometries.push(geometry); rt.tileHits.push(mesh); rt.tileLookup.set(mesh, cells); rt.world.add(mesh);
  }
  if (outline.length) {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(outline, 3));
    rt.floorGeometries.push(geometry);
    rt.world.add(new THREE.LineSegments(geometry, rt.materials.grid as THREE.LineBasicMaterial));
  }

  const occupied = new Set(snapshot.actors.map(actor => hexKey(actor.position)));
  if (snapshot.player) occupied.add(hexKey(snapshot.player.position));
  let decorCount = 0;
  for (const cell of snapshot.cells) {
    if (cell.visibility === "unexplored" || cell.terrain !== "rock") continue;
    const { x, z } = hexWorld(cell.position);
    const hash = Math.abs(cell.position.q * 1031 + cell.position.r * 701);
    const brazier = cell.environmentType === "brazier" || cell.structureType?.toLowerCase() === "brazier";
    const clear = !occupied.has(hexKey(cell.position)) && !cell.resource && !cell.barrel && !cell.isPortal && (!cell.structureType || brazier);
    if (brazier) {
      if (cell.visibility === "visible" && clear) {
        const fire = makeFloorBrazier(rt.atmosphere, x, z, rt.fires.length, hash);
        fire.group.scale.setScalar(1.3);
        rt.world.add(fire.group); rt.fires.push(fire);
      }
      continue;
    }
    if (!clear) continue;
    if (cell.visibility === "visible" && hash % 13 === 0) continue;
    const stone = rt.materials[cell.visibility === "visible" ? "rockStone" : "rockExplored"];
    const main = new THREE.Mesh(rt.rockGeometries[hash % 2], stone);
    main.position.set(x, .48 + hash % 3 * .07, z);
    main.scale.set(.92 + hash % 4 * .06, .75 + hash % 5 * .11, .83 + hash % 3 * .07);
    main.rotation.y = hash % 7 * Math.PI / 9;
    main.castShadow = cell.visibility === "visible"; main.receiveShadow = true;
    rt.world.add(main);
    if (hash % 3 !== 0) {
      const shard = new THREE.Mesh(rt.rockGeometries[2], stone);
      shard.position.set(x + ((hash % 3) - 1) * .26, .38, z + ((hash % 5) - 2) * .1);
      shard.rotation.y = hash % 6 * Math.PI / 6;
      shard.castShadow = cell.visibility === "visible";
      rt.world.add(shard);
    }
    if (cell.visibility === "visible" && decorCount < 12 && hash % 23 === 0) {
      rt.world.add(makeRubble(rt.atmosphere, x + .24, z - .14, hash)); decorCount++;
    }
  }

  const seenEdges = new Set<string>();
  let torchCount = 0;
  for (const edge of snapshot.edges) {
    const neighbor = hexNeighbor(edge.position, edge.direction);
    const canonical = canonicalHexEdge(edge.position, neighbor);
    const key = `${hexKey(canonical.a)}:${hexKey(canonical.b)}`;
    if (seenEdges.has(key)) continue;
    seenEdges.add(key);
    const aCell = cellsByKey.get(hexKey(edge.position)), bCell = cellsByKey.get(hexKey(neighbor));
    if (!aCell || !bCell || aCell.visibility === "unexplored" || bCell.visibility === "unexplored") continue;
    const lit = aCell.visibility === "visible" || bCell.visibility === "visible";
    const a = hexWorld(edge.position), b = hexWorld(neighbor);
    const dx = b.x - a.x, dz = b.z - a.z;
    const x = (a.x + b.x) / 2, z = (a.z + b.z) / 2;
    const rotation = -Math.atan2(dx, -dz);
    const brazierEdge = aCell.environmentType === "brazier" || bCell.environmentType === "brazier" ||
      aCell.structureType?.toLowerCase() === "brazier" || bCell.structureType?.toLowerCase() === "brazier";
    const wall = new THREE.Mesh(brazierEdge ? rt.brazierEdgeGeometry : rt.wallGeometry, rt.materials.wall);
    wall.position.set(x, brazierEdge ? .22 : .95, z); wall.rotation.y = rotation;
    wall.castShadow = true; wall.receiveShadow = true;
    wall.material = rt.materials[lit ? "wall" : "wallExplored"];
    rt.world.add(wall);
    if (!brazierEdge) {
      const cap = new THREE.Mesh(rt.wallCapGeometry, rt.materials.trim);
      cap.position.set(x, 1.94, z); cap.rotation.y = rotation; cap.castShadow = true;
      cap.material = rt.materials[lit ? "trim" : "trimExplored"];
      rt.world.add(cap);
    }
    const facing = aCell.visibility === "visible" && aCell.terrain === "floor" ? a : bCell.visibility === "visible" && bCell.terrain === "floor" ? b : null;
    if (!brazierEdge && facing && torchCount < 6 && (torchCount === 0 || Math.abs((edge.position.q * 73 + edge.position.r * 41 + edge.direction * 19) % 5) === 0)) {
      const face = facing;
      const torch = makeWallTorch(rt.atmosphere, x, z, Math.atan2(face.x - x, face.z - z), rt.fires.length, rt.fires.length * 1.7);
      torch.group.scale.setScalar(.66);
      rt.world.add(torch.group); rt.fires.push(torch); torchCount++;
    }
  }
  rt.heroFill.visible = !!snapshot.player;
  rt.heroRim.visible = !!snapshot.player;
  if (snapshot.player) {
    const position = hexWorld(snapshot.player.position);
    rt.heroFill.position.set(position.x, 2.2, position.z);
    rt.heroRim.position.set(position.x - 1.2, 2.8, position.z - 1.2);
  }
  for (const cell of snapshot.cells) {
    if (cell.visibility !== "visible" || cell.terrain !== "rock" || decorCount >= 16) continue;
    if (cell.environmentType === "brazier" || cell.structureType?.toLowerCase() === "brazier" || occupied.has(hexKey(cell.position)) || cell.resource || cell.barrel || cell.isPortal || cell.structureType) continue;
    const hash = Math.abs(cell.position.q * 1031 + cell.position.r * 701);
    const { x, z } = hexWorld(cell.position);
    if (hash % 13 === 0) {
      const { group } = makeColumn(rt.atmosphere, x, z, hash);
      rt.world.add(group); decorCount++;
    }
  }

  const visibleBarrels = new Set<string>();
  const visiblePortals = new Set<string>();
  const visibleArtifacts = new Set<string>();
  for (const cell of snapshot.cells) {
    if (cell.visibility === "unexplored" || cell.visibility === "explored") continue;
    const { x, z } = hexWorld(cell.position);
    const base = tileTop(cell);
    const addObject = (object: THREE.Object3D) => { object.userData.cell = cell; rt.objectHits.push(object); rt.world.add(object); };
    if (cell.isPortal) {
      const key = hexKey(cell.position);
      visiblePortals.add(key);
      let portal = rt.portalModels.get(key);
      if (!portal) { portal = rt.portalAssets.create(); rt.portalModels.set(key, portal); }
      portal.group.position.set(x, base, z);
      rt.world.add(portal.group);
      for (const mesh of portal.hitMeshes) { mesh.userData.cell = cell; rt.objectHits.push(mesh); }
    }
    if (cell.structureType && cell.structureType.toLowerCase() !== "brazier") {
      const structure = new THREE.Mesh(rt.structureGeometry, rt.materials.structure);
      structure.position.set(x, base + .3, z); addObject(structure);
    }
    if (cell.barrel) {
      const key = hexKey(cell.position);
      visibleBarrels.add(key);
      let barrel = rt.barrelModels.get(key);
      if (!barrel) { barrel = rt.barrelAssets.create(); rt.barrelModels.set(key, barrel); }
      barrel.group.position.set(x, base, z);
      rt.world.add(barrel.group);
      for (const mesh of barrel.hitMeshes) { mesh.userData.cell = cell; rt.objectHits.push(mesh); }
    }
    if (cell.resource?.professionNode) {
      const group = rt.professionNodes.create(cell.resource.professionNode.category);
      group.position.set(x + .32, base, z + .2); rt.world.add(group);
      group.traverse(object => { if (object instanceof THREE.Mesh) { object.userData.cell = cell; rt.objectHits.push(object); } });
      continue;
    }
    if (cell.resource) {
      const quest = snapshot.questArtifact;
      if (cell.visibility === "visible" && cell.resource.item_type === "artifact" && quest?.status === "ground" &&
          quest.id === cell.resource.id && quest.position && hexKey(quest.position) === hexKey(cell.position)) {
        const key = hexKey(cell.position);
        const image = quest.image || cell.resource.image || "";
        visibleArtifacts.add(key);
        let entry = rt.artifactModels.get(key);
        if (!entry || entry.image !== image) {
          entry = { image, model: rt.artifactAssets.create(image) };
          rt.artifactModels.set(key, entry);
        }
        entry.model.group.position.set(x, base, z);
        rt.world.add(entry.model.group);
        for (const object of entry.model.hitObjects) { object.userData.cell = cell; rt.objectHits.push(object); }
        continue;
      }
      const asset = cell.resource.item_type === "artifact" ? null : resourceImage(cell.resource.type, cell.resource.image);
      if (asset) {
        let visual = rt.resourceVisuals.get(asset);
        if (!visual) {
          const texture = rt.resourceLoader.load(asset, () => { if (!rt.disposed) rt.invalidate(); });
          texture.colorSpace = THREE.SRGBColorSpace;
          const material = new THREE.SpriteMaterial({ map: texture, transparent: true, alphaTest: .08, depthWrite: false, toneMapped: false });
          visual = { texture, material };
          rt.resourceVisuals.set(asset, visual);
        }
        const offset = cell.isPortal ? .28 : 0;
        const shadow = new THREE.Mesh(rt.resourceShadowGeometry, rt.materials.resourceShadow);
        shadow.rotation.x = -Math.PI / 2;
        shadow.position.set(x + offset, base + .014, z);
        rt.world.add(shadow);
        const sprite = new THREE.Sprite(visual.material);
        sprite.center.set(.5, 0);
        sprite.scale.set(.82, .82, 1);
        sprite.position.set(x + offset, base + .06, z);
        addObject(sprite);
      } else {
        const gem = new THREE.Mesh(rt.gemGeometry, cell.resource.item_type === "artifact" ? rt.materials.artifact : rt.materials.resource);
        gem.position.set(x + (cell.isPortal ? .32 : 0), base + .31, z); addObject(gem);
      }
    }
  }
  for (const key of rt.barrelModels.keys()) if (!visibleBarrels.has(key)) rt.barrelModels.delete(key);
  for (const key of rt.portalModels.keys()) if (!visiblePortals.has(key)) rt.portalModels.delete(key);
  for (const key of rt.artifactModels.keys()) if (!visibleArtifacts.has(key)) rt.artifactModels.delete(key);

  const actors = [...snapshot.actors];
  if (snapshot.player && !actors.some(actor => actor.kind === "player" && actor.userId === snapshot.player?.userId)) {
    actors.push({ id: `player-${snapshot.player.userId}`, kind: "player", userId: snapshot.player.userId, position: snapshot.player.position });
  }
  for (const actor of actors) {
    const self = actor.kind === "player" && actor.userId === snapshot.player?.userId;
    const cell = cellsByKey.get(hexKey(actor.position));
    if (!cell || !self && cell.visibility !== "visible" && cell.visibility !== undefined) continue;
    const { x, z } = hexWorld(actor.position);
    const rangerSelf = self && snapshot.player?.characterType?.toLowerCase() === "ranger";
    const marker = new THREE.Mesh(rt.markerGeometry, rt.materials[rangerSelf ? "invisibleHit" : self ? "self" : actor.kind === "monster" ? "monster" : "ally"]);
    marker.position.set(x, tileTop(cell) + .52, z);
    marker.userData.actor = actor;
    rt.actorHits.push(marker); rt.world.add(marker);
    const base = new THREE.Mesh(rt.ringGeometry, rt.materials[self ? "selfRing" : "actorRing"]);
    base.rotation.x = -Math.PI / 2; base.position.set(x, tileTop(cell) + .04, z);
    rt.world.add(base);
  }

  const player = snapshot.player;
  const isRanger = player?.characterType?.toLowerCase() === "ranger";
  if (!isRanger || !player) {
    rt.ranger?.dispose(); rt.ranger = null; rt.rangerUserId = null; rt.rangerPosition = null; rt.movement = null;
  } else {
    const world = hexWorld(player.position);
    const sameInstance = rt.instanceId === null || rt.instanceId === snapshot.instanceId;
    const expectedMove = visualMove?.instanceId === snapshot.instanceId &&
      (hexKey(player.position) === hexKey(visualMove.from) || hexKey(player.position) === hexKey(visualMove.position));
    if (!sameInstance || connection !== "live" || !expectedMove) rt.movement = null;
    const hold = sameInstance && connection === "live" && expectedMove &&
      (rt.movement !== null || rt.lastMoveId !== visualMove?.commandId);
    if (!rt.ranger || rt.rangerUserId !== player.userId) {
      rt.ranger?.dispose();
      rt.ranger = new AnimatedRanger({ position: { x: world.x, y: world.z }, model: "animated", defeated: player.health <= 0 });
      rt.rangerUserId = player.userId;
      rt.scene.add(rt.ranger.root);
      rt.ranger.onReady = rt.invalidate;
    } else {
      const previous = rt.rangerPosition;
      if (!hold) {
        if (!sameInstance || connection !== "live" || previous && hexDistance(previous, player.position) > 1)
          rt.ranger.snapTo({ x: world.x, y: world.z });
        rt.ranger.setSpec({ position: { x: world.x, y: world.z }, defeated: player.health <= 0 }, performance.now() * .001);
      }
    }
    rt.rangerPosition = player.position;
  }
  rt.instanceId = snapshot.instanceId;

  const coords = snapshot.cells.map(cell => hexWorld(cell.position));
  rt.bounds = { minX: Math.min(...coords.map(p => p.x), 0), maxX: Math.max(...coords.map(p => p.x), 0), minZ: Math.min(...coords.map(p => p.z), 0), maxZ: Math.max(...coords.map(p => p.z), 0) };
  rt.scene.updateMatrixWorld(true);
  pointCamera(rt);
}

function populateMarks(rt: Runtime, snapshot: Props["snapshot"], selected: HexCoord | null | undefined, route: HexRoute | null | undefined, targetId: number | null | undefined, targetKind: "player" | "monster" | undefined, indicators: HexCombatIndicator[]) {
  clearMarks(rt);
  const cellsByKey = new Map(snapshot.cells.map(cell => [hexKey(cell.position), cell]));
  const availableSteps = route?.affordableSteps ?? (route?.affordable ? route.path.length : 0);
  for (const [index, position] of (route?.path ?? []).entries()) {
    const cell = cellsByKey.get(hexKey(position));
    if (!cell || cell.visibility === "unexplored") continue;
    const { x, z } = hexWorld(position);
    const marker = new THREE.Mesh(rt.discGeometry, rt.materials[index < availableSteps ? "route" : "blocked"]);
    marker.position.set(x, tileTop(cell) + .035, z); rt.marks.add(marker);
  }
  if (route?.reachablePosition && availableSteps > 0) {
    const cell = cellsByKey.get(hexKey(route.reachablePosition));
    if (cell && cell.visibility !== "unexplored") {
      const { x, z } = hexWorld(route.reachablePosition);
      const ring = new THREE.Mesh(rt.ringGeometry, rt.materials.routeEnd);
      ring.rotation.x = -Math.PI / 2; ring.position.set(x, tileTop(cell) + .11, z); rt.marks.add(ring);
    }
  }
  if (selected) {
    const cell = cellsByKey.get(hexKey(selected));
    if (cell && cell.visibility !== "unexplored") {
      const { x, z } = hexWorld(selected);
      const ring = new THREE.Mesh(rt.ringGeometry, rt.materials[targetId != null && route ? "attackPoint" : "selected"]);
      ring.rotation.x = -Math.PI / 2; ring.position.set(x, tileTop(cell) + .08, z); rt.marks.add(ring);
    }
  }
  const target = snapshot.actors.find(actor => actor.kind === targetKind && (actor.kind === "player" ? actor.userId : Number(actor.id.replace(/^monster-/, ""))) === targetId);
  const targetCell = target && cellsByKey.get(hexKey(target.position));
  if (target && targetCell && targetCell.visibility !== "unexplored") {
    const { x, z } = hexWorld(target.position);
    const ring = new THREE.Mesh(rt.ringGeometry, rt.materials.target);
    ring.rotation.x = -Math.PI / 2; ring.position.set(x, .2, z); rt.marks.add(ring);
  }
  for (const indicator of indicators) {
    const cell = cellsByKey.get(hexKey(indicator.position));
    if (!cell || cell.visibility !== "visible") continue;
    const canvas = document.createElement("canvas"); canvas.width = 256; canvas.height = 64;
    const context = canvas.getContext("2d");
    if (!context) continue;
    context.font = "bold 27px sans-serif"; context.textAlign = "center";
    context.strokeStyle = "#08111a"; context.lineWidth = 6; context.strokeText(indicator.text, 128, 40);
    context.fillStyle = indicator.tone === "damage" || indicator.tone === "critical" ? "#ff8c79" : "#d7ecf1";
    context.fillText(indicator.text, 128, 40);
    const texture = new THREE.CanvasTexture(canvas);
    const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: texture, transparent: true, depthTest: false }));
    const { x, z } = hexWorld(indicator.position);
    sprite.position.set(x, tileTop(cell) + 1.25 + indicator.lane * .3, z); sprite.scale.set(1.8, .45, 1);
    sprite.userData.ownedMaterial = true; rt.marks.add(sprite);
  }
  rt.invalidate();
}

export default function HexMap3D({ snapshot, visualNotices = [], visualMove, openedBarrel, connection, selected, route, selectedTargetId, selectedTargetKind, indicators = [], busy, onCellClick, onMonsterClick, onUnavailable, labels }: Props) {
  const host = useRef<HTMLDivElement>(null);
  const runtime = useRef<Runtime | null>(null);
  const current = useRef({ busy, onCellClick, onMonsterClick, onUnavailable });
  current.current = { busy, onCellClick, onMonsterClick, onUnavailable };

  useEffect(() => {
    const element = host.current;
    if (!element) return;
    let renderer: THREE.WebGLRenderer;
    try { renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: "high-performance" }); }
    catch { current.current.onUnavailable(); return; }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.28;
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
    renderer.setClearColor(0x101820, 0);
    renderer.domElement.style.touchAction = "none";
    element.appendChild(renderer.domElement);
    const scene = new THREE.Scene();
    scene.background = null;
    scene.fog = new THREE.Fog(0x101820, 36, 88);
    scene.add(new THREE.HemisphereLight(0xb0bfd0, 0x424449, 1.65));
    const sunlight = new THREE.DirectionalLight(0xd3dfeb, 1.5);
    sunlight.position.set(-7, 16, 9);
    sunlight.castShadow = true;
    sunlight.shadow.mapSize.set(1024, 1024);
    sunlight.shadow.camera.left = sunlight.shadow.camera.bottom = -22;
    sunlight.shadow.camera.right = sunlight.shadow.camera.top = 22;
    scene.add(sunlight);
    const heroFill = new THREE.PointLight(0xffbc80, 1.7, 5.5, 2);
    heroFill.visible = false;
    scene.add(heroFill);
    const heroRim = new THREE.PointLight(0x94bdde, .95, 4, 2);
    heroRim.visible = false;
    scene.add(heroRim);
    const camera = new THREE.PerspectiveCamera(45, 1, .1, 500);
    const world = new THREE.Group(), marks = new THREE.Group(); scene.add(world, marks);
    const standard = (color: number, options: THREE.MeshStandardMaterialParameters = {}) => new THREE.MeshStandardMaterial({ color, roughness: .93, metalness: .03, flatShading: true, ...options });
    const basic = (color: number, opacity = 1) => new THREE.MeshBasicMaterial({ color, transparent: opacity < 1, opacity, depthWrite: opacity === 1 });
    let invalidateTexture = () => {};
    const textureLoader = new THREE.TextureLoader();
    const textures = ["floor_base", "floor_cracked", "floor_moss", "floor_runes", "floor_stone", "wall_base", "stone_trim", "wall_stone"].map(name => {
      const texture = textureLoader.load(`/assets/nachalo/catacombs/${name}.webp`, () => invalidateTexture());
      texture.colorSpace = THREE.SRGBColorSpace;
      texture.wrapS = texture.wrapT = THREE.MirroredRepeatWrapping;
      texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy());
      return texture;
    });
    const materials: Record<string, THREE.MeshStandardMaterial | THREE.MeshBasicMaterial | THREE.LineBasicMaterial> = {
      "visible-floor": standard(0x52616a), "visible-floor-side": standard(0x28343d),
      "visible-rock": standard(0x343d42), "visible-rock-side": standard(0x1d262b),
      "explored-floor": standard(0x293944), "explored-floor-side": standard(0x17232d),
      "explored-rock": standard(0x202b34), "explored-rock-side": standard(0x131d26),
      unknown: standard(0x080e14), "unknown-side": standard(0x060a10),
      wall: standard(0xffffff, { map: textures[5], emissive: 0x302a23, emissiveIntensity: .35 }),
      wallExplored: new THREE.MeshBasicMaterial({ color: 0x838b8e, map: textures[5] }),
      rockStone: standard(0xc6c8c1, { map: textures[7], emissive: 0x454740, emissiveIntensity: .32 }),
      rockExplored: new THREE.MeshBasicMaterial({ color: 0x777f81, map: textures[7] }),
      trim: standard(0xffffff, { map: textures[6] }),
      trimExplored: new THREE.MeshBasicMaterial({ color: 0x7b8588, map: textures[6] }),
      grid: new THREE.LineBasicMaterial({ color: 0xb3c4c4, transparent: true, opacity: .065, depthWrite: false }),
      invisibleHit: new THREE.MeshBasicMaterial({ transparent: true, opacity: 0, depthWrite: false }),
      resource: standard(0x76bfc5, { emissive: 0x397b85, emissiveIntensity: .7 }),
      resourceShadow: basic(0x050a0c, .3),
      artifact: standard(0xd9b96a, { emissive: 0x8e6527, emissiveIntensity: .8 }), structure: standard(0x92968b),
      self: standard(0x83d8e9, { emissive: 0x235762, emissiveIntensity: .35 }),
      monster: standard(0xcf8072, { emissive: 0x5c2420, emissiveIntensity: .55 }), ally: standard(0xe2cc94, { emissive: 0x4c4021, emissiveIntensity: .4 }),
      selfRing: basic(0x69e1ef), actorRing: basic(0xc9ac7a), selected: basic(0x77e7f1),
      routeEnd: basic(0xc4fff4), target: basic(0xf0bf67), attackPoint: basic(0xf5d78c), route: basic(0x52d8d8, .3), blocked: basic(0xe78b77, .3),
    };
    for (const visibility of ["visible", "explored"] as const) {
      for (let variant = 0; variant < 4; variant++) {
        const material = visibility === "visible"
          ? standard(0xffffff, { map: textures[0], emissive: 0x91948c, emissiveMap: textures[0], emissiveIntensity: .72 })
          : new THREE.MeshBasicMaterial({ color: 0x9ca6a9, map: textures[0] });
        if (variant > 0) {
          material.onBeforeCompile = shader => {
            shader.uniforms.variantMap = { value: textures[variant] };
            shader.vertexShader = `attribute vec2 hexLocal; varying vec2 vHexLocal;\n${shader.vertexShader}`.replace(
              "#include <uv_vertex>", "#include <uv_vertex>\n vHexLocal = hexLocal;");
            shader.fragmentShader = `uniform sampler2D variantMap; varying vec2 vHexLocal;\n${shader.fragmentShader}`.replace(
              "#include <map_fragment>", `#ifdef USE_MAP
                vec4 baseStone = texture2D(map, vMapUv);
                vec4 variantStone = texture2D(variantMap, vMapUv);
                float edgeDistance = 0.8660254 - max(abs(vHexLocal.x), max(abs(0.5 * vHexLocal.x + 0.8660254 * vHexLocal.y), abs(-0.5 * vHexLocal.x + 0.8660254 * vHexLocal.y)));
                diffuseColor *= mix(baseStone, variantStone, smoothstep(0.02, 0.25, edgeDistance));
              #endif`);
          };
          material.customProgramCacheKey = () => "hex-stone-variant-v1";
        }
        materials[`floor-${visibility}-${variant}`] = material;
      }
      materials[`floor-${visibility}-stone`] = visibility === "visible"
        ? standard(0xd5d8d4, { map: textures[4], emissive: 0x878782, emissiveMap: textures[4], emissiveIntensity: .68 })
        : new THREE.MeshBasicMaterial({ color: 0x939d9f, map: textures[4] });
    }
    const hoverGeometry = new THREE.BufferGeometry();
    const hoverPoints: number[] = [];
    for (let side = 0; side < 6; side++) {
      const angle = Math.PI / 6 + side * Math.PI / 3;
      hoverPoints.push(Math.cos(angle) * .99, .06, Math.sin(angle) * .99);
    }
    hoverGeometry.setAttribute("position", new THREE.Float32BufferAttribute(hoverPoints, 3));
    const hover = new THREE.LineLoop(hoverGeometry, new THREE.LineBasicMaterial({ color: 0x8eeeff, transparent: true, opacity: .85, depthWrite: false }));
    hover.visible = false; scene.add(hover);
    const rt: Runtime = {
      renderer, scene, camera, world, marks,
      barrelAssets: createBarrelAssets(), barrelModels: new Map(), barrelAnimation: null, lastBarrelOpenId: null,
      portalAssets: createPortalAssets(textures[5]), portalModels: new Map(),
      artifactAssets: createQuestArtifactAssets(textureLoader, () => invalidateTexture(), textures[5]), artifactModels: new Map(),
      gemGeometry: new THREE.IcosahedronGeometry(.29, 0),
      professionNodes: createProfessionNodeAssets(),
      resourceShadowGeometry: new THREE.CircleGeometry(.34, 24), resourceLoader: textureLoader, resourceVisuals: new Map(), disposed: false,
      markerGeometry: new THREE.CapsuleGeometry(.18, .4, 4, 8),
      ringGeometry: new THREE.TorusGeometry(.66, .045, 4, 36), wallGeometry: new THREE.BoxGeometry(1.12, 1.9, .23),
      wallCapGeometry: new THREE.BoxGeometry(1.16, .12, .28),
      brazierEdgeGeometry: new THREE.BoxGeometry(1.08, .44, .2),
      rockGeometries: [new THREE.IcosahedronGeometry(.73, 0), new THREE.DodecahedronGeometry(.7, 0), new THREE.ConeGeometry(.44, .96, 5)],
      structureGeometry: new THREE.BoxGeometry(.48, .58, .48), discGeometry: new THREE.CylinderGeometry(.78, .78, .025, 6),
      materials, tileHits: [], tileLookup: new Map(), floorGeometries: [], textures, atmosphere: null, fires: [],
      ranger: null, rangerUserId: null, rangerPosition: null, movement: null, lastMoveId: null,
      instanceId: null, heroFill, heroRim, hover, objectHits: [], actorHits: [],
      bounds: { minX: -10, maxX: 10, minZ: -10, maxZ: 10 }, focus: new THREE.Vector3(), distance: 24, yaw: .7, pitch: Math.PI / 4,
      frame: null, lastFrame: 0, invalidate: () => {},
    };
    const render = (now: number) => {
      rt.frame = null;
      const delta = rt.lastFrame ? Math.min((now - rt.lastFrame) / 1000, .05) : 0;
      rt.lastFrame = now;
      const seconds = now * .001;
      if (rt.portalModels.size) rt.portalAssets.update(seconds, rt.portalModels.values());
      for (const entry of rt.artifactModels.values()) rt.artifactAssets.update(entry.model, seconds);
      if (rt.barrelAnimation) {
        const { barrel, started } = rt.barrelAnimation;
        const elapsed = seconds - started;
        const pulse = Math.sin(Math.min(elapsed / .55, 1) * Math.PI);
        barrel.lid.position.y = .835 + pulse * .22;
        barrel.lid.rotation.z = -pulse * .2;
        barrel.group.rotation.z = Math.sin(elapsed * 55) * .025 * Math.max(0, 1 - elapsed / .55);
        if (elapsed >= .55) { scene.remove(barrel.group); rt.barrelAnimation = null; }
      }
      if (rt.ranger?.status.failed && rt.ranger.model === "animated" && rt.rangerPosition) {
        const position = rt.ranger.root.position.clone();
        rt.ranger.dispose();
        rt.ranger = new AnimatedRanger({ position: { x: position.x, y: position.z }, model: "legacy", defeated: (snapshotRef.current.player?.health ?? 1) <= 0 });
        rt.scene.add(rt.ranger.root);
        rt.ranger.onReady = rt.invalidate;
        const next = rt.movement?.points[rt.movement.index];
        if (next) rt.ranger.setSpec({ position: { x: next.x, y: next.z } }, seconds);
      }
      rt.ranger?.update(delta, seconds);
      const movement = rt.movement;
      if (rt.ranger && movement) {
        let next = movement.points[movement.index];
        while (next && rt.ranger.root.position.distanceTo(next) < .025) {
          movement.index++;
          next = movement.points[movement.index];
          if (next) {
            rt.ranger.setSpec({ position: { x: next.x, y: next.z } }, seconds);
            rt.ranger.update(0, seconds);
          } else rt.movement = null;
        }
      }
      if (rt.ranger?.isReady) {
        rt.heroFill.position.set(rt.ranger.root.position.x, 2.2, rt.ranger.root.position.z);
        rt.heroRim.position.set(rt.ranger.root.position.x - 1.2, 2.8, rt.ranger.root.position.z - 1.2);
      }
      const lit = new Set(rt.fires.map((_, index) => index).sort((a, b) =>
        rt.fires[a].group.position.distanceToSquared(rt.heroFill.position) - rt.fires[b].group.position.distanceToSquared(rt.heroFill.position)).slice(0, 4));
      for (const [index, fire] of rt.fires.entries()) {
        animateFire(fire, seconds);
        fire.light.intensity *= .65;
        fire.light.visible = lit.has(index);
      }
      renderer.render(scene, camera);
      if (rt.ranger || rt.fires.length || rt.barrelAnimation || rt.portalModels.size || rt.artifactModels.size) rt.frame = window.requestAnimationFrame(render);
    };
    rt.invalidate = () => { if (rt.frame === null) rt.frame = window.requestAnimationFrame(render); };
    invalidateTexture = rt.invalidate;
    runtime.current = rt;
    const resize = () => {
      const rect = element.getBoundingClientRect();
      if (!rect.width || !rect.height) return;
      renderer.setSize(rect.width, rect.height, false);
      camera.aspect = rect.width / rect.height; camera.updateProjectionMatrix(); rt.invalidate();
    };
    const observer = new ResizeObserver(resize); observer.observe(element); resize();
    const raycaster = new THREE.Raycaster(), pointer = new THREE.Vector2();
    const pick = (event: PointerEvent) => {
      if (current.current.busy) return;
      const rect = renderer.domElement.getBoundingClientRect();
      pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, -(event.clientY - rect.top) / rect.height * 2 + 1);
      scene.updateMatrixWorld(true); raycaster.setFromCamera(pointer, camera);
      for (const hit of raycaster.intersectObjects([...rt.actorHits, ...rt.objectHits, ...rt.tileHits], false)) {
        const actor = hit.object.userData.actor as HexActor | undefined;
        if (actor) {
          if (actor.kind === "player" && actor.userId === snapshotRef.current.player?.userId) current.current.onCellClick(actor.position);
          else current.current.onMonsterClick(actor);
          return;
        }
        const objectCell = hit.object.userData.cell as HexCell | undefined;
        if (objectCell) { current.current.onCellClick(objectCell.position); return; }
        const cell = rt.tileLookup.get(hit.object)?.[Math.floor((hit.faceIndex ?? -6) / 6)];
        if (cell) { if (cell.visibility !== "unexplored") current.current.onCellClick(cell.position); return; }
      }
    };
    type Drag = { x: number; y: number; focusX: number; focusZ: number; yaw: number; pitch: number; panScale: number; button: number; moved: boolean };
    let drag: Drag | null = null;
    const down = (event: PointerEvent) => {
      if (event.button !== 0 && event.button !== 2) return;
      const panScale = 2 * rt.distance * Math.tan(THREE.MathUtils.degToRad(camera.fov) / 2) / Math.max(renderer.domElement.clientHeight, 1);
      drag = { x: event.clientX, y: event.clientY, focusX: rt.focus.x, focusZ: rt.focus.z, yaw: rt.yaw, pitch: rt.pitch, panScale, button: event.button, moved: false };
      renderer.domElement.setPointerCapture(event.pointerId);
    };
    const move = (event: PointerEvent) => {
      if (!drag) {
        const rect = renderer.domElement.getBoundingClientRect();
        pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, -(event.clientY - rect.top) / rect.height * 2 + 1);
        raycaster.setFromCamera(pointer, camera);
        const hit = raycaster.intersectObjects([...rt.objectHits, ...rt.tileHits], false)[0];
        const cell = hit && ((hit.object.userData.cell as HexCell | undefined) ?? rt.tileLookup.get(hit.object)?.[Math.floor((hit.faceIndex ?? -6) / 6)]);
        rt.hover.visible = !!cell && cell.visibility !== "unexplored";
        if (cell) { const position = hexWorld(cell.position); rt.hover.position.set(position.x, 0, position.z); }
        rt.invalidate();
        return;
      }
      const dx = event.clientX - drag.x, dy = event.clientY - drag.y;
      if (!drag.moved && Math.hypot(dx, dy) < 6) return;
      drag.moved = true;
      if (drag.button === 2) {
        rt.yaw = drag.yaw - dx * .006;
        rt.pitch = clamp(drag.pitch + dy * .004, Math.PI / 6, Math.PI * .36);
      } else {
        const rightX = Math.cos(drag.yaw), rightZ = -Math.sin(drag.yaw);
        const upX = -Math.sin(drag.yaw), upZ = -Math.cos(drag.yaw);
        const verticalScale = drag.panScale / Math.sin(drag.pitch);
        rt.focus.x = drag.focusX - dx * rightX * drag.panScale + dy * upX * verticalScale;
        rt.focus.z = drag.focusZ - dx * rightZ * drag.panScale + dy * upZ * verticalScale;
      }
      pointCamera(rt);
    };
    const up = (event: PointerEvent) => {
      const was = drag; drag = null;
      if (renderer.domElement.hasPointerCapture(event.pointerId)) renderer.domElement.releasePointerCapture(event.pointerId);
      if (was?.button === 0 && !was.moved && event.type === "pointerup") pick(event);
    };
    const wheel = (event: WheelEvent) => { event.preventDefault(); rt.distance = clamp(rt.distance * Math.exp(event.deltaY * .001), 7, 80); pointCamera(rt); };
    const contextMenu = (event: MouseEvent) => event.preventDefault();
    const leave = () => { rt.hover.visible = false; rt.invalidate(); };
    const canvas = renderer.domElement;
    const contextLost = (event: Event) => { event.preventDefault(); current.current.onUnavailable(); };
    canvas.addEventListener("pointerdown", down); canvas.addEventListener("pointermove", move); canvas.addEventListener("pointerleave", leave);
    canvas.addEventListener("pointerup", up); canvas.addEventListener("pointercancel", up);
    canvas.addEventListener("wheel", wheel, { passive: false }); canvas.addEventListener("contextmenu", contextMenu); canvas.addEventListener("webglcontextlost", contextLost);
    return () => {
      rt.disposed = true;
      observer.disconnect();
      canvas.removeEventListener("pointerdown", down); canvas.removeEventListener("pointermove", move); canvas.removeEventListener("pointerleave", leave);
      canvas.removeEventListener("pointerup", up); canvas.removeEventListener("pointercancel", up);
      canvas.removeEventListener("wheel", wheel); canvas.removeEventListener("contextmenu", contextMenu); canvas.removeEventListener("webglcontextlost", contextLost);
      if (rt.frame !== null) window.cancelAnimationFrame(rt.frame);
      if (rt.barrelAnimation) rt.scene.remove(rt.barrelAnimation.barrel.group);
      rt.ranger?.dispose();
      clearMarks(rt); rt.world.clear();
      rt.floorGeometries.forEach(geometry => geometry.dispose());
      if (rt.atmosphere) disposeAtmosphere(rt.atmosphere);
      rt.hover.geometry.dispose(); (rt.hover.material as THREE.Material).dispose();
      rt.professionNodes.dispose();
      rt.barrelAssets.dispose();
      rt.barrelModels.clear();
      rt.portalAssets.dispose(); rt.portalModels.clear();
      rt.artifactAssets.dispose(); rt.artifactModels.clear();
      for (const geometry of [rt.gemGeometry, rt.resourceShadowGeometry, rt.markerGeometry, rt.ringGeometry, rt.wallGeometry, rt.wallCapGeometry, rt.brazierEdgeGeometry, ...rt.rockGeometries, rt.structureGeometry, rt.discGeometry]) geometry.dispose();
      for (const visual of rt.resourceVisuals.values()) { visual.material.dispose(); visual.texture.dispose(); }
      for (const material of Object.values(rt.materials)) material.dispose();
      rt.textures.forEach(texture => texture.dispose());
      invalidateTexture = () => {};
      renderer.dispose(); renderer.forceContextLoss(); canvas.remove(); runtime.current = null; initialized.current = false;
    };
  }, []);

  const snapshotRef = useRef(snapshot);
  snapshotRef.current = snapshot;
  useEffect(() => {
    const rt = runtime.current;
    if (!rt) return;
    populateWorld(rt, snapshot, visualMove, connection);
    if (snapshot.player && !initialized.current) {
      const world = hexWorld(snapshot.player.position);
      rt.focus.set(world.x, 0, world.z);
      const spread = Math.max(rt.bounds.maxX - rt.bounds.minX, rt.bounds.maxZ - rt.bounds.minZ);
      rt.distance = clamp(spread * .7, 14, 32);
      initialized.current = true; pointCamera(rt);
    }
  }, [snapshot.instanceId, snapshot.cells, snapshot.edges, snapshot.actors, snapshot.player, snapshot.questArtifact, visualMove, connection]);
  useEffect(() => {
    const rt = runtime.current;
    if (!rt) return;
    const key = selected && hexKey(selected);
    for (const [position, portal] of rt.portalModels) rt.portalAssets.setSelected(portal, position === key);
    for (const [position, artifact] of rt.artifactModels) artifact.model.selection.visible = position === key;
    rt.invalidate();
  }, [selected, snapshot.cells, snapshot.questArtifact]);
  useEffect(() => {
    const rt = runtime.current;
    if (!rt || !openedBarrel || openedBarrel.instanceId !== snapshot.instanceId || connection !== "live" || rt.lastBarrelOpenId === openedBarrel.commandId) return;
    const cell = snapshot.cells.find(candidate => hexKey(candidate.position) === hexKey(openedBarrel.position));
    if (!cell || cell.visibility === "unexplored" || cell.visibility === "explored" || cell.barrel) return;
    rt.lastBarrelOpenId = openedBarrel.commandId;
    if (rt.barrelAnimation) rt.scene.remove(rt.barrelAnimation.barrel.group);
    const barrel = rt.barrelAssets.create();
    const position = hexWorld(openedBarrel.position);
    barrel.group.position.set(position.x, tileTop(cell), position.z);
    rt.scene.add(barrel.group);
    rt.barrelAnimation = { barrel, started: performance.now() * .001 };
    rt.invalidate();
  }, [openedBarrel, snapshot.instanceId, snapshot.cells, connection]);
  useEffect(() => {
    const rt = runtime.current;
    if (!rt?.ranger || !visualMove || !visualMove.path || visualMove.path.length === 0 || connection !== "live" || visualMove.instanceId !== snapshot.instanceId ||
        rt.lastMoveId === visualMove.commandId) return;
    const path = visualMove.path;
    let previous = visualMove.from;
    for (const point of path) {
      if (hexDistance(previous, point) !== 1) return;
      previous = point;
    }
    if (hexKey(previous) !== hexKey(visualMove.position)) return;
    const points = path.map(point => {
      const world = hexWorld(point);
      return new THREE.Vector3(world.x, 0, world.z);
    });
    if (rt.movement && hexKey(rt.movement.position) === hexKey(visualMove.from)) {
      rt.movement.points.push(...points);
      rt.movement.position = visualMove.position;
    } else {
      const origin = hexWorld(visualMove.from);
      if (rt.ranger.root.position.distanceTo(new THREE.Vector3(origin.x, 0, origin.z)) > .2) {
        rt.movement = null;
        rt.lastMoveId = visualMove.commandId;
        if (snapshot.player) {
          const confirmed = hexWorld(snapshot.player.position);
          rt.ranger.snapTo({ x: confirmed.x, y: confirmed.z });
        }
        return;
      }
      rt.movement = { commandId: visualMove.commandId, position: visualMove.position, points, index: 0 };
      rt.ranger.setSpec({ position: { x: points[0].x, y: points[0].z } }, performance.now() * .001);
    }
    rt.lastMoveId = visualMove.commandId;
    rt.invalidate();
  }, [visualMove, connection, snapshot.instanceId]);
  useEffect(() => {
    const rt = runtime.current;
    const player = snapshot.player;
    if (!rt?.ranger || !player || player.characterType.toLowerCase() !== "ranger") return;
    const cues: RangerCue[] = [];
    for (const notice of visualNotices) {
      if (notice.kind !== "attack") continue;
      if (notice.stateRevision < (visualNotices[visualNotices.length - 1]?.stateRevision ?? notice.stateRevision) - 1) continue;
      const steps = notice.steps ?? [];
      if (player.isRanged && steps.some(step => step.source?.type === "player" && step.source.id === player.userId)) {
        const target = notice.targetPosition && hexWorld(notice.targetPosition);
        cues.push({ id: `${notice.commandId}:bow`, clip: "Attack_Bow", target: target ? { x: target.x, y: target.z } : undefined });
      }
      for (const [index, step] of steps.entries()) {
        if (step.target.type !== "player" || step.target.id !== player.userId) continue;
        if (step.kind === "death") cues.push({ id: `${notice.commandId}:death:${index}`, clip: "Death" });
        else if ((step.damage ?? 0) > 0) cues.push({ id: `${notice.commandId}:hit:${index}`, clip: "Hit" });
      }
    }
    if (cues.length) {
      const position = rt.movement?.points[rt.movement.index] ?? hexWorld(player.position);
      rt.ranger.setSpec({ position: { x: position.x, y: position.z }, cues, defeated: player.health <= 0 }, performance.now() * .001);
      rt.invalidate();
    }
  }, [visualNotices, snapshot.player]);
  const initialized = useRef(false);
  useEffect(() => {
    const rt = runtime.current;
    if (rt) populateMarks(rt, snapshot, selected, route, selectedTargetId, selectedTargetKind, indicators);
  }, [snapshot.cells, snapshot.actors, selected, route, selectedTargetId, selectedTargetKind, indicators]);

  const cameraAction = (action: "in" | "out" | "left" | "right" | "focus") => {
    const rt = runtime.current;
    if (!rt) return;
    if (action === "in") rt.distance = clamp(rt.distance / 1.2, 7, 80);
    if (action === "out") rt.distance = clamp(rt.distance * 1.2, 7, 80);
    if (action === "left") rt.yaw -= Math.PI / 12;
    if (action === "right") rt.yaw += Math.PI / 12;
    if (action === "focus" && snapshot.player) {
      const world = hexWorld(snapshot.player.position); rt.focus.set(world.x, 0, world.z);
    }
    pointCamera(rt);
  };

  return <div className={styles.viewport} data-testid="hex-game-map-3d">
    <div ref={host} className={styles.canvas} aria-label={labels?.map ?? "3D HEX dungeon map"} />
    <div className={styles.controls} aria-label={labels?.camera ?? "Map camera"}>
      <button type="button" onClick={() => cameraAction("left")} aria-label="Rotate left">↶</button>
      <button type="button" onClick={() => cameraAction("in")} aria-label={labels?.zoomIn ?? "Zoom in"}>+</button>
      <button type="button" onClick={() => cameraAction("focus")}>{labels?.recenter ?? "To hero"}</button>
      <button type="button" onClick={() => cameraAction("out")} aria-label={labels?.zoomOut ?? "Zoom out"}>−</button>
      <button type="button" onClick={() => cameraAction("right")} aria-label="Rotate right">↷</button>
    </div>
  </div>;
}
