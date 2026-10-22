const RESOURCE_ASSETS = {
  wood: "/assets/nachalo/resources/wood.webp",
  stone: "/assets/nachalo/resources/stone.webp",
  iron: "/assets/nachalo/resources/iron.webp",
  water: "/assets/nachalo/resources/water.webp",
  food: "/assets/nachalo/resources/food.webp",
  gold: "/assets/nachalo/resources/gold.webp",
} as const;

export function resourceAsset(type: string | undefined | null): string | null {
  const key = type?.trim().toLowerCase();
  return key && Object.prototype.hasOwnProperty.call(RESOURCE_ASSETS, key)
    ? RESOURCE_ASSETS[key as keyof typeof RESOURCE_ASSETS]
    : null;
}

export function resourceImage(type: string | undefined | null, image: string | undefined | null): string | null {
  const direct = resourceAsset(type);
  if (direct) return direct;
  const legacyType = /^\/?main_resources\/(wood|stone|iron|water|food|gold)\.(?:webp|png)$/i.exec(image?.trim() ?? "")?.[1];
  return resourceAsset(legacyType);
}
