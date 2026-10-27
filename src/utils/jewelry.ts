import catalog from "../../beckend/gameservice/repository/jewelry_catalog.json";

// The server embeds this same catalog. Display text and art never duplicate combat data.
export const jewelryEntries = [...catalog.sets.flatMap(set => set.items), ...catalog.resources];
const byIdentity = new Map(jewelryEntries.flatMap(item => [item.code, item.name, item.ru, item.de].map(key => [key, item] as const)));
export function jewelryEntry(identity?: string | null) {
    return identity ? byIdentity.get(identity) : undefined;
}
