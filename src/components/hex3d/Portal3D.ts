import * as THREE from "three";

export type Portal3D = { group: THREE.Group; hitMeshes: THREE.Object3D[]; selection: THREE.Mesh; particles: THREE.Mesh[] };

export function createPortalAssets(stoneTexture: THREE.Texture) {
  const geometries = {
    pillar: new THREE.BoxGeometry(.27, 1.28, .31),
    arch: new THREE.BoxGeometry(.29, .29, .32),
    base: new THREE.BoxGeometry(.48, .16, .43),
    crack: new THREE.BoxGeometry(.012, .23, .006),
    rune: new THREE.OctahedronGeometry(.052, 0),
    particle: new THREE.OctahedronGeometry(.018, 0),
    energy: new THREE.PlaneGeometry(.94, 1.47),
    selection: new THREE.TorusGeometry(.58, .025, 5, 32),
  };
  const stone = new THREE.MeshStandardMaterial({ color: 0x8f969b, map: stoneTexture, roughness: .96, metalness: .02 });
  const darkStone = new THREE.MeshStandardMaterial({ color: 0x3d4953, map: stoneTexture, roughness: .95 });
  const crack = new THREE.MeshBasicMaterial({ color: 0x111923 });
  const rune = new THREE.MeshBasicMaterial({ color: 0x77d8f8, toneMapped: false });
  const runeColor = rune.color.clone();
  const selection = new THREE.MeshBasicMaterial({ color: 0x9eeeff, transparent: true, opacity: .82, depthWrite: false });
  const energy = new THREE.ShaderMaterial({
    uniforms: { uTime: { value: 0 }, uSelected: { value: 0 } }, transparent: true, depthWrite: false, side: THREE.DoubleSide,
    vertexShader: "varying vec2 vUv; void main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }",
    fragmentShader: `varying vec2 vUv;
      uniform float uTime;
      uniform float uSelected;
      void main() {
        float ripple = sin(vUv.y * 15.0 - uTime * 1.15 + sin(vUv.x * 13.0 + uTime * .55) * 1.4);
        float veil = sin(vUv.x * 22.0 + vUv.y * 9.0 + uTime * .8);
        float edge = smoothstep(0.0, .16, vUv.x) * smoothstep(0.0, .16, 1.0 - vUv.x)
                   * smoothstep(0.0, .1, vUv.y) * smoothstep(0.0, .1, 1.0 - vUv.y);
        vec3 color = mix(vec3(.16, .26, .55), vec3(.38, .78, .91), .5 + .5 * ripple);
        gl_FragColor = vec4(color, edge * (.29 + .11 * ripple + .07 * veil) * (1.0 + uSelected * .55));
      }`,
  });

  const create = (): Portal3D => {
    const group = new THREE.Group();
    const hitMeshes: THREE.Object3D[] = [];
    const add = (geometry: THREE.BufferGeometry, material: THREE.Material, hit = true) => {
      const mesh = new THREE.Mesh(geometry, material);
      mesh.castShadow = material !== energy;
      mesh.receiveShadow = material !== energy;
      group.add(mesh);
      if (hit) hitMeshes.push(mesh);
      return mesh;
    };
    for (const side of [-1, 1]) {
      add(geometries.base, darkStone).position.set(side * .55, .08, 0);
      add(geometries.pillar, stone).position.set(side * .55, .76, 0);
      const fissure = add(geometries.crack, crack, false);
      fissure.position.set(side * .55 + .035, .89, .16);
      fissure.rotation.z = side * .24;
    }
    for (let i = 0; i <= 6; i++) {
      const angle = Math.PI * i / 6;
      const block = add(geometries.arch, i % 2 ? darkStone : stone);
      block.position.set(.55 * Math.cos(angle), 1.4 + .51 * Math.sin(angle), 0);
      block.rotation.z = angle - Math.PI / 2;
      if (i % 2 === 0) {
        const glyph = add(geometries.rune, rune, false);
        glyph.position.set(block.position.x, block.position.y, .17);
      }
    }
    const veil = add(geometries.energy, energy);
    veil.position.set(0, .85, 0);
    veil.castShadow = false;
    const particles: THREE.Mesh[] = [];
    for (let i = 0; i < 4; i++) {
      const particle = add(geometries.particle, rune, false);
      particle.position.set((i - 1.5) * .2, .42 + i * .28, .08);
      particles.push(particle);
    }
    const halo = add(geometries.selection, selection, false);
    halo.rotation.x = -Math.PI / 2;
    halo.position.y = .035;
    halo.visible = false;
    return { group, hitMeshes, selection: halo, particles };
  };

  const update = (seconds: number, models: Iterable<Portal3D>) => {
    energy.uniforms.uTime.value = seconds;
    rune.color.copy(runeColor).multiplyScalar(.84 + .16 * Math.sin(seconds * 1.7));
    for (const model of models) for (const [index, particle] of model.particles.entries()) {
      particle.position.x = (index - 1.5) * .2 + Math.sin(seconds * .7 + index * 1.9) * .055;
      particle.position.y = .42 + index * .28 + Math.sin(seconds * .85 + index) * .055;
    }
  };
  const setSelected = (model: Portal3D, active: boolean) => {
    model.selection.visible = active;
    energy.uniforms.uSelected.value = active ? 1 : 0;
  };
  const dispose = () => {
    Object.values(geometries).forEach(geometry => geometry.dispose());
    for (const material of [stone, darkStone, crack, rune, selection, energy]) material.dispose();
  };
  return { create, update, setSelected, dispose };
}
