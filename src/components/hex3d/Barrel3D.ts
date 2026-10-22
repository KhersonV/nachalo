import * as THREE from "three";

export type Barrel3D = { group: THREE.Group; lid: THREE.Group; hitMeshes: THREE.Mesh[] };

export function createBarrelAssets() {
  const profile = [
    new THREE.Vector2(.265, .025), new THREE.Vector2(.295, .16),
    new THREE.Vector2(.345, .42), new THREE.Vector2(.3, .68), new THREE.Vector2(.265, .82),
  ];
  const geometries = {
    core: new THREE.LatheGeometry(profile.map(p => new THREE.Vector2(p.x - .014, p.y)), 24),
    stave: new THREE.LatheGeometry(profile, 3, 0, Math.PI * 2 / 12 - .018),
    topHoop: new THREE.CylinderGeometry(.309, .317, .075, 24, 1, true),
    middleHoop: new THREE.CylinderGeometry(.354, .354, .078, 24, 1, true),
    bottomHoop: new THREE.CylinderGeometry(.293, .288, .067, 24, 1, true),
    rivet: new THREE.SphereGeometry(.013, 5, 4),
    lid: new THREE.CylinderGeometry(.263, .263, .035, 24),
    lidRim: new THREE.TorusGeometry(.265, .018, 5, 24),
    lidSeam: new THREE.BoxGeometry(.51, .003, .007),
    shadow: new THREE.CircleGeometry(.37, 24),
  };
  const materials = {
    core: new THREE.MeshStandardMaterial({ color: 0x24150f, roughness: .98 }),
    wood: [0x68402a, 0x74472b, 0x593621, 0x805035].map(color => new THREE.MeshStandardMaterial({ color, roughness: .9, metalness: 0 })),
    lid: new THREE.MeshStandardMaterial({ color: 0x67402a, roughness: .88 }),
    seam: new THREE.MeshStandardMaterial({ color: 0x2a1a13, roughness: .96 }),
    iron: new THREE.MeshStandardMaterial({ color: 0x373c3c, roughness: .53, metalness: .7 }),
    rivet: new THREE.MeshStandardMaterial({ color: 0x77706a, roughness: .45, metalness: .75 }),
    shadow: new THREE.MeshBasicMaterial({ color: 0x050707, transparent: true, opacity: .28, depthWrite: false }),
  };

  const create = (): Barrel3D => {
    const group = new THREE.Group();
    const hitMeshes: THREE.Mesh[] = [];
    const add = (parent: THREE.Group, geometry: THREE.BufferGeometry, material: THREE.Material, hit = true) => {
      const mesh = new THREE.Mesh(geometry, material);
      mesh.castShadow = hit;
      mesh.receiveShadow = hit;
      parent.add(mesh);
      if (hit) hitMeshes.push(mesh);
      return mesh;
    };
    const shadow = add(group, geometries.shadow, materials.shadow, false);
    shadow.rotation.x = -Math.PI / 2;
    shadow.position.y = .012;
    add(group, geometries.core, materials.core);
    for (let i = 0; i < 12; i++) {
      const stave = add(group, geometries.stave, materials.wood[(i * 7 + 1) % materials.wood.length]);
      stave.rotation.y = i * Math.PI * 2 / 12;
    }
    for (const [geometry, height, radius] of [
      [geometries.bottomHoop, .11, .294], [geometries.middleHoop, .42, .355], [geometries.topHoop, .69, .313],
    ] as const) {
      const hoop = add(group, geometry, materials.iron);
      hoop.position.y = height;
      for (let i = 0; i < 6; i++) {
        const angle = i * Math.PI / 3;
        const rivet = add(group, geometries.rivet, materials.rivet);
        rivet.position.set(Math.sin(angle) * radius, height, Math.cos(angle) * radius);
      }
    }
    const lid = new THREE.Group();
    lid.position.y = .835;
    group.add(lid);
    add(lid, geometries.lid, materials.lid).position.y = .017;
    const rim = add(lid, geometries.lidRim, materials.iron);
    rim.rotation.x = Math.PI / 2;
    rim.position.y = .036;
    for (const z of [-.095, .08]) {
      const seam = add(lid, geometries.lidSeam, materials.seam);
      seam.position.set(0, .036, z);
    }
    return { group, lid, hitMeshes };
  };

  const dispose = () => {
    Object.values(geometries).forEach(geometry => geometry.dispose());
    Object.values(materials).forEach(material => Array.isArray(material)
      ? material.forEach(item => item.dispose()) : material.dispose());
  };
  return { create, dispose };
}
