# Phase 2.5: wall occlusion and sprite alpha

`LiveMatchScene3D` passes the objects from `adaptRealMap` to `NachaloScene3D`. The adapter includes dynamic objects and players only in the current team's visible cells. Wall occlusion samples only those rendered objects, plus the local hero marker, and checks the current visible-cell set again before tracing. Explored and unknown walls are never selected for fading. The GameService map, coordinates, movement and combat rules are unchanged.

The camera is orthographic. Five parallel rays sample each entity's projected silhouette (feet, middle, top and both sides). Only wall meshes hit before the entity fade to 20% opacity. The sampler runs at most every 33 ms, with an immediate check when the rendered object list changes. Existing shared wall materials provide 32 fade steps; opacity is eased every frame. Geometry is retained. Camera movement, player movement, fog updates and object removal are picked up by the next sample.

## Legacy sprite alpha audit

The sprite material uses `transparent: true`, `alphaTest: 0.05` and `depthWrite: false`. This correctly respects alpha in a source image and lets cutout pixels reveal the scene. It cannot remove an opaque background painted into the image.

Checked with `sips -g hasAlpha` and visual inspection:

| Source assets | Alpha | Preparation needed |
| --- | --- | --- |
| `public/ranger/ranger.webp`, `guardian/guardian.webp`, `berserk/berserk.webp`, `mag/mag.webp` | No | Yes; character portraits have an opaque light background. |
| `public/monsters/orc.webp`, `monsters/troll.webp` | No | Yes; the checkerboard/other background is painted into the image. |
| `public/main_resources/wood.webp` | No | Inspect and prepare if used as a 3D cutout. |
| `public/monsters/goblin.webp`, `monsters/goblins.webp`, `main_resources/barrel.webp`, `portal.png`, class `idle-front` PNG sprites | Yes | No alpha conversion indicated by this check. |

No original image was edited. Dedicated transparent cutouts should be prepared as separate assets in a later art task.

## Visual reference

- [Before](../artifacts/graphics-lab/phase-2-5-before.png)
- [After](../artifacts/graphics-lab/phase-2-5-after.png)

Both frames use the same GameService-format 15×15 fixture in Graphics Lab. They show the wall in front of the visible resource and the existing opaque ranger portrait. This fixture is a preview, not a live match.

## Checks

- Orthographic ray checks passed for five visible targets and five nearby walls, using the silhouette sizes passed for players and other objects. A distant wall stayed opaque.
- Removing targets, hiding all targets, hiding a wall across the fog boundary and moving the camera all updated the selected wall set correctly.
- A CPU-only sampler benchmark with 25 targets and 40 walls averaged 0.673 ms per pass over 300 passes on the local machine. Browser GPU frame time was not measured.
- `npx tsc --noEmit` and `npm run build` passed.
- Docker services are running, but no authenticated test match was supplied for a live gameplay check in this session.
