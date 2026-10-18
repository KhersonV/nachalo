# Nachalo — Ancient Catacombs Props v1

6 original transparent PNG references (1254×1254), plus 6 reduced WebP previews (768×768).

## Files
- wall_torch_01, wall_torch_02 — wall torches, alternative designs
- floor_brazier — freestanding fire bowl
- column_intact — intact stone pillar
- column_broken — broken pillar
- rubble_pile — decorative stone debris

## IMPORTANT
These are **2D concept illustrations with transparency**, NOT 3D geometry, GLB, skeletal rigs, or flame animations. Preview WebP files can be displayed as sprites/billboards but will stay flat. Do not put these images directly on walls as if they were flat wall textures.

## Recommended Three.js usage
1. Keep these assets as visual references when constructing reusable 3D models from geometry and existing v2 stone/iron materials. No new dependencies required.
2. Create wall torches from a wall bracket, metal basket, small ember core, animated flame planes/particles, and a limited pooled warm point light. Mount only on visible interior wall faces, chosen deterministically from map seed and coordinates.
3. Build columns from shared cylinder/box/beveled geometry and stone textures, with an intact and a broken variant. Rubble via small irregular stone meshes with few draw calls. Do not add invisible collision or block a walkable tile.
4. For fire, consider camera-facing transparent flame sprites as a temporary visual option, but the full torch renders include built-in lighting that may clash with live scene illumination. Prefer procedural animated fire.
5. Fog: never show lights/particles, sounds or object changes from outside currently visible tiles. In explored but not visible, show only known static architecture with dimmed materials; no active flame effects or light leakage.
6. Avoid changing Go backend or matchmaking or game logic; implement in /graphics-lab until validated.
7. Check performance and low-end browsers; limit number of real dynamic shadow-casting lights.

## Suggested order
1. Implement one 3D torch and visually compare with wall_torch_01.
2. Make its fire flicker and nearby floor/wall receive warm local illumination.
3. Add column variants and small safe rubble, then optional brazier.
4. Confirm fog-of-war, transparency/camera occlusion and performance.
