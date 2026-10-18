# Ranger 3D prototype (Phase 3.1)

This is a procedural, rigidly skinned prototype based on the supplied character sheet. It follows the hood, face scarf, dark cloth, leather, cape, quiver, bow and restrained turquoise palette. It is **not** a finished high-detail character or a production quality deformation rig.

## Rebuild

```sh
blender --background --python scripts/build_ranger.py
```

Blender 4.5's bundled NumPy does not load on this macOS 12 machine because the system Accelerate framework lacks `_cblas_caxpy$NEWLAPACK`. A local NumPy 1.24.4 override makes the Blender GLB exporter work:

```sh
/Applications/Blender.app/Contents/Resources/4.5/python/bin/python3.11 -m pip install --target /tmp/nachalo-blender-numpy 'numpy==1.24.4'
blender --background --python-expr "import sys; sys.path.insert(0, '/tmp/nachalo-blender-numpy'); import numpy" --python scripts/build_ranger.py
test -s public/assets/nachalo/characters/ranger/ranger.glb
```

The exported GLB has 17 joints, one skin and five clips: `Idle`, `Walk`, `Attack_Bow`, `Hit`, and `Death`. The animation is in place; Three.js interpolates the actor's position after a server snapshot changes. The local Ranger alone uses this model in the live 3D match. Other players, heroes, monsters, and Classic retain their existing art and logic. If the GLB fails to load, the previous sprite stays visible.

## Preview

Open `/graphics-lab`, use arrow buttons or keyboard arrows for movement in four directions, and use the Ranger buttons for attack, hit, death and reset. The Lab uses map fixtures and is not a match simulator. Preview images are in `artifacts/ranger/`.
The browser recording is `artifacts/ranger/graphics-lab-demo.webm`.

Known visual limits: rigid skinning creates segmented joints; the bowstring does not deform with the draw; the small tactical scale masks some gesture detail; walk uses a generic cycle and can show mild foot sliding. A hand-authored mesh, weight painting, and polished animation are still needed for a final asset.
