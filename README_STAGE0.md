# HEX stage 0 — isolated geometry

Copy the included directories into the Nachalo repository root, preserving paths. Existing files are not replaced.

Run from Git Bash:

```bash
cd ~/Desktop/nachalo
cd beckend/gameservice && go test ./internal/hexgrid && cd ../..
node scripts/test_hex_geometry.mjs
npx.cmd tsc --noEmit
npm.cmd run build
```

No runtime imports, DB migrations, map changes or Docker actions. Geometry is pointy-top axial on X/Z with Y as rendering height. Full 3D hex prisms will be generated later by Three.js.
