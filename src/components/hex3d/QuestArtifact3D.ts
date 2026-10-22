import * as THREE from "three";

export type QuestArtifact3D = { group: THREE.Group; floating: THREE.Group; hitObjects: THREE.Object3D[]; selection: THREE.Mesh };

export function createQuestArtifactAssets(loader: THREE.TextureLoader, invalidate: () => void, stoneTexture: THREE.Texture) {
  const pedestalGeometry = new THREE.CylinderGeometry(.28, .37, .27, 8);
  const capGeometry = new THREE.CylinderGeometry(.3, .3, .055, 8);
  const glyphGeometry = new THREE.TorusGeometry(.27, .015, 5, 24);
  const fallbackGeometry = new THREE.OctahedronGeometry(.18, 0);
  const stone = new THREE.MeshStandardMaterial({ color: 0x737b7e, map: stoneTexture, roughness: .95 });
  const gold = new THREE.MeshStandardMaterial({ color: 0xc29b58, roughness: .42, metalness: .65 });
  const glow = new THREE.MeshBasicMaterial({ color: 0xe8c681, transparent: true, opacity: .55, depthWrite: false });
  const fallback = new THREE.MeshStandardMaterial({ color: 0xb7a5e4, emissive: 0x55478b, emissiveIntensity: .6, roughness: .3, metalness: .35 });
  const images = new Map<string, { texture: THREE.Texture; material: THREE.SpriteMaterial }>();

  const create = (image: string): QuestArtifact3D => {
    const group = new THREE.Group();
    const hitObjects: THREE.Object3D[] = [];
    const pedestal = new THREE.Mesh(pedestalGeometry, stone);
    pedestal.position.y = .15;
    pedestal.castShadow = true;
    pedestal.receiveShadow = true;
    group.add(pedestal);
    hitObjects.push(pedestal);
    const cap = new THREE.Mesh(capGeometry, gold);
    cap.position.y = .305;
    group.add(cap);
    hitObjects.push(cap);
    const aura = new THREE.Mesh(glyphGeometry, glow);
    aura.rotation.x = -Math.PI / 2;
    aura.position.y = .36;
    group.add(aura);
    const selection = new THREE.Mesh(glyphGeometry, glow);
    selection.rotation.x = -Math.PI / 2;
    selection.position.y = .035;
    selection.visible = false;
    group.add(selection);
    const floating = new THREE.Group();
    floating.position.y = .69;
    group.add(floating);
    if (image) {
      let visual = images.get(image);
      if (!visual) {
        const texture = loader.load(image, invalidate);
        texture.colorSpace = THREE.SRGBColorSpace;
        visual = { texture, material: new THREE.SpriteMaterial({ map: texture, transparent: true, alphaTest: .08, depthWrite: false, toneMapped: false }) };
        images.set(image, visual);
      }
      const sprite = new THREE.Sprite(visual.material);
      sprite.scale.set(.48, .56, 1);
      floating.add(sprite);
      hitObjects.push(sprite);
    } else {
      const gem = new THREE.Mesh(fallbackGeometry, fallback);
      gem.castShadow = true;
      floating.add(gem);
      hitObjects.push(gem);
    }
    return { group, floating, hitObjects, selection };
  };
  const update = (model: QuestArtifact3D, seconds: number) => {
    model.floating.position.y = .69 + Math.sin(seconds * 1.25) * .055;
    model.floating.rotation.y = Math.sin(seconds * .4) * .12;
    glow.opacity = .42 + Math.sin(seconds * 1.5) * .1;
  };
  const dispose = () => {
    for (const geometry of [pedestalGeometry, capGeometry, glyphGeometry, fallbackGeometry]) geometry.dispose();
    for (const material of [stone, gold, glow, fallback]) material.dispose();
    for (const visual of images.values()) { visual.material.dispose(); visual.texture.dispose(); }
  };
  return { create, update, dispose };
}
