/** Pointy-top axial geometry shared by the HEX map views. */
export type HexCoord = Readonly<{ q: number; r: number }>;
export type HexWorld = Readonly<{ x: number; z: number }>;
export type HexCube = Readonly<{ x: number; y: number; z: number }>;
export const MAX_HEX_COORD = 1_000_000;
export const HEX_DIRECTIONS: readonly HexCoord[] = [
  { q: 1, r: 0 }, { q: 1, r: -1 }, { q: 0, r: -1 },
  { q: -1, r: 0 }, { q: -1, r: 1 }, { q: 0, r: 1 },
] as const;
function integer(v: number): boolean { return Number.isSafeInteger(v); }
export function validHex(c: HexCoord): boolean { return integer(c.q) && integer(c.r) && Math.abs(c.q) <= MAX_HEX_COORD && Math.abs(c.r) <= MAX_HEX_COORD; }
function requireHex(c: HexCoord): void { if (!validHex(c)) throw new RangeError('invalid hex coordinate'); }
export function hexNeighbor(c: HexCoord, direction: number): HexCoord { requireHex(c); if (!integer(direction) || direction < 0 || direction >= 6) throw new RangeError('direction'); const d=HEX_DIRECTIONS[direction];const n={q:c.q+d.q,r:c.r+d.r};requireHex(n);return n; }
export function hexOpposite(direction: number): number { if (!integer(direction)||direction<0||direction>=6) throw new RangeError('direction');return (direction+3)%6; }
export function axialToCube(c: HexCoord): HexCube { requireHex(c);return {x:c.q,y:-c.q-c.r,z:c.r}; }
export function cubeToAxial(c: HexCube): HexCoord { if (![c.x,c.y,c.z].every(integer)||c.x+c.y+c.z!==0)throw new RangeError('invalid cube');const h={q:c.x,r:c.z};requireHex(h);return h; }
export function hexDistance(a: HexCoord,b: HexCoord): number { requireHex(a);requireHex(b);const q=a.q-b.q,r=a.r-b.r;return (Math.abs(q)+Math.abs(r)+Math.abs(q+r))/2; }
function radiusOK(radius:number): boolean{return Number.isFinite(radius)&&radius>0;}
export function axialToWorld(c: HexCoord,radius:number): HexWorld {requireHex(c);if(!radiusOK(radius))throw new RangeError('radius');return {x:Math.sqrt(3)*radius*(c.q+c.r/2),z:1.5*radius*c.r};}
export function roundAxial(q:number,r:number):HexCoord{
 if(!Number.isFinite(q)||!Number.isFinite(r)||Math.abs(q)>MAX_HEX_COORD+2||Math.abs(r)>MAX_HEX_COORD+2)throw new RangeError('fractional hex');
 const x=q,y=-q-r,z=r;let rx=Math.floor(x+0.5),ry=Math.floor(y+0.5),rz=Math.floor(z+0.5);
 const dx=Math.abs(rx-x),dy=Math.abs(ry-y),dz=Math.abs(rz-z);
 if(dx>=dy&&dx>=dz)rx=-ry-rz;else if(dy>=dz)ry=-rx-rz;else rz=-rx-ry;
 const h={q:rx||0,r:rz||0};requireHex(h);return h;
}
export function worldToAxial(p:HexWorld,radius:number):HexCoord {if(!radiusOK(radius)||!Number.isFinite(p.x)||!Number.isFinite(p.z))throw new RangeError('world');return roundAxial((Math.sqrt(3)*p.x/3-p.z/3)/radius,2*p.z/(3*radius));}
export function hexInDisk(c: HexCoord,radius:number):boolean {return validHex(c)&&integer(radius)&&radius>=0&&hexDistance({q:0,r:0},c)<=radius;}
export function hexRing(center:HexCoord,radius:number):HexCoord[]{requireHex(center);if(!integer(radius)||radius<0||radius>MAX_HEX_COORD)throw new RangeError('ring radius');if(radius===0)return [center];let c={q:center.q+HEX_DIRECTIONS[4].q*radius,r:center.r+HEX_DIRECTIONS[4].r*radius};const out:HexCoord[]=[];for(const dir of HEX_DIRECTIONS)for(let i=0;i<radius;i++){requireHex(c);out.push(c);c={q:c.q+dir.q,r:c.r+dir.r};}return out;}
export function hexDisk(center:HexCoord,radius:number):HexCoord[]{if(!integer(radius)||radius<0||radius>2048)throw new RangeError('disk radius');const out:HexCoord[]=[];for(let i=0;i<=radius;i++)out.push(...hexRing(center,i));return out;}
export function canonicalHexEdge(a:HexCoord,b:HexCoord):Readonly<{a:HexCoord;b:HexCoord}>{if(hexDistance(a,b)!==1)throw new RangeError('edge not adjacent');return b.q<a.q||(b.q===a.q&&b.r<a.r)?{a:b,b:a}:{a,b};}
export function hexKey(c:HexCoord):string{requireHex(c);return `${c.q},${c.r}`;}
