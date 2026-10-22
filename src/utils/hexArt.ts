import type { HexActor, HexResource } from "@/types/HexGameTypes";
import { resourceImage } from "@/utils/resourceAssets";

const heroes: Record<string, string> = {
  guardian: "/guardian/idle-front-1.png",
  ranger: "/ranger/idle-front-1.png",
  berserker: "/berserk/idle-front-2.png",
  mystic: "/mag/mag-idle-front-2.png",
};
const monsters = new Set(["goblin", "goblins", "orc", "troll"]);

export function heroArt(characterType: string | undefined): string {
  return heroes[characterType ?? ""] ?? "/assets/nachalo/catacombs/hero_reference.webp";
}

export function monsterArt(actor: HexActor): string | null {
  const image = actor.image?.toLowerCase();
  if (image && /^\/monsters\/[a-z]+\.webp$/.test(image)) {
    const name = image.slice("/monsters/".length, -".webp".length);
    if (monsters.has(name)) return image;
  }
  const name = actor.name?.toLowerCase();
  return name && monsters.has(name) ? `/monsters/${name}.webp` : null;
}

export function resourceArt(resource: HexResource | undefined): string | null {
  if (!resource) return null;
  if (resource.item_type !== "artifact") {
    const asset = resourceImage(resource.type, resource.image);
    if (asset) return asset;
  }
  const image = resource.image?.toLowerCase();
  if (image && /^\/main_resources\/[a-z]+\.webp$/.test(image)) {
    const name = image.slice("/main_resources/".length, -".webp".length);
    if (name === "barrel") return "/assets/nachalo/objects/barrel.png";
    if (resource.item_type === "artifact") return image;
  }
  const name = resource.type?.toLowerCase();
  return name === "barrel" ? "/assets/nachalo/objects/barrel.png" : null;
}
