import type { EquipmentBonuses, EquipmentItem, EquipmentStats } from '@/types/equipment';
import type { CraftRecipe } from '@/components/ProfessionCrafting';

export type CodexRecipe = CraftRecipe & { pocketStack?: number; handedness?: string };
export type CodexFact = { id: string; title: string; text: string; formula?: string };
export type CodexHero = { code: string; name: string; image: string; stats: EquipmentStats };
export type CodexMonster = { id: number; name: string; type: string; image: string; hp: number; attack: number; defense: number; speed: number; maneuverability: number; vision: number; equipmentChance?: number };
export type CodexResource = { id: number; code: string; name: string; description: string; image: string; tradable?: boolean };
export type CodexShopItem = { id: number; type: string; category: string; name: string; description: string; image: string; price: number; requiresForge: boolean; requiresLibrary: boolean; effect: Record<string, number> };
export type CodexDrop = { code: string; name: string; professionChance: number; equipmentChance: number; xp: number; materials: { code: string; name: string; class: string; professionName: string; quantity: number; chance: number; description: string; image: string }[] };
export type CodexGathering = { code: string; name: string; class: string; profession: string; professionName: string; description: string; image: string; materials: { code: string; name: string; minLevel: number; chance: number; description: string; image: string }[] };
export type CodexData = {
 ruleset: 'legacy' | 'experimental_v34';
 loot?: { uncommonEquipmentChance: number; barrelArtifactChance: number; barrelDamageChance: number; barrelResourceChance: number; barrelEmptyChance: number; questArtifactGuaranteedInLastBarrel: boolean };
 balance?: { hatred_percent: number; mystic_magic_through_block: boolean; gear_hp: number; gear_atk: number; gear_def: number; gear_main: number; slot_budgets: Record<string, { hp: number; attack: number; defense: number; reflex: number }>; rare_slot_reflex: Record<string, number>; jewelry_reflex: Record<string, Record<string, number>>; fx: Record<string, number>; mechanics: Record<string, number> };
 heroes: CodexHero[]; facts: CodexFact[]; equipment: EquipmentItem[]; recipes: CodexRecipe[];
 resources: CodexResource[]; mapResources: string[]; monsters: CodexMonster[]; drops: CodexDrop[]; gathering: CodexGathering[];
 professions: { code: string; name: string; description: string; category: string }[];
 artifacts: { id: number; code: string; description: string; image: string; bonus: Record<string, number> }[];
 dismantle: Record<string, { code: string; quantity: number }[]>;
 buildings: { code: string; name: string; costs: Record<string, number> }[];
 reflexProgression?: Record<string,Record<string,number>>; itemSources: Record<string,string>; shop: CodexShopItem[]; unavailable: string[]; sources: string[];
};
export type CodexArticle = {
 id: string; category: string; title: string; lead: string; paragraphs: string[];
 lore?: string[]; image?: string; related: string[]; facts?: string[]; keywords?: string[];
 kind?: 'hero' | 'item' | 'recipe' | 'material' | 'monster' | 'artifact' | 'gathering' | 'shop' | 'comparison' | 'slots' | 'buildings' | 'professions';
 entity?: string;
 action?: { label: string; href: string };
};
export type CodexCategory = { id: string; numeral: string; title: string; subtitle: string; rune: string; image?: string };
export type BonusValues = EquipmentBonuses;
