"use client";

import { useEffect, useRef } from "react";
import * as THREE from "three";
import type { Cell } from "@/types/GameTypes";
import { getCellIndex } from "@/utils/fogOfWar";
import { TILE, type RenderObject } from "@/utils/realMapAdapter";
import { animateFire, disposeAtmosphere, makeAtmosphereContext, makeColumn, makeRubble, makeWallTorch } from "./catacombsAtmosphere";
import { cellHash, selectColumnCells } from "./columnPlacement";
import { createWallOcclusionSampler, type OcclusionTarget } from "./wallOcclusion";
import { AnimatedRanger, type RangerSpec, type RangerStatus } from "./AnimatedRanger";
import { CombatOverlay3D } from "./CombatOverlay3D";
import type { ActiveEffect } from "@/types/combat";

type Position = { x: number; y: number };
type Props = {
  grid: Cell[];
  mapWidth: number;
  mapHeight: number;
  heroPosition: Position;
  cameraFocus?: Position | null;
  visibleCells: ReadonlySet<number>;
  exploredCells: ReadonlySet<number>;
  materialSeed?: number;
  showTacticalGrid?: boolean;
  onCellClick?: (cell: Cell) => void;
  onMove?: (dx: number, dy: number) => void;
  onPlayerClick?: (userId: number) => void;
  objects?: RenderObject[];
  renderAllTerrain?: boolean;
  preloadTerrain?: boolean;
  ranger?: RangerSpec;
  onRangerStatus?: (status: RangerStatus) => void;
  combatEffects?: readonly ActiveEffect[];
};

type CellMeshes = {
  floor: THREE.Mesh;
  floorVariant: number;
  wall?: WallModule;
  staticDetails: THREE.Mesh[];
  details: THREE.Object3D[];
};

type WallPart = { mesh: THREE.Mesh; trim: boolean; dark?: boolean; castsShadow?: boolean };
type WallModule = {
  x: number;
  y: number;
  body: THREE.Mesh;
  parts: WallPart[];
  decor: WallPart[];
  mask: number;
  opacity: number;
  materialVariant: number;
  column?: boolean;
};

const WALL_FADE_STEPS = 32;
const NORTH = 1, EAST = 2, SOUTH = 4, WEST = 8;

function wallShape(mask: number) {
  const degree = [NORTH, EAST, SOUTH, WEST].filter(bit => mask & bit).length;
  if (degree === 0) return "isolated";
  if (degree === 1) return "end";
  if (degree === 3) return "tee";
  if (degree === 4) return "cross";
  return mask === (NORTH | SOUTH) || mask === (EAST | WEST) ? "straight" : "corner";
}

// Keep the UVs aligned between neighboring boxes. Top faces use the map plane;
// vertical faces use distance along the wall and height above the floor.
function makeWorldUvBox(width: number, height: number, depth: number, x: number, y: number, z: number) {
  const geometry = new THREE.BoxGeometry(width, height, depth);
  const positions = geometry.getAttribute("position");
  const normals = geometry.getAttribute("normal");
  const uv = geometry.getAttribute("uv");
  for (let i = 0; i < uv.count; i++) {
    const worldX = x + positions.getX(i);
    const worldY = y + positions.getY(i);
    const worldZ = z + positions.getZ(i);
    if (Math.abs(normals.getY(i)) > 0.5) uv.setXY(i, worldX, worldZ);
    else if (Math.abs(normals.getX(i)) > 0.5) uv.setXY(i, worldZ, worldY);
    else uv.setXY(i, worldX, worldY);
  }
  uv.needsUpdate = true;
  return geometry;
}

function makeExploredMaterial(source: THREE.MeshStandardMaterial) {
  const material = source.clone();
  material.onBeforeCompile = (shader, renderer) => {
    source.onBeforeCompile(shader, renderer);
    shader.fragmentShader = shader.fragmentShader.replace(
      "#include <color_fragment>",
      `#include <color_fragment>
       float exploredLuma = dot(diffuseColor.rgb, vec3(0.2126, 0.7152, 0.0722));
       diffuseColor.rgb = mix(vec3(exploredLuma), diffuseColor.rgb, 0.35) * 0.55;`,
    );
  };
  material.customProgramCacheKey = () => `${source.customProgramCacheKey()}-explored-v2`;
  return material;
}

// Rare variants fade to the main image at tile boundaries. Every image uses the
// same world UVs, so a variant cannot cut a visible line through a wall run.
function makeVariantMaterial(source: THREE.MeshStandardMaterial, variant: THREE.Texture, floor: boolean) {
  const material = source.clone();
  material.onBeforeCompile = shader => {
    shader.uniforms.variantMap = { value: variant };
    shader.vertexShader = `varying vec2 vWorldUv;\n${shader.vertexShader}`.replace(
      "#include <uv_vertex>", "#include <uv_vertex>\n vWorldUv = uv;",
    );
    shader.fragmentShader = `uniform sampler2D variantMap;\nvarying vec2 vWorldUv;\n${shader.fragmentShader}`.replace(
      "#include <map_fragment>",
      `#ifdef USE_MAP
         vec4 baseStone = texture2D(map, vMapUv);
         vec4 variantStone = texture2D(variantMap, vMapUv);
         float localX = fract(vWorldUv.x + 0.5);
         float edgeDistance = min(localX, 1.0 - localX);
         ${floor ? "float localY = fract(vWorldUv.y + 0.5); edgeDistance = min(edgeDistance, min(localY, 1.0 - localY));" : ""}
         float variantWeight = smoothstep(0.02, 0.19, edgeDistance);
         diffuseColor *= mix(baseStone, variantStone, variantWeight);
       #endif`,
    );
  };
  material.customProgramCacheKey = () => `catacombs-variant-${floor ? "floor" : "wall"}-v1`;
  return material;
}

// Decor uses shared box geometry. Projecting from world position keeps stone
// courses continuous across adjacent plinths, cornices and corner pieces.
function makeTrimMaterial(texture: THREE.Texture) {
  const material = new THREE.MeshStandardMaterial({ map: texture, roughness: 0.95 });
  material.onBeforeCompile = shader => {
    shader.vertexShader = `varying vec3 vTrimPosition;\nvarying vec3 vTrimNormal;\n${shader.vertexShader}`.replace(
      "#include <begin_vertex>",
      `#include <begin_vertex>
       vTrimPosition = (modelMatrix * vec4(transformed, 1.0)).xyz;
       vTrimNormal = normalize(mat3(modelMatrix) * normal);`,
    );
    shader.fragmentShader = `varying vec3 vTrimPosition;\nvarying vec3 vTrimNormal;\n${shader.fragmentShader}`.replace(
      "#include <map_fragment>",
      `#ifdef USE_MAP
         vec3 axis = abs(vTrimNormal);
         vec2 stoneUv = axis.y > axis.x && axis.y > axis.z ? vTrimPosition.xz
           : axis.x > axis.z ? vTrimPosition.zy : vTrimPosition.xy;
         diffuseColor *= texture2D(map, stoneUv * vec2(0.25, 0.5));
       #endif`,
    );
  };
  material.customProgramCacheKey = () => "catacombs-world-trim-v1";
  return material;
}

/** Rendering only: the caller owns movement rules and visibility. */
export default function NachaloScene3D({
  grid, mapWidth, mapHeight, heroPosition, cameraFocus, visibleCells, exploredCells, materialSeed = 271828,
  showTacticalGrid = false, onCellClick, onMove, onPlayerClick, objects, renderAllTerrain = false, preloadTerrain = false,
  ranger, onRangerStatus, combatEffects,
}: Props) {
  const mountRef = useRef<HTMLDivElement>(null);
  const callbacksRef = useRef({ onCellClick, onMove, onPlayerClick, onRangerStatus });
  const syncObjectsRef = useRef<((next: RenderObject[] | undefined) => void) | null>(null);
  const fogRef = useRef({ visibleCells, exploredCells });
  const heroTargetRef = useRef(heroPosition);
  const cameraFocusRef = useRef(cameraFocus);
  const tacticalGridRef = useRef(showTacticalGrid);
  const rangerSpecRef = useRef(ranger);
  const objectsRef = useRef(objects);
  const combatEffectsRef = useRef(combatEffects);
  combatEffectsRef.current = combatEffects;
  const updateFogRef = useRef<(() => void) | null>(null);
  callbacksRef.current = { onCellClick, onMove, onPlayerClick, onRangerStatus };
  fogRef.current = { visibleCells, exploredCells };
  heroTargetRef.current = heroPosition;
  cameraFocusRef.current = cameraFocus;
  tacticalGridRef.current = showTacticalGrid;
  rangerSpecRef.current = ranger;
  objectsRef.current = objects;

  useEffect(() => {
    updateFogRef.current?.();
  }, [visibleCells, exploredCells, showTacticalGrid]);

  useEffect(() => {
    syncObjectsRef.current?.(objects);
  }, [objects]);

  useEffect(() => {
    const host = mountRef.current;
    if (!host || mapWidth <= 0 || mapHeight <= 0) return;

    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x101722);
    const camera = new THREE.OrthographicCamera();
    const initialFocus = cameraFocusRef.current ?? heroTargetRef.current;
    const cameraTarget = new THREE.Vector3(initialFocus.x, 0, initialFocus.y);
    const cameraOffset = new THREE.Vector3(7, 12, 9);
    camera.position.copy(cameraTarget).add(cameraOffset);
    camera.lookAt(cameraTarget);

    const renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.domElement.style.display = "block";
    renderer.domElement.style.width = "100%";
    renderer.domElement.style.height = "100%";
    host.appendChild(renderer.domElement);
    const combatOverlay = new CombatOverlay3D(host);

    camera.layers.enable(1);
    const ambient = new THREE.HemisphereLight(0x9eb7d4, 0x303b43, 1.55);
    ambient.layers.enable(1);
    scene.add(ambient);
    const sun = new THREE.DirectionalLight(0xd3dfeb, 1.5);
    sun.layers.enable(1);
    sun.position.set(-7, 16, 9);
    sun.castShadow = true;
    sun.shadow.mapSize.set(1024, 1024);
    sun.shadow.camera.left = sun.shadow.camera.bottom = -20;
    sun.shadow.camera.right = sun.shadow.camera.top = 20;
    scene.add(sun);

    const loader = new THREE.TextureLoader();
    const assetRoot = "/assets/nachalo/catacombs";
    const floorTextures = ["floor_base", "floor_cracked", "floor_moss", "floor_runes"]
      .map(name => loader.load(`${assetRoot}/${name}.webp`));
    const wallTextures = ["wall_base", "wall_cracked", "wall_moss"]
      .map(name => loader.load(`${assetRoot}/${name}.webp`));
    const trimTexture = loader.load(`${assetRoot}/stone_trim.webp`);
    const allTextures = [...floorTextures, ...wallTextures, trimTexture];
    for (const texture of allTextures) {
      texture.colorSpace = THREE.SRGBColorSpace;
      // Mirroring joins the image at its own edge even when the source file is
      // not perfectly tileable; world UVs keep neighboring geometry aligned.
      texture.wrapS = texture.wrapT = THREE.MirroredRepeatWrapping;
      texture.minFilter = THREE.LinearMipmapLinearFilter;
      texture.magFilter = THREE.LinearFilter;
      texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy());
      texture.generateMipmaps = true;
    }
    // One texture image covers four floor cells; wall courses span two units vertically.
    for (const texture of floorTextures) texture.repeat.set(0.25, 0.25);
    for (const texture of wallTextures) texture.repeat.set(0.25, 0.5);
    trimTexture.repeat.set(0.25, 0.5);

    const geometries = {
      wall: new THREE.BoxGeometry(0.88, 1.9, 0.88),
      plinthX: new THREE.BoxGeometry(0.10, 0.22, 1),
      plinthZ: new THREE.BoxGeometry(1, 0.22, 0.10),
      corniceX: new THREE.BoxGeometry(0.13, 0.12, 1),
      corniceZ: new THREE.BoxGeometry(1, 0.12, 0.13),
      bevelX: new THREE.BoxGeometry(0.06, 0.055, 1),
      bevelZ: new THREE.BoxGeometry(1, 0.055, 0.06),
      pier: new THREE.BoxGeometry(0.14, 1.46, 0.14),
      endStoneX: new THREE.BoxGeometry(0.10, 1.2, 0.42),
      endStoneZ: new THREE.BoxGeometry(0.42, 1.2, 0.10),
      boss: new THREE.BoxGeometry(0.38, 0.08, 0.38),
      chip: new THREE.BoxGeometry(0.18, 0.025, 0.12),
      barrel: new THREE.CylinderGeometry(0.28, 0.33, 0.6, 9),
      orb: new THREE.IcosahedronGeometry(0.27, 1),
      portal: new THREE.TorusGeometry(0.37, 0.075, 10, 28),
      marker: new THREE.TorusGeometry(0.43, 0.045, 8, 32),
      boots: new THREE.CylinderGeometry(0.22, 0.26, 0.18, 10),
      body: new THREE.CylinderGeometry(0.21, 0.28, 0.65, 10),
      head: new THREE.SphereGeometry(0.19, 12, 8),
      helm: new THREE.ConeGeometry(0.26, 0.32, 10),
    };
    const materials = {
      floorTop: new THREE.MeshStandardMaterial({ map: floorTextures[0], roughness: 0.95 }),
      floorEdge: new THREE.MeshStandardMaterial({ color: 0x3b4243, roughness: 1 }),
      floorBottom: new THREE.MeshStandardMaterial({ color: 0x262b30, roughness: 1 }),
      wallSide: new THREE.MeshStandardMaterial({ map: wallTextures[0], roughness: 0.95 }),
      wallTop: new THREE.MeshStandardMaterial({ map: floorTextures[0], color: 0xb8bfbc, roughness: 0.95 }),
      wallBottom: new THREE.MeshStandardMaterial({ color: 0x303638, roughness: 1 }),
      trim: makeTrimMaterial(trimTexture),
      water: new THREE.MeshStandardMaterial({ color: 0x236b88, roughness: 0.35, metalness: 0.15 }),
      start: new THREE.MeshStandardMaterial({ map: floorTextures[0], color: 0x5487b8, roughness: 0.95 }),
      barrel: new THREE.MeshStandardMaterial({ color: 0x8d5735, roughness: 0.85 }),
      portal: new THREE.MeshStandardMaterial({ color: 0x39e5bd, emissive: 0x24a889, emissiveIntensity: 1.8 }),
      monster: new THREE.MeshStandardMaterial({ color: 0xc55d53, roughness: 0.8 }),
      resource: new THREE.MeshStandardMaterial({ color: 0xedc66e, metalness: 0.35, roughness: 0.35 }),
      hero: new THREE.MeshStandardMaterial({ color: 0x3d83cf, roughness: 0.75 }),
      skin: new THREE.MeshStandardMaterial({ color: 0xeac09a, roughness: 0.85 }),
      helm: new THREE.MeshStandardMaterial({ color: 0xe9d773, metalness: 0.25, roughness: 0.55 }),
      position: new THREE.MeshBasicMaterial({ color: 0x68ebff }),
      selection: new THREE.MeshBasicMaterial({ color: 0xffd16b }),
    };
    const floorTopMaterials = [materials.floorTop, ...floorTextures.slice(1).map(texture =>
      makeVariantMaterial(materials.floorTop, texture, true))];
    const wallSideMaterials = [materials.wallSide, ...wallTextures.slice(1).map(texture =>
      makeVariantMaterial(materials.wallSide, texture, false))];
    const floorMaterials = floorTopMaterials.map(top =>
      [materials.floorEdge, materials.floorEdge, top,
        materials.floorBottom, materials.floorEdge, materials.floorEdge]);
    const wallMaterials = wallSideMaterials.map(side =>
      [side, side, materials.wallTop, materials.wallBottom, side, side]);
    const exploredMaterials = {
      floorTop: makeExploredMaterial(materials.floorTop),
      floorEdge: makeExploredMaterial(materials.floorEdge),
      floorBottom: makeExploredMaterial(materials.floorBottom),
      wallSide: makeExploredMaterial(materials.wallSide),
      wallTop: makeExploredMaterial(materials.wallTop),
      wallBottom: makeExploredMaterial(materials.wallBottom),
      trim: makeExploredMaterial(materials.trim),
      water: makeExploredMaterial(materials.water),
      start: makeExploredMaterial(materials.start),
    };
    const exploredFloorTopMaterials = [exploredMaterials.floorTop,
      ...floorTopMaterials.slice(1).map(makeExploredMaterial)];
    const exploredWallSideMaterials = [exploredMaterials.wallSide,
      ...wallSideMaterials.slice(1).map(makeExploredMaterial)];
    const exploredFloorMaterials = exploredFloorTopMaterials.map(top =>
      [exploredMaterials.floorEdge, exploredMaterials.floorEdge, top,
        exploredMaterials.floorBottom, exploredMaterials.floorEdge, exploredMaterials.floorEdge]);
    const exploredWallMaterials = exploredWallSideMaterials.map(side =>
      [side, side, exploredMaterials.wallTop, exploredMaterials.wallBottom, side, side]);
    const fadedWallMaterials = wallSideMaterials.map(sideSource => Array.from({ length: WALL_FADE_STEPS }, (_, step) => {
      const opacity = 1 - 0.8 * (step + 1) / WALL_FADE_STEPS;
      const side = sideSource.clone();
      side.onBeforeCompile = sideSource.onBeforeCompile;
      side.customProgramCacheKey = sideSource.customProgramCacheKey;
      const top = materials.wallTop.clone();
      const bottom = materials.wallBottom.clone();
      for (const material of [side, top, bottom]) {
        material.transparent = true;
        material.depthWrite = false;
        material.opacity = opacity;
      }
      return [side, side, top, bottom, side, side];
    }));
    const fadedTrimMaterials = Array.from({ length: WALL_FADE_STEPS }, (_, step) => {
      const material = materials.trim.clone();
      material.onBeforeCompile = materials.trim.onBeforeCompile;
      material.customProgramCacheKey = materials.trim.customProgramCacheKey;
      material.transparent = true;
      material.depthWrite = false;
      material.opacity = 1 - 0.8 * (step + 1) / WALL_FADE_STEPS;
      return material;
    });
    const fadedDarkMaterials = Array.from({ length: WALL_FADE_STEPS }, (_, step) => {
      const material = materials.floorEdge.clone();
      material.transparent = true;
      material.depthWrite = false;
      material.opacity = 1 - 0.8 * (step + 1) / WALL_FADE_STEPS;
      return material;
    });
    const texturedGeometries: THREE.BufferGeometry[] = [];
    const cells = new Map<number, Cell>();
    const meshes = new Map<number, CellMeshes>();
    const visibleAtMount = new Set([...fogRef.current.visibleCells, ...fogRef.current.exploredCells]);
    const initiallyKnown = (renderAllTerrain || preloadTerrain)
      ? new Set(grid.map(cell => getCellIndex(cell.x, cell.y, mapWidth))) : visibleAtMount;
    const wallCells = new Set(grid.filter(cell => initiallyKnown.has(getCellIndex(cell.x, cell.y, mapWidth)) &&
      (cell.tileCode === TILE.obstacle || cell.tileCode === TILE.border))
      .map(cell => getCellIndex(cell.x, cell.y, mapWidth)));
    const columnCells = selectColumnCells(grid, mapWidth, mapHeight, visibleAtMount, materialSeed);
    const floorTargets: THREE.Object3D[] = [];
    const wallTargets: THREE.Mesh[] = [];
    const wallModules = new Map<number, WallModule>();
    const portals: THREE.Mesh[] = [];
    const atmosphere = makeAtmosphereContext(materials.trim, materials.floorEdge);
    const fires: ReturnType<typeof makeWallTorch>[] = [];
    const props: Array<{ group: THREE.Group; cellIndex: number }> = [];
    const gridMaterial = new THREE.LineBasicMaterial({ color: 0x8bb9c4, transparent: true, opacity: 0.55 });
    const gridOverlay = new THREE.LineSegments(new THREE.BufferGeometry(), gridMaterial);
    gridOverlay.visible = false;
    scene.add(gridOverlay);
    const addMesh = (geometry: THREE.BufferGeometry, material: THREE.Material | THREE.Material[], x: number, y: number, z: number) => {
      const mesh = new THREE.Mesh(geometry, material);
      mesh.position.set(x, y, z);
      mesh.castShadow = true;
      mesh.receiveShadow = true;
      scene.add(mesh);
      return mesh;
    };
    const addWallPart = (wall: WallModule, geometry: THREE.BufferGeometry, x: number, y: number, z: number,
      trim: boolean, decor = false) => {
      const mesh = addMesh(geometry, trim ? materials.trim : wallMaterials[wall.materialVariant], x, y, z);
      const part = { mesh, trim };
      (decor ? wall.decor : wall.parts).push(part);
      return mesh;
    };

    for (const cell of grid) {
      const index = getCellIndex(cell.x, cell.y, mapWidth);
      if (!initiallyKnown.has(index)) continue;
      const variation = cellHash(cell.x, cell.y, materialSeed) % 37;
      const floorVariant = variation < 3 ? variation + 1 : 0;
      const wallVariation = cellHash(cell.x + 113, cell.y - 71, materialSeed) % 23;
      const wallVariant = wallVariation < 2 ? wallVariation + 1 : 0;
      cells.set(index, cell);
      // Exact cell width and world UVs remove the dark gaps without changing tile coordinates.
      const floorGeometry = makeWorldUvBox(1, 0.22, 1, cell.x, -0.12, cell.y);
      texturedGeometries.push(floorGeometry);
      const floor = addMesh(floorGeometry, cell.tileCode === TILE.portal ? materials.water :
        cell.tileCode === TILE.start ? materials.start : floorMaterials[floorVariant],
        cell.x, -0.12, cell.y);
      floor.userData.cellIndex = index;
      floorTargets.push(floor);
      const entry: CellMeshes = { floor, floorVariant, staticDetails: [], details: [] };
      if (wallCells.has(index)) {
        if (columnCells.has(index)) {
          const { group, shaft } = makeColumn(atmosphere, cell.x, cell.y,
            cellHash(cell.x, cell.y + 103, materialSeed));
          scene.add(group);
          const parts: WallPart[] = [];
          group.traverse(object => {
            if (object instanceof THREE.Mesh) parts.push({ mesh: object,
              trim: object.material === materials.trim,
              dark: object.material === materials.floorEdge,
              castsShadow: object.castShadow });
          });
          entry.wall = { x: cell.x, y: cell.y, body: shaft, parts,
            decor: [], mask: 0, opacity: 1, materialVariant: 0, column: true };
        } else {
          const bodyGeometry = makeWorldUvBox(1, 1.72, 1, cell.x, 0.85, cell.y);
          const lowerCapGeometry = makeWorldUvBox(0.94, 0.16, 0.94, cell.x, 1.75, cell.y);
          const upperCapGeometry = makeWorldUvBox(1, 0.08, 1, cell.x, 1.86, cell.y);
          texturedGeometries.push(bodyGeometry, lowerCapGeometry, upperCapGeometry);
          const body = addMesh(bodyGeometry, wallMaterials[wallVariant], cell.x, 0.85, cell.y);
          const wall: WallModule = {
            x: cell.x, y: cell.y, body, parts: [{ mesh: body, trim: false }],
            decor: [], mask: -1, opacity: 1, materialVariant: wallVariant,
          };
          addWallPart(wall, lowerCapGeometry, cell.x, 1.75, cell.y, false);
          addWallPart(wall, upperCapGeometry, cell.x, 1.86, cell.y, false);
          entry.wall = wall;
        }
        entry.wall.body.userData.cellIndex = index;
        wallTargets.push(entry.wall.body);
        wallModules.set(index, entry.wall);
      } else if (cell.tileCode !== 112 && variation === 2) {
        const chip = addMesh(geometries.chip, materials.floorEdge,
          cell.x + ((cellHash(cell.x, cell.y + 9, materialSeed) % 5) - 2) * 0.08,
          0.012,
          cell.y + ((cellHash(cell.x + 9, cell.y, materialSeed) % 5) - 2) * 0.08);
        chip.castShadow = false;
        entry.staticDetails.push(chip);
      }
      // These can change during play, so never display them outside current visibility.
      if (!objects && cell.barbel) entry.details.push(addMesh(geometries.barrel, materials.barrel, cell.x, 0.31, cell.y));
      if (!objects && cell.resource) entry.details.push(addMesh(geometries.orb, materials.resource, cell.x, 0.45, cell.y));
      if (!objects && cell.monster) entry.details.push(addMesh(geometries.orb, materials.monster, cell.x, 0.62, cell.y));
      if (!objects && cell.structure_type) entry.details.push(addMesh(geometries.wall, materials.barrel, cell.x, 0.94, cell.y));
      if (!objects && cell.isPortal) {
        const portal = addMesh(geometries.portal, materials.portal, cell.x, 0.75, cell.y);
        portal.rotation.x = -Math.PI / 2;
        portals.push(portal);
        entry.details.push(portal);
      }
      meshes.set(index, entry);
    }

    // Dynamic objects use their existing art until a dedicated model is ready.
    const spriteGeometry = new THREE.PlaneGeometry(0.82, 0.82);
    const spriteTextures = new Map<string, THREE.Texture>();
    const spriteMaterials: THREE.Material[] = [];
    const playerTargets: THREE.Mesh[] = [];
    let animatedRanger: AnimatedRanger | null = null;
    const occlusionTargets: OcclusionTarget[] = [];
    let occlusionDirty = true;
    const syncObjects = (next: RenderObject[] | undefined) => {
      if (!next) return;
      for (const entry of meshes.values()) {
        for (const detail of entry.details) scene.remove(detail);
        entry.details = [];
      }
      for (const material of spriteMaterials) material.dispose();
      spriteMaterials.length = 0;
      playerTargets.length = 0;
      portals.length = 0;
      occlusionTargets.length = 0;
      occlusionDirty = true;
      for (const object of next) {
        if (object.kind === "player" && animatedRanger?.isReady &&
            object.userId === rangerSpecRef.current?.userId) continue;
        const index = getCellIndex(object.x, object.y, mapWidth);
        const entry = meshes.get(index);
        if (!entry) continue;
        const rawImage = object.image?.trim();
        const image = rawImage && !rawImage.startsWith("//") && !rawImage.includes("..") &&
          !rawImage.includes(":") && !rawImage.includes("\\")
          ? (rawImage.startsWith("/") ? rawImage : `/${rawImage}`) : undefined;
        let mesh: THREE.Mesh;
        if (image) {
          let texture = spriteTextures.get(image);
          if (!texture) {
            texture = loader.load(image);
            texture.colorSpace = THREE.SRGBColorSpace;
            spriteTextures.set(image, texture);
          }
          const material = new THREE.MeshBasicMaterial({ map: texture, transparent: true, side: THREE.DoubleSide,
            depthWrite: false, alphaTest: 0.05 });
          spriteMaterials.push(material);
          mesh = addMesh(spriteGeometry, material, object.x, 0.53, object.y);
          mesh.rotation.y = Math.PI / 4;
          mesh.castShadow = false;
        } else {
          const kind = object.kind;
          const geometry = kind === "barrel" ? geometries.barrel :
            kind === "portal" ? geometries.portal : kind === "player" ? geometries.body : geometries.orb;
          const material = kind === "portal" ? materials.portal : kind === "monster" ? materials.monster :
            kind === "resource" ? materials.resource : kind === "player" ? materials.hero : materials.barrel;
          mesh = addMesh(geometry, material, object.x, 0.5, object.y);
          if (kind === "portal") { mesh.rotation.x = -Math.PI / 2; portals.push(mesh); }
        }
        mesh.userData.cellIndex = index;
        occlusionTargets.push({ x: object.x, z: object.y, cellIndex: index,
          top: object.kind === "player" || object.kind === "monster" ? 1.05 : 0.95,
          halfWidth: object.kind === "player" ? 0.42 : 0.38 });
        if (object.kind === "player" && object.userId !== undefined) {
          mesh.userData.playerId = object.userId;
          playerTargets.push(mesh);
        }
        entry.details.push(mesh);
      }
      updateFogRef.current?.();
    };
    syncObjectsRef.current = syncObjects;
    syncObjects(objects);

    const torchSites = grid.filter(cell => visibleAtMount.has(getCellIndex(cell.x, cell.y, mapWidth)) &&
      wallCells.has(getCellIndex(cell.x, cell.y, mapWidth)) &&
      !columnCells.has(getCellIndex(cell.x, cell.y, mapWidth)))
      .flatMap(cell => ([
        [0, 1, 0], [0, -1, Math.PI], [1, 0, Math.PI / 2], [-1, 0, -Math.PI / 2],
      ] as const).filter(([dx, dz]) => {
        if (cell.x + dx < 0 || cell.x + dx >= mapWidth ||
          cell.y + dz < 0 || cell.y + dz >= mapHeight) return false;
        const neighbor = cells.get(getCellIndex(cell.x + dx, cell.y + dz, mapWidth));
        return neighbor && !wallCells.has(getCellIndex(neighbor.x, neighbor.y, mapWidth)) && neighbor.tileCode !== 112;
      }).map(([, , rotation]) => ({ cell, rotation, hash: cellHash(cell.x, cell.y, materialSeed) })));
    // Anchor one torch near the current preview position, then scatter by seed.
    const firstTorch = [...torchSites].sort((a, b) =>
      (Math.abs(a.cell.x - heroTargetRef.current.x) + Math.abs(a.cell.y - heroTargetRef.current.y)) -
      (Math.abs(b.cell.x - heroTargetRef.current.x) + Math.abs(b.cell.y - heroTargetRef.current.y)) || a.hash - b.hash)[0];
    const chosenTorches = firstTorch ? [firstTorch] : [];
    for (const site of [...torchSites].sort((a, b) => a.hash - b.hash)) {
      if (chosenTorches.length >= Math.min(12, Math.max(2, Math.ceil(wallModules.size / 11)))) break;
      if (chosenTorches.some(other => Math.abs(other.cell.x - site.cell.x) +
        Math.abs(other.cell.y - site.cell.y) < 4)) continue;
      chosenTorches.push(site);
    }
    for (const { cell, rotation } of chosenTorches) {
      const torch = makeWallTorch(atmosphere, cell.x, cell.y, rotation,
        getCellIndex(cell.x, cell.y, mapWidth), cellHash(cell.x, cell.y, materialSeed) / 0xffffffff * Math.PI * 2);
      scene.add(torch.group);
      fires.push(torch);
    }

    const emptyFloor = grid.filter(cell => initiallyKnown.has(getCellIndex(cell.x, cell.y, mapWidth)) &&
      !wallCells.has(getCellIndex(cell.x, cell.y, mapWidth)) && cell.tileCode !== 112 &&
      cell.tileCode !== TILE.start &&
      (cell.x !== heroTargetRef.current.x || cell.y !== heroTargetRef.current.y) &&
      !cell.barbel && !cell.resource && !cell.monster && !cell.structure_type && !cell.isPortal &&
      !objects?.some(object => object.x === cell.x && object.y === cell.y));
    for (const cell of emptyFloor) {
      const index = getCellIndex(cell.x, cell.y, mapWidth);
      const hash = cellHash(cell.x + 211, cell.y - 29, materialSeed);
      if (hash % 17 !== 0) continue;
      const rubble = makeRubble(atmosphere, cell.x + ((hash >> 5) % 5 - 2) * 0.07,
        cell.y + ((hash >> 9) % 5 - 2) * 0.07, hash);
      scene.add(rubble);
      props.push({ group: rubble, cellIndex: index });
    }

    const hero = new THREE.Group();
    hero.position.set(heroTargetRef.current.x, 0, heroTargetRef.current.y);
    const addHeroPart = (geometry: THREE.BufferGeometry, material: THREE.Material, y: number) => {
      if (renderAllTerrain) return;
      const part = new THREE.Mesh(geometry, material);
      part.position.y = y;
      part.castShadow = true;
      hero.add(part);
    };
    // Floor top is -0.01; the boots rest directly on it.
    addHeroPart(geometries.boots, materials.helm, 0.08);
    addHeroPart(geometries.body, materials.hero, 0.48);
    addHeroPart(geometries.head, materials.skin, 0.98);
    addHeroPart(geometries.helm, materials.helm, 1.19);
    const positionRing = new THREE.Mesh(geometries.marker, materials.position);
    positionRing.rotation.x = -Math.PI / 2;
    positionRing.position.y = 0.035;
    hero.add(positionRing);
    scene.add(hero);
    const syncRanger = () => {
      const spec = rangerSpecRef.current;
      if (spec && animatedRanger?.model === (spec.model ?? "legacy") &&
          animatedRanger.resetKey === (spec.resetKey ?? 0)) return;
      if (!spec && !animatedRanger) return;
      const previous = animatedRanger;
      // Acquire the new asset before releasing the previous instance, so a
      // reset can reuse its template. No renderer, Canvas or RAF is recreated.
      animatedRanger = spec ? new AnimatedRanger({ ...spec, position: previous ? {
        x: previous.root.position.x, y: previous.root.position.z,
      } : spec.position }) : null;
      if (animatedRanger) {
        if (previous) animatedRanger.root.rotation.copy(previous.root.rotation);
        scene.add(animatedRanger.root);
        animatedRanger.onReady = () => {
          for (const part of hero.children) if (part !== positionRing) part.visible = false;
          syncObjects(objectsRef.current);
        };
      }
      previous?.dispose();
      for (const part of hero.children) if (part !== positionRing) part.visible = true;
      syncObjects(objectsRef.current);
    };
    syncRanger();

    const selectionRing = new THREE.Mesh(geometries.marker, materials.selection);
    selectionRing.rotation.x = -Math.PI / 2;
    selectionRing.position.y = 0.04;
    selectionRing.visible = false;
    scene.add(selectionRing);
    let selectedIndex: number | null = null;
    const knownWall = (x: number, y: number, known: ReadonlySet<number>) =>
      x >= 0 && y >= 0 && x < mapWidth && y < mapHeight &&
      known.has(getCellIndex(x, y, mapWidth)) && wallCells.has(getCellIndex(x, y, mapWidth)) &&
      !columnCells.has(getCellIndex(x, y, mapWidth));
    const knownOpen = (x: number, y: number, known: ReadonlySet<number>) =>
      x >= 0 && y >= 0 && x < mapWidth && y < mapHeight &&
      known.has(getCellIndex(x, y, mapWidth)) && !wallCells.has(getCellIndex(x, y, mapWidth));
    const wallMaskFor = (wall: WallModule, known: ReadonlySet<number>) =>
      (knownWall(wall.x, wall.y - 1, known) ? NORTH : 0) |
      (knownWall(wall.x + 1, wall.y, known) ? EAST : 0) |
      (knownWall(wall.x, wall.y + 1, known) ? SOUTH : 0) |
      (knownWall(wall.x - 1, wall.y, known) ? WEST : 0);
    const rebuildWallDecor = (wall: WallModule, mask: number, known: ReadonlySet<number>) => {
      for (const part of wall.decor) scene.remove(part.mesh);
      wall.decor = [];
      wall.mask = mask;
      if (mask < 0) return;
      const shape = wallShape(mask);
      wall.body.userData.wallShape = shape;
      wall.body.userData.wallMask = mask;
      const x = wall.x, z = wall.y;
      const addTrim = (geometry: THREE.BufferGeometry, px: number, py: number, pz: number) =>
        addWallPart(wall, geometry, px, py, pz, true, true);
      if (!(mask & NORTH)) {
        addTrim(geometries.plinthZ, x, 0.12, z - 0.45);
        addTrim(geometries.corniceZ, x, 1.59, z - 0.435);
        addTrim(geometries.bevelZ, x, 1.85, z - 0.47).rotation.x = -Math.PI / 4;
      }
      if (!(mask & SOUTH)) {
        addTrim(geometries.plinthZ, x, 0.12, z + 0.45);
        addTrim(geometries.corniceZ, x, 1.59, z + 0.435);
        addTrim(geometries.bevelZ, x, 1.85, z + 0.47).rotation.x = Math.PI / 4;
      }
      if (!(mask & EAST)) {
        addTrim(geometries.plinthX, x + 0.45, 0.12, z);
        addTrim(geometries.corniceX, x + 0.435, 1.59, z);
        addTrim(geometries.bevelX, x + 0.47, 1.85, z).rotation.z = -Math.PI / 4;
      }
      if (!(mask & WEST)) {
        addTrim(geometries.plinthX, x - 0.45, 0.12, z);
        addTrim(geometries.corniceX, x - 0.435, 1.59, z);
        addTrim(geometries.bevelX, x - 0.47, 1.85, z).rotation.z = Math.PI / 4;
      }
      const corners: Array<[number, number, number, number, number, number]> = [
        [NORTH, EAST, 1, -1, 1, -1], [SOUTH, EAST, 1, 1, 1, 1],
        [SOUTH, WEST, -1, 1, -1, 1], [NORTH, WEST, -1, -1, -1, -1],
      ];
      for (const [first, second, dx, dz, diagonalX, diagonalZ] of corners) {
        const firstJoined = !!(mask & first), secondJoined = !!(mask & second);
        if ((!firstJoined && !secondJoined) ||
          (firstJoined && secondJoined && knownOpen(x + diagonalX, z + diagonalZ, known))) {
          addTrim(geometries.pier, x + dx * 0.43, 0.81, z + dz * 0.43);
        }
      }
      if (shape === "end") {
        if (mask & SOUTH) addTrim(geometries.endStoneZ, x, 0.78, z - 0.45);
        if (mask & NORTH) addTrim(geometries.endStoneZ, x, 0.78, z + 0.45);
        if (mask & WEST) addTrim(geometries.endStoneX, x + 0.45, 0.78, z);
        if (mask & EAST) addTrim(geometries.endStoneX, x - 0.45, 0.78, z);
      }
      if (shape === "tee" || shape === "cross" || shape === "isolated") {
        addTrim(geometries.boss, x, 1.94, z);
      }
    };
    const applyWallVisual = (wall: WallModule, visible: boolean, explored: boolean) => {
      const fadeLevel = Math.min(WALL_FADE_STEPS - 1,
        Math.floor((1 - wall.opacity) / 0.8 * WALL_FADE_STEPS));
      const stoneMaterial = !visible ? exploredWallMaterials[wall.materialVariant]
        : wall.opacity < 0.995 ? fadedWallMaterials[wall.materialVariant][fadeLevel]
          : wallMaterials[wall.materialVariant];
      const trimMaterial = !visible ? exploredMaterials.trim
        : wall.opacity < 0.995 ? fadedTrimMaterials[fadeLevel] : materials.trim;
      const setPart = (part: WallPart) => {
        part.mesh.visible = explored;
        part.mesh.layers.set(visible ? 0 : 1);
        part.mesh.material = part.dark
          ? (!visible ? exploredMaterials.floorEdge : wall.opacity < 0.995 ? fadedDarkMaterials[fadeLevel] : materials.floorEdge)
          : part.trim ? trimMaterial : stoneMaterial;
        part.mesh.castShadow = visible && wall.opacity >= 0.995 && (part.castsShadow ?? true);
      };
      for (const part of wall.parts) setPart(part);
      for (const part of wall.decor) setPart(part);
    };
    const updateFog = () => {
      occlusionDirty = true;
      const { visibleCells, exploredCells } = fogRef.current;
      const known = new Set([...visibleCells, ...exploredCells]);
      for (const [index, entry] of meshes) {
        const visible = visibleCells.has(index);
        const explored = visible || exploredCells.has(index);
        const code = cells.get(index)?.tileCode;
        const isWater = code === TILE.portal;
        const isStart = code === TILE.start;
        entry.floor.visible = explored;
        entry.floor.layers.set(visible ? 0 : 1);
        entry.floor.material = visible
          ? (isWater ? materials.water : isStart ? materials.start : floorMaterials[entry.floorVariant])
          : (isWater ? exploredMaterials.water : isStart ? exploredMaterials.start : exploredFloorMaterials[entry.floorVariant]);
        for (const detail of entry.staticDetails) {
          detail.visible = explored;
          detail.layers.set(visible ? 0 : 1);
          detail.material = visible ? materials.floorEdge : exploredMaterials.floorEdge;
        }
        if (entry.wall) {
          if (entry.wall.column) {
            applyWallVisual(entry.wall, visible, explored);
            continue;
          }
          const mask = explored ? wallMaskFor(entry.wall, known) : -1;
          const diagonalKey = explored ?
            (knownOpen(entry.wall.x + 1, entry.wall.y - 1, known) ? 1 : 0) |
            (knownOpen(entry.wall.x + 1, entry.wall.y + 1, known) ? 2 : 0) |
            (knownOpen(entry.wall.x - 1, entry.wall.y + 1, known) ? 4 : 0) |
            (knownOpen(entry.wall.x - 1, entry.wall.y - 1, known) ? 8 : 0) : 0;
          const topologyKey = explored ? mask | (diagonalKey << 4) : -1;
          if (entry.wall.mask !== topologyKey) {
            rebuildWallDecor(entry.wall, mask, known);
            entry.wall.mask = topologyKey;
          }
          applyWallVisual(entry.wall, visible, explored);
        }
        for (const detail of entry.details) detail.visible = visible;
      }
      for (const prop of props) prop.group.visible = visibleCells.has(prop.cellIndex);
      const activeFires = fires.filter(fire => visibleCells.has(fire.cellIndex))
        .sort((a, b) => {
          const da = a.group.position.distanceToSquared(new THREE.Vector3(heroTargetRef.current.x, 0, heroTargetRef.current.y));
          const db = b.group.position.distanceToSquared(new THREE.Vector3(heroTargetRef.current.x, 0, heroTargetRef.current.y));
          return da - db;
        }).slice(0, 4);
      for (const fire of fires) {
        const visible = visibleCells.has(fire.cellIndex);
        fire.group.visible = visible;
        fire.light.visible = activeFires.includes(fire);
        for (const spark of fire.sparks) spark.visible = activeFires.includes(fire);
      }
      const lines: number[] = [];
      if (tacticalGridRef.current) {
        for (const [index, cell] of cells) {
          if (!visibleCells.has(index) || wallCells.has(index)) continue;
          const x = cell.x, z = cell.y, y = 0.015;
          lines.push(
            x - 0.5, y, z - 0.5, x + 0.5, y, z - 0.5,
            x + 0.5, y, z - 0.5, x + 0.5, y, z + 0.5,
            x + 0.5, y, z + 0.5, x - 0.5, y, z + 0.5,
            x - 0.5, y, z + 0.5, x - 0.5, y, z - 0.5,
          );
        }
      }
      const previousGridGeometry = gridOverlay.geometry;
      gridOverlay.geometry = new THREE.BufferGeometry();
      gridOverlay.geometry.setAttribute("position", new THREE.Float32BufferAttribute(lines, 3));
      gridOverlay.visible = lines.length > 0;
      previousGridGeometry.dispose();
      selectionRing.visible = selectedIndex !== null && visibleCells.has(selectedIndex);
    };
    updateFogRef.current = updateFog;
    updateFog();

    let viewHeight = 11;
    const resize = () => {
      const width = Math.max(1, host.clientWidth);
      const height = Math.max(1, host.clientHeight);
      renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
      renderer.setSize(width, height, false);
      const aspect = width / height;
      camera.left = -viewHeight * aspect / 2;
      camera.right = viewHeight * aspect / 2;
      camera.top = viewHeight / 2;
      camera.bottom = -viewHeight / 2;
      camera.updateProjectionMatrix();
    };
    resize();
    const resizeObserver = new ResizeObserver(resize);
    resizeObserver.observe(host);
    window.addEventListener("resize", resize);

    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      viewHeight = THREE.MathUtils.clamp(viewHeight * Math.exp(event.deltaY * 0.001), 6, 26);
      resize();
    };
    host.addEventListener("wheel", onWheel, { passive: false });
    const onKeyDown = (event: KeyboardEvent) => {
      const directions: Record<string, [number, number]> = {
        ArrowUp: [0, -1], ArrowDown: [0, 1], ArrowLeft: [-1, 0], ArrowRight: [1, 0],
      };
      const direction = directions[event.key];
      if (!direction) return;
      event.preventDefault();
      callbacksRef.current.onMove?.(...direction);
    };
    host.addEventListener("keydown", onKeyDown);
    const onPointerDown = () => host.focus();
    host.addEventListener("pointerdown", onPointerDown);
    const raycaster = new THREE.Raycaster();
    const pointer = new THREE.Vector2();
    const sampleWallOcclusion = createWallOcclusionSampler();
    let blockingWalls = new Set<THREE.Mesh>();
    let lastOcclusionCheck = -Infinity;
    const onClick = (event: MouseEvent) => {
      const bounds = renderer.domElement.getBoundingClientRect();
      pointer.set(((event.clientX - bounds.left) / bounds.width) * 2 - 1,
        -((event.clientY - bounds.top) / bounds.height) * 2 + 1);
      raycaster.setFromCamera(pointer, camera);
      const rangerId = rangerSpecRef.current?.userId;
      if (animatedRanger?.isReady && rangerId !== undefined &&
          raycaster.intersectObject(animatedRanger.root, true).length > 0 &&
          fogRef.current.visibleCells.has(getCellIndex(
            Math.round(animatedRanger.root.position.x), Math.round(animatedRanger.root.position.z), mapWidth))) {
        callbacksRef.current.onPlayerClick?.(rangerId);
        return;
      }
      const playerHit = raycaster.intersectObjects(playerTargets.filter(target => target.visible), false)[0];
      if (playerHit && fogRef.current.visibleCells.has(playerHit.object.userData.cellIndex as number)) {
        callbacksRef.current.onPlayerClick?.(playerHit.object.userData.playerId as number);
        return;
      }
      const hit = raycaster.intersectObjects(floorTargets, false).find(item => {
        const index = item.object.userData.cellIndex as number;
        return fogRef.current.visibleCells.has(index);
      });
      const index = hit?.object.userData.cellIndex as number | undefined;
      if (index === undefined) return;
      selectedIndex = index;
      const cell = cells.get(index);
      if (!cell) return;
      selectionRing.position.set(cell.x, 0.04, cell.y);
      selectionRing.visible = true;
      callbacksRef.current.onCellClick?.(cell);
    };
    renderer.domElement.addEventListener("click", onClick);

    let lastFrame = performance.now();
    let lastRangerStatus = -Infinity;
    let frame = 0;
    const animate = (now: number) => {
      frame = requestAnimationFrame(animate);
      const delta = Math.min((now - lastFrame) / 1000, 0.05);
      lastFrame = now;
      const smoothing = 1 - Math.exp(-9 * delta);
      const target = heroTargetRef.current;
      hero.position.x = THREE.MathUtils.lerp(hero.position.x, target.x, smoothing);
      hero.position.z = THREE.MathUtils.lerp(hero.position.z, target.y, smoothing);
      const focus = cameraFocusRef.current;
      cameraTarget.set(focus?.x ?? hero.position.x, 0, focus?.y ?? hero.position.z);
      camera.position.copy(cameraTarget).add(cameraOffset);
      camera.lookAt(cameraTarget);
      // Recheck at 30 Hz while opacity still animates every frame. No terrain
      // geometry or fade material is created by this pass.
      if (occlusionDirty || now - lastOcclusionCheck >= 33) {
        const visibleWalls = wallTargets.filter(wall =>
          wall.visible && fogRef.current.visibleCells.has(wall.userData.cellIndex as number));
        const heroIndex = getCellIndex(Math.round(hero.position.x), Math.round(hero.position.z), mapWidth);
        const targets = [...occlusionTargets];
        if (fogRef.current.visibleCells.has(heroIndex)) targets.push({
          x: hero.position.x, z: hero.position.z, cellIndex: heroIndex,
          top: animatedRanger?.isReady ? 1.55 : renderAllTerrain ? 0.25 : 1.4, halfWidth: 0.43,
        });
        blockingWalls = sampleWallOcclusion(camera, visibleWalls, targets, fogRef.current.visibleCells);
        lastOcclusionCheck = now;
        occlusionDirty = false;
      }
      const wallSmoothing = 1 - Math.exp(-7 * delta);
      for (const [index, wall] of wallModules) {
        const visible = fogRef.current.visibleCells.has(index);
        if (!visible && !fogRef.current.exploredCells.has(index)) continue;
        const targetOpacity = visible && blockingWalls.has(wall.body) ? 0.2 : 1;
        wall.opacity = THREE.MathUtils.lerp(wall.opacity, targetOpacity, wallSmoothing);
        if (Math.abs(wall.opacity - targetOpacity) < 0.005) wall.opacity = targetOpacity;
        applyWallVisual(wall, visible, visible || fogRef.current.exploredCells.has(index));
      }
      for (const portal of portals) portal.rotation.z += delta * 0.65;
      const seconds = now * 0.001;
      syncRanger();
      if (animatedRanger && rangerSpecRef.current) {
        animatedRanger.setSpec(rangerSpecRef.current, seconds);
        animatedRanger.update(delta, seconds);
        if (animatedRanger.model === "animated" && animatedRanger.isReady) {
          hero.position.copy(animatedRanger.root.position);
        }
        if (now - lastRangerStatus >= 100) {
          callbacksRef.current.onRangerStatus?.(animatedRanger.status);
          lastRangerStatus = now;
        }
      }
      for (const fire of fires) {
        const wall = wallModules.get(fire.cellIndex);
        fire.group.visible = fogRef.current.visibleCells.has(fire.cellIndex) && (!wall || wall.opacity > 0.55);
        if (fire.group.visible) animateFire(fire, seconds);
      }
      camera.updateMatrixWorld();
      combatOverlay.update(combatEffectsRef.current ?? [], now, camera, host.clientWidth, host.clientHeight,
        point => fogRef.current.visibleCells.has(getCellIndex(point.x, point.y, mapWidth)) ||
          (rangerSpecRef.current?.defeated === true && point.x === rangerSpecRef.current.position.x &&
            point.y === rangerSpecRef.current.position.y),
        animatedRanger?.projectileOrigin && rangerSpecRef.current ? {
          cell: rangerSpecRef.current.position, point: animatedRanger.projectileOrigin!,
        } : undefined);
      renderer.render(scene, camera);
    };
    scene.updateMatrixWorld(true);
    animate(lastFrame);

    return () => {
      cancelAnimationFrame(frame);
      animatedRanger?.dispose();
      combatOverlay.dispose();
      updateFogRef.current = null;
      syncObjectsRef.current = null;
      renderer.domElement.removeEventListener("click", onClick);
      host.removeEventListener("wheel", onWheel);
      host.removeEventListener("keydown", onKeyDown);
      host.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("resize", resize);
      resizeObserver.disconnect();
      for (const geometry of Object.values(geometries)) geometry.dispose();
      gridOverlay.geometry.dispose();
      for (const geometry of texturedGeometries) geometry.dispose();
      disposeAtmosphere(atmosphere);
      gridMaterial.dispose();
      const ownedMaterials = new Set<THREE.Material>([
        ...Object.values(materials), ...floorTopMaterials, ...wallSideMaterials,
        ...Object.values(exploredMaterials), ...exploredFloorTopMaterials, ...exploredWallSideMaterials,
        ...fadedWallMaterials.flat(2), ...fadedTrimMaterials, ...fadedDarkMaterials,
      ]);
      for (const material of ownedMaterials) material.dispose();
      for (const texture of allTextures) texture.dispose();
      spriteGeometry.dispose();
      for (const texture of spriteTextures.values()) texture.dispose();
      for (const material of spriteMaterials) material.dispose();
      sun.shadow.map?.dispose();
      renderer.dispose();
      renderer.forceContextLoss();
      renderer.domElement.remove();
    };
  }, [grid, mapWidth, mapHeight, materialSeed, renderAllTerrain, preloadTerrain]);

  return <div ref={mountRef} tabIndex={0} role="application" aria-label="3D карта: стрелки для движения, колесо для масштаба"
    style={{ position: "relative", width: "100%", height: "min(74vh, 720px)", minHeight: 420, overflow: "hidden", borderRadius: 12, outlineColor: "#68ebff" }} />;
}
