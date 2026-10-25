import * as THREE from "three";

/** Shared low-poly geometry/materials; no lights, shadows, textures or animation. */
export function createProfessionNodeAssets() {
  const stone = new THREE.DodecahedronGeometry(.24, 0);
  const stem = new THREE.CylinderGeometry(.035, .055, .45, 5);
  const box = new THREE.BoxGeometry(.23, .2, .22);
  const leaf = new THREE.ConeGeometry(.16, .4, 5);
  const materials = {
    rock: new THREE.MeshLambertMaterial({ color: 0x52616b }),
    metal: new THREE.MeshLambertMaterial({ color: 0x91b1cc, emissive: 0x182a39 }),
    wood: new THREE.MeshLambertMaterial({ color: 0x795335 }),
    bone: new THREE.MeshLambertMaterial({ color: 0xc8bd99 }),
    herb: new THREE.MeshLambertMaterial({ color: 0x5d9d82, emissive: 0x102b22 }),
    dark: new THREE.MeshLambertMaterial({ color: 0x242a30 }),
  };
  return {
    create(category: string) {
      const group = new THREE.Group();
      const add = (geometry: THREE.BufferGeometry, material: THREE.Material, x: number, y: number, z: number) => {
        const mesh = new THREE.Mesh(geometry, material); mesh.position.set(x, y, z); group.add(mesh); return mesh;
      };
      if (category === "ore") {
        for (let i = 0; i < 3; i++) {
          const rock = add(stone, materials.rock, (i - 1) * .18, .15 + i * .04, i % 2 * .15); rock.scale.set(1, 1 + i * .2, .8);
          const vein = add(stem, materials.metal, (i - 1) * .18, .28, i % 2 * .15); vein.scale.set(.45, .6, .45); vein.rotation.z = .7;
        }
      } else if (category === "forest") {
        for (let i = 0; i < 4; i++) {
          const root = add(stem, materials.wood, (i - 1.5) * .12, .12, i % 2 * .12); root.rotation.z = (i - 1.5) * .6;
          if (i % 2 === 0) add(leaf, materials.herb, (i - 1.5) * .12, .38, .12).scale.setScalar(.65);
        }
      } else if (category === "remains") {
        add(box, materials.bone, 0, .16, 0).scale.set(1.2, 1.1, 1);
        for (const x of [-.07, .07]) add(box, materials.dark, x, .19, .12).scale.setScalar(.25);
        for (let i = 0; i < 3; i++) {
          const bone = add(stem, materials.bone, (i - 1) * .17, .06, .25); bone.rotation.z = Math.PI / 2; bone.rotation.y = i * .9;
        }
      } else {
        for (let i = 0; i < 4; i++) {
          const x = (i - 1.5) * .13, z = i % 2 * .16;
          add(stem, materials.herb, x, .15, z).scale.setScalar(.65);
          if (i % 2) { const cap = add(stone, materials.herb, x, .3, z); cap.scale.set(.5, .25, .5); }
          else add(leaf, materials.herb, x, .26, z).scale.setScalar(.7);
        }
      }
      group.scale.setScalar(.8);
      return group;
    },
    dispose() { for (const geometry of [stone, stem, box, leaf]) geometry.dispose(); for (const material of Object.values(materials)) material.dispose(); },
  };
}
