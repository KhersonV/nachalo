import * as THREE from "three";

export type OcclusionTarget = {
  x: number;
  z: number;
  cellIndex: number;
  top: number;
  halfWidth: number;
};

/** Samples the screen silhouette with orthographic camera rays. Targets must
 * come from the visible render list; the visibility check is a second guard. */
export function createWallOcclusionSampler() {
  const ray = new THREE.Raycaster();
  const point = new THREE.Vector3();
  const projected = new THREE.Vector3();
  const screen = new THREE.Vector2();
  const right = new THREE.Vector3();
  const blockers = new Set<THREE.Mesh>();
  const samples: ReadonlyArray<readonly [number, number]> = [
    [0, 0.08], [0, 0.5], [0, 0.94], [-1, 0.5], [1, 0.5],
  ];

  return (camera: THREE.OrthographicCamera, walls: THREE.Mesh[],
    targets: readonly OcclusionTarget[], visibleCells: ReadonlySet<number>) => {
    blockers.clear();
    camera.updateMatrixWorld();
    right.setFromMatrixColumn(camera.matrixWorld, 0).setY(0).normalize();
    for (const target of targets) {
      if (!visibleCells.has(target.cellIndex)) continue;
      for (const [horizontal, height] of samples) {
        point.set(target.x + right.x * target.halfWidth * horizontal,
          target.top * height, target.z + right.z * target.halfWidth * horizontal);
        projected.copy(point).project(camera);
        if (Math.abs(projected.x) > 1 || Math.abs(projected.y) > 1 ||
          projected.z < -1 || projected.z > 1) continue;
        screen.set(projected.x, projected.y);
        ray.setFromCamera(screen, camera);
        ray.far = ray.ray.origin.distanceTo(point) - 0.06;
        if (ray.far <= 0) continue;
        for (const hit of ray.intersectObjects(walls, false)) {
          const wall = hit.object as THREE.Mesh;
          if (wall.visible && visibleCells.has(wall.userData.cellIndex as number)) blockers.add(wall);
        }
      }
    }
    return blockers;
  };
}
