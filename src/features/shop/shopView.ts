import type { ShopItem } from "../../types/LobbyTypes";
import type { EquipmentItem, EquipmentSlot } from "../../types/equipment";
import { getAvailableEquipment } from "../equipment/equipmentView.ts";

export type ShopCategory = "all" | ShopItem["category"] | "preview";

export function clampShopQuantity(value: number): number {
    return Number.isFinite(value) ? Math.max(1, Math.min(99, Math.floor(value))) : 1;
}

export function maxAffordableQuantity(balance: number | null, unitPrice: number): number {
    if (balance === null || !Number.isFinite(balance) || !Number.isFinite(unitPrice) || unitPrice <= 0) return 0;
    return Math.max(0, Math.min(99, Math.floor(balance / unitPrice)));
}

export function purchaseTotal(unitPrice: number, quantity: number): number {
    return unitPrice * clampShopQuantity(quantity);
}

export function purchaseBlockReason(
    item: ShopItem,
    quantity: number,
    balance: number | null,
    forgeBuilt: boolean | null,
    libraryBuilt: boolean | null,
): string | null {
    if (item.requiresForge && forgeBuilt !== true) return forgeBuilt === false ? "Forge required" : "Checking Forge…";
    if (item.category === "scroll" && libraryBuilt !== true) return libraryBuilt === false ? "Library required" : "Checking Library…";
    if (balance === null) return "Checking balance…";
    if (!Number.isFinite(item.price) || item.price < 0) return "Price unavailable";
    if (balance < purchaseTotal(item.price, quantity)) return "Not enough gold";
    return null;
}

export function filterShopItems(items: ShopItem[], category: ShopCategory): ShopItem[] {
    if (category === "preview") return [];
    return category === "all" ? items : items.filter(item => item.category === category);
}

export function getSellableEquipment(
    inventory: EquipmentItem[] | undefined,
    equippedByCharacter: Record<string, Partial<Record<EquipmentSlot, EquipmentItem>>>,
    activeEquipped: Partial<Record<EquipmentSlot, EquipmentItem>>,
): EquipmentItem[] {
    return getAvailableEquipment(inventory, equippedByCharacter, activeEquipped)
        .filter(item => typeof item.sellPrice === "number" && Number.isFinite(item.sellPrice) && item.sellPrice >= 0);
}

export function selectionAfterSale(currentId: string | null, soldId: string): string | null {
    return currentId === soldId ? null : currentId;
}

export function actionDisabled(busy: boolean, reason: string | null): boolean {
    return busy || reason !== null;
}
