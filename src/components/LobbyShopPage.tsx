"use client";

import UiText from "@/components/UiText";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

import React from "react";
import Link from "next/link";
import { API_BASE } from "@/utils/serviceUrls";
import { useAuth } from "../contexts/AuthContext";
import { ConfirmDialog, EmptyState, ErrorState, LoadingState, StatusMessage, Tabs } from "./ui";
import { actionDisabled, clampShopQuantity, filterShopItems, getSellableEquipment, maxAffordableQuantity, purchaseBlockReason, purchaseTotal, selectionAfterSale, type ShopCategory } from "@/features/shop/shopView";
import type { PlayerShopState, ShopItem } from "../types";
import type { EquipmentItem, EquipmentSlot } from "../types/equipment";
import styles from "../styles/LobbyShopPage.module.css";
import { resourceImage } from "@/utils/resourceAssets";
import { equipmentImage } from "@/utils/equipmentAssets";

type ShopTab = "buy" | "sell";
type EquipmentSnapshot = {
    inventory?: EquipmentItem[];
    equippedByCharacter: Record<string, Partial<Record<EquipmentSlot, EquipmentItem>>>;
    equipped: Partial<Record<EquipmentSlot, EquipmentItem>>;
};

const CATEGORY_LABELS: Record<ShopCategory, string> = {
    all: "All goods", resource: "Supplies", scroll: "Scrolls", blueprint: "Blueprints", preview: "Green sets",
};
const GREEN_SET_NAMES = ["Sagecloth · Mystic", "Bloodroot · Berserker", "Greenwisp · Ranger", "Aegiswarden · Guardian"];
const EFFECT_LABELS: Record<string, string> = {
    health: "Health", ap_half: "AP (×0.5)", sight_bonus: "Sight", structure_health: "Structure health",
    structure_defense: "Structure defense", turret_damage: "Turret damage", structure_blocking: "Blocks passage",
};

function formatGold(value: number) { return `${value.toLocaleString()} Gold`; }
function formatEnum(value: string) { return value.split("_").map(part => part.charAt(0).toUpperCase() + part.slice(1)).join(" "); }
function errorMessage(error: unknown) { return error instanceof Error ? error.message : "Shop request failed"; }

async function responseError(response: Response): Promise<Error> {
    const text = (await response.text()).trim();
    let code = text;
    try { code = JSON.parse(text)?.error || text; } catch { /* Shop errors can be plain text. */ }
    const messages: Record<string, string> = {
        forge_required: "Build the Forge in Base to buy this blueprint.",
        library_required: "Build the Library in Base to buy scrolls.",
        "insufficient balance": "Not enough gold for this purchase.",
        item_is_equipped: "Unequip this item before selling it.",
        item_locked: "This item is locked and cannot be sold.",
        item_not_in_inventory: "This item is no longer in your inventory.",
    };
    return new Error(messages[code] || code || `Request failed (${response.status})`);
}

function equipmentFromResponse(value: unknown): EquipmentSnapshot {
    const raw = value && typeof value === "object" ? value as Record<string, unknown> : {};
    const inventory = Array.isArray(raw.inventoryItems) ? raw.inventoryItems : Array.isArray(raw.inventory) ? raw.inventory : undefined;
    return {
        inventory: inventory as EquipmentItem[] | undefined,
        equippedByCharacter: raw.equippedByCharacter && typeof raw.equippedByCharacter === "object" ? raw.equippedByCharacter as EquipmentSnapshot["equippedByCharacter"] : {},
        equipped: raw.equipped && typeof raw.equipped === "object" ? raw.equipped as EquipmentSnapshot["equipped"] : {},
    };
}

function normalizePlayer(value: unknown): PlayerShopState {
    const raw = value && typeof value === "object" ? value as Record<string, unknown> : {};
    if ((typeof raw.balance !== "number" && typeof raw.balance !== "string") || raw.balance === "") throw new Error("Server balance is unavailable");
    const balance = Number(raw.balance);
    if (!Number.isFinite(balance)) throw new Error("Server balance is unavailable");
    return { user_id: Number(raw.user_id), balance, inventory: typeof raw.inventory === "string" ? raw.inventory : JSON.stringify(raw.inventory ?? {}) };
}

function ownedCount(player: PlayerShopState | null, item: ShopItem): number {
    if (!player) return 0;
    try {
        const inventory = JSON.parse(player.inventory);
        const key = item.inventoryKey || `resource_${item.id}`;
        return Math.max(0, Number(inventory?.[key]?.item_count) || 0);
    } catch { return 0; }
}

function ProductImage({ src, name, large = false }: { src?: string; name: string; large?: boolean }) {
    const [failed, setFailed] = React.useState(false);
    React.useEffect(() => setFailed(false), [src]);
    return <span className={`${styles.productImage} ${large ? styles.productImageLarge : ""}`} aria-label={name}>
        {src && !failed ? <img src={src} alt="" onError={() => setFailed(true)} /> : <span aria-hidden="true">✦</span>}
    </span>;
}

function ShopItemCard({ item, selected, onSelect }: { item: ShopItem; selected: boolean; onSelect: () => void }) {
    return <button type="button" className={`${styles.productCard} ${selected ? styles.productCardSelected : ""}`} aria-pressed={selected} onClick={onSelect}>
        <ProductImage src={item.category === "resource" ? resourceImage(item.type, item.image) ?? item.image : item.image} name={item.name} />
        <span className={styles.cardCopy}><span className={styles.cardType}><UiText text={CATEGORY_LABELS[item.category]} /></span><strong><UiText text={item.name} /></strong><small><UiText text={item.description} /></small></span>
        <span className={styles.cardPrice}>{formatGold(item.price)}</span>
    </button>;
}

function SellItemCard({ item, selected, onSelect }: { item: EquipmentItem; selected: boolean; onSelect: () => void }) {
    return <button type="button" className={`${styles.productCard} ${selected ? styles.productCardSelected : ""}`} data-rarity={item.rarity} aria-pressed={selected} onClick={onSelect}>
        <ProductImage src={equipmentImage(item.imageUrl, item.code)} name={item.name} />
        <span className={styles.cardCopy}><span className={styles.cardType}><UiText text={formatEnum(item.slot)} /> · <UiText text={formatEnum(item.rarity)} /></span><strong><UiText text={item.name} /></strong><small><UiText text={item.setName || item.classRestriction || "Equipment"} /></small></span>
        <span className={styles.cardPrice}>{formatGold(item.sellPrice!)}</span>
    </button>;
}

function QuantitySelector({ count, max, disabled, onChange }: { count: number; max: number; disabled: boolean; onChange: (count: number) => void }) {
    return <div className={styles.quantityControl}>
        <label htmlFor="shop-quantity"><UiText text="Quantity" /></label>
        <div className={styles.quantityRow}>
            {[-5, -1].map(delta => <button key={delta} type="button" disabled={disabled || count <= 1} onClick={() => onChange(count + delta)}>{delta}</button>)}
            <input id="shop-quantity" aria-label="Quantity" type="number" inputMode="numeric" min={1} max={99} value={count} disabled={disabled} onChange={event => onChange(Number(event.target.value))} />
            {[1, 5].map(delta => <button key={delta} type="button" disabled={disabled || count >= 99} onClick={() => onChange(count + delta)}>+{delta}</button>)}
            <button type="button" disabled={disabled || max < 1 || count === max} onClick={() => onChange(max)}><UiText text="Max" /></button>
        </div>
    </div>;
}

function ShopItemDetails({ item, player, count, forgeBuilt, libraryBuilt, busy, onCount, onBuy }: {
    item: ShopItem; player: PlayerShopState | null; count: number; forgeBuilt: boolean | null; libraryBuilt: boolean | null;
    busy: boolean; onCount: (count: number) => void; onBuy: () => void;
}) {
    const reason = purchaseBlockReason(item, count, player?.balance ?? null, forgeBuilt, libraryBuilt);
    const max = maxAffordableQuantity(player?.balance ?? null, item.price);
    const effects = Object.entries(item.effect || {}).filter(([, value]) => typeof value === "number");
    return <div className={styles.detailsBody}>
        <div className={styles.detailsHero}><ProductImage src={item.category === "resource" ? resourceImage(item.type, item.image) ?? item.image : item.image} name={item.name} large /><div><span className={styles.eyebrow}><UiText text={CATEGORY_LABELS[item.category]} /></span><h2><UiText text={item.name} /></h2><p><UiText text={item.description} /></p></div></div>
        {effects.length > 0 && <div className={styles.effects}>{effects.map(([key, value]) => <span key={key}><UiText text={EFFECT_LABELS[key] || formatEnum(key)} />: {key === "structure_blocking" ? <UiText text={value ? "Yes" : "No"} /> : `${value > 0 ? "+" : ""}${value}`}</span>)}</div>}
        <div className={styles.detailFacts}><span><UiText text="Unit price" /> <strong>{formatGold(item.price)}</strong></span><span><UiText text="Owned" /> <strong>{ownedCount(player, item)}</strong></span></div>
        <QuantitySelector count={count} max={max} disabled={busy} onChange={onCount} />
        <div className={styles.totalRow}><span><UiText text="Total" /></span><strong>{formatGold(purchaseTotal(item.price, count))}</strong></div>
        <div className={styles.balanceHint}><UiText text="Available:" /> {player ? formatGold(player.balance) : <UiText text="Loading…" />}</div>
        {reason && <p className={styles.blockReason}><UiText text={reason} />{(reason.includes("Forge") || reason.includes("Library")) && <> · <Link href="/base"><UiText text="Go to Base" /></Link></>}</p>}
        <button type="button" className={styles.primaryAction} disabled={actionDisabled(busy, reason)} onClick={onBuy}><UiText text={busy ? "Buying…" : "Buy"} /> {!busy && count}</button>
    </div>;
}

function SellItemDetails({ item, busy, onSell }: { item: EquipmentItem; busy: boolean; onSell: () => void }) {
    return <div className={styles.detailsBody}>
        <div className={styles.detailsHero}><ProductImage src={equipmentImage(item.imageUrl, item.code)} name={item.name} large /><div><span className={styles.eyebrow}><UiText text={formatEnum(item.rarity)} /> · <UiText text={formatEnum(item.slot)} /></span><h2><UiText text={item.name} /></h2><p><UiText text={item.setName || item.classRestriction || "Available equipment"} /></p></div></div>
        <div className={styles.detailFacts}><span><UiText text="Item instance" /> <strong className={styles.instanceId}>{item.instanceId}</strong></span><span><UiText text="Sell value" /> <strong>{formatGold(item.sellPrice!)}</strong></span></div>
        <p className={styles.sellWarning}><UiText text="Selling permanently removes this item from your inventory." /></p>
        <button type="button" className={styles.primaryAction} disabled={busy} onClick={onSell}><UiText text={busy ? "Selling…" : "Sell for"} /> {!busy && formatGold(item.sellPrice!)}</button>
    </div>;
}

function MobileDetails({ open, children, onClose }: { open: boolean; children: React.ReactNode; onClose: () => void }) {
    const ref = React.useRef<HTMLDialogElement>(null);
    React.useEffect(() => {
        const dialog = ref.current;
        if (!dialog) return;
        if (open && !dialog.open) dialog.showModal();
        if (!open && dialog.open) dialog.close();
    }, [open]);
    return <dialog ref={ref} className={styles.mobileDialog} aria-label="Item details" onCancel={event => { event.preventDefault(); onClose(); }}>
        <button type="button" className={styles.drawerClose} onClick={onClose} aria-label="Close item details">✕</button>{children}
    </dialog>;
}

export default function LobbyShopPage() {
    const { language } = useGameLanguage();
    const t = (source: string) => uiText(language, source);
    const { user } = useAuth();
    const [tab, setTab] = React.useState<ShopTab>("buy");
    const [category, setCategory] = React.useState<ShopCategory>("all");
    const [catalog, setCatalog] = React.useState<ShopItem[]>([]);
    const [player, setPlayer] = React.useState<PlayerShopState | null>(null);
    const [equipment, setEquipment] = React.useState<EquipmentSnapshot>({ equippedByCharacter: {}, equipped: {} });
    const [forgeBuilt, setForgeBuilt] = React.useState<boolean | null>(null);
    const [libraryBuilt, setLibraryBuilt] = React.useState<boolean | null>(null);
    const [counts, setCounts] = React.useState<Record<string, number>>({});
    const [selectedBuyId, setSelectedBuyId] = React.useState<string | null>(null);
    const [selectedSellId, setSelectedSellId] = React.useState<string | null>(null);
    const [mobileDetailsOpen, setMobileDetailsOpen] = React.useState(false);
    const [confirmSellId, setConfirmSellId] = React.useState<string | null>(null);
    const [busy, setBusy] = React.useState(false);
    const busyRef = React.useRef(false);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState("");
    const [notice, setNotice] = React.useState("");

    const sellItems = React.useMemo(() => getSellableEquipment(equipment.inventory, equipment.equippedByCharacter, equipment.equipped), [equipment]);
    const visibleCatalog = React.useMemo(() => filterShopItems(catalog, category), [catalog, category]);
    const selectedBuy = visibleCatalog.find(item => item.type === selectedBuyId) || visibleCatalog[0] || null;
    const selectedSell = sellItems.find(item => item.instanceId === selectedSellId) || sellItems[0] || null;
    const confirmItem = sellItems.find(item => item.instanceId === confirmSellId) || null;

    const loadData = React.useCallback(async (confirmedPlayer?: PlayerShopState | null, confirmedEquipment?: EquipmentSnapshot) => {
        if (!user?.token) { setLoading(false); return false; }
        setLoading(true);
        const headers = { Authorization: `Bearer ${user.token}` };
        const fetchJson = async (path: string) => {
            const response = await fetch(`${API_BASE}${path}`, { headers, cache: "no-store" });
            if (!response.ok) throw await responseError(response);
            return response.json();
        };
        const results = await Promise.allSettled([
            fetchJson("/game/shop/items"), fetchJson(`/game/player/${user.id}`),
            fetchJson("/game/base/state"), fetchJson("/game/equipment"),
        ]);
        const failures: string[] = [];
        const [catalogResult, playerResult, baseResult, equipmentResult] = results;
        if (catalogResult.status === "fulfilled" && Array.isArray(catalogResult.value?.items)) setCatalog(catalogResult.value.items);
        else { setCatalog([]); failures.push("catalog"); }
        if (playerResult.status === "fulfilled") { try { setPlayer(normalizePlayer(playerResult.value)); } catch { setPlayer(confirmedPlayer || null); failures.push("balance"); } }
        else { setPlayer(confirmedPlayer || null); failures.push("balance"); }
        if (baseResult.status === "fulfilled") {
            const base = baseResult.value;
            const forge = typeof base?.forge?.built === "boolean" ? base.forge.built : null;
            const library = typeof base?.library?.built === "boolean" ? base.library.built : null;
            setForgeBuilt(forge);
            setLibraryBuilt(library);
            if (forge === null || library === null) failures.push("building access");
        } else { setForgeBuilt(null); setLibraryBuilt(null); failures.push("building access"); }
        if (equipmentResult.status === "fulfilled") {
            const snapshot = equipmentFromResponse(equipmentResult.value?.data);
            setEquipment(snapshot);
            if (!snapshot.inventory) failures.push("equipment");
        }
        else { setEquipment(confirmedEquipment || { equippedByCharacter: {}, equipped: {} }); failures.push("equipment"); }
        setError(failures.length ? `Could not refresh ${failures.join(", ")}. Retry to load current server data.` : "");
        setLoading(false);
        return failures.length === 0;
    }, [user?.id, user?.token]);

    React.useEffect(() => { void loadData(); }, [loadData]);
    const changeCount = (type: string, next: number) => setCounts(current => ({ ...current, [type]: clampShopQuantity(next) }));

    const purchaseAttempt = React.useRef<{ key: string; commandId: string } | null>(null);
    const buy = async (item: ShopItem) => {
        if (!user?.token || busyRef.current || !catalog.some(entry => entry.type === item.type)) return;
        const count = clampShopQuantity(counts[item.type] ?? 1);
        const reason = purchaseBlockReason(item, count, player?.balance ?? null, forgeBuilt, libraryBuilt);
        if (reason) { setError(reason); return; }
        busyRef.current = true; setBusy(true); setError(""); setNotice("");
        const requestKey = `${item.type}:${count}`;
        if (purchaseAttempt.current?.key !== requestKey) purchaseAttempt.current = { key: requestKey, commandId: crypto.randomUUID() };
        try {
            const response = await fetch(`${API_BASE}/game/shop/buy`, {
                method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" },
                body: JSON.stringify({ player_id: user.id, item_type: item.type, count, commandId: purchaseAttempt.current!.commandId }),
            });
            if (!response.ok) throw await responseError(response);
            purchaseAttempt.current = null;
            const data = await response.json().catch(() => null);
            let confirmedPlayer: PlayerShopState | null = null;
            try { if (data?.player) confirmedPlayer = normalizePlayer(data.player); } catch { /* Refresh below is authoritative. */ }
            if (confirmedPlayer) setPlayer(confirmedPlayer);
            const fresh = await loadData(confirmedPlayer);
            setNotice(`${t("Purchased")} ${t(item.name)} ×${count}.${fresh ? "" : ` ${t("Refresh the shop to verify the latest details.")}`}`);
        } catch (caught) { setError(errorMessage(caught)); }
        finally { busyRef.current = false; setBusy(false); }
    };

    const sell = async () => {
        const item = confirmItem;
        if (!item || !user?.token || busyRef.current || !equipment.inventory) return;
        busyRef.current = true; setBusy(true); setError(""); setNotice("");
        try {
            const response = await fetch(`${API_BASE}/game/equipment/sell`, {
                method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" },
                body: JSON.stringify({ itemInstanceId: item.instanceId }),
            });
            if (!response.ok) throw await responseError(response);
            const data = await response.json().catch(() => null);
            const confirmedPlayer = typeof data?.balance === "number" && player ? { ...player, balance: data.balance } : null;
            if (confirmedPlayer) setPlayer(confirmedPlayer);
            const confirmedEquipment = data?.data ? equipmentFromResponse(data.data) : undefined;
            setEquipment(confirmedEquipment || { equippedByCharacter: {}, equipped: {} });
            setSelectedSellId(current => selectionAfterSale(current, item.instanceId));
            setConfirmSellId(null);
            setMobileDetailsOpen(false);
            const fresh = await loadData(confirmedPlayer, confirmedEquipment);
            setNotice(`${t("Sold")} ${t(item.name)}.${fresh ? "" : ` ${t("Refresh the shop to verify the latest details.")}`}`);
        } catch (caught) { setError(errorMessage(caught)); }
        finally { busyRef.current = false; setBusy(false); }
    };

    const selectTab = (next: ShopTab) => { if (!busyRef.current) { setTab(next); setMobileDetailsOpen(false); setError(""); setNotice(""); } };
    const showDetails = () => { if (window.matchMedia("(max-width: 760px)").matches) setMobileDetailsOpen(true); };
    const closeDetails = () => { if (!busyRef.current && !confirmSellId) setMobileDetailsOpen(false); };
    const detail = tab === "buy" ? selectedBuy && <ShopItemDetails item={selectedBuy} player={player} count={counts[selectedBuy.type] ?? 1} forgeBuilt={forgeBuilt} libraryBuilt={libraryBuilt} busy={busy} onCount={next => changeCount(selectedBuy.type, next)} onBuy={() => void buy(selectedBuy)} /> : selectedSell && <SellItemDetails item={selectedSell} busy={busy} onSell={() => setConfirmSellId(selectedSell.instanceId)} />;

    return <main className={styles.pageRoot}>
        <div className={styles.container}>
            <header className={styles.pageHeader}>
                <div><span className={styles.eyebrow}><UiText text="THE MARKETPLACE" /></span><h1><UiText text="Shop" /></h1><p><UiText text="Supplies for the road. Equipment for the next journey." /></p></div>
                <div className={styles.goldSummary}><span><UiText text="YOUR GOLD" /></span><strong>{player ? formatGold(player.balance) : <UiText text="Loading…" />}</strong></div>
            </header>
            <div className={styles.toolbar}>
                <Tabs id="shop-mode" tabs={[{ id: "buy", label: "Buy" }, { id: "sell", label: "Sell" }]} activeId={tab} onChange={id => selectTab(id as ShopTab)} label="Shop mode" />
                {tab === "buy" && <div className={styles.categoryTabs} role="group" aria-label="Shop categories">
                    {(["all", "resource", "scroll", "blueprint", "preview"] as ShopCategory[]).filter(key => key === "all" || key === "preview" || catalog.some(item => item.category === key)).map(key =>
                        <button key={key} type="button" aria-pressed={category === key} onClick={() => { setCategory(key); setMobileDetailsOpen(false); }}><UiText text={CATEGORY_LABELS[key]} />{key === "preview" && <small> <UiText text="Soon" /></small>}</button>)}
                </div>}
            </div>
            {error && <StatusMessage tone="error" className={styles.message}>{error} <button type="button" onClick={() => void loadData()}><UiText text="Retry" /></button></StatusMessage>}
            {notice && <StatusMessage tone="success" className={styles.message}>{notice}</StatusMessage>}
            {loading && !player && catalog.length === 0 ? <LoadingState title="Opening the market…" /> : <div className={styles.contentGrid}>
                <section className={styles.catalogPanel} aria-label={tab === "buy" ? "Shop catalog" : "Available equipment for sale"}>
                    <div className={styles.sectionHeading}><div><span className={styles.eyebrow}><UiText text={tab === "buy" ? "BROWSE GOODS" : "YOUR INVENTORY"} /></span><h2><UiText text={tab === "buy" ? CATEGORY_LABELS[category] : "Sell Equipment"} /></h2></div><span>{tab === "buy" ? visibleCatalog.length : sellItems.length} <UiText text="items" /></span></div>
                    {tab === "buy" && category === "scroll" && <div className={styles.categoryArt}><span><UiText text="Scriptorium" /></span><small><UiText text="Scrolls require a Library." /></small></div>}
                    {tab === "buy" && category === "preview" ? <div className={styles.previewSection}><div className={styles.previewArt}><span><UiText text="Green Sets" /></span><strong><UiText text="Coming Soon" /></strong></div><p><UiText text="Equipment sets cannot be purchased yet." /></p><div className={styles.previewNames}>{GREEN_SET_NAMES.map(name => <span key={name}>{name}</span>)}</div></div> : tab === "buy" ? visibleCatalog.length ? <div className={styles.productGrid}>{visibleCatalog.map(item => <ShopItemCard key={item.type} item={item} selected={selectedBuy?.type === item.type} onSelect={() => { setSelectedBuyId(item.type); showDetails(); }} />)}</div> : <EmptyState title="No goods in this category" description="The current server catalog has no items here." /> : !equipment.inventory ? <ErrorState title="Sell inventory unavailable" description="Refresh to load the server's free equipment list." action={<button type="button" className={styles.secondaryAction} onClick={() => void loadData()}><UiText text="Retry" /></button>} /> : sellItems.length ? <div className={styles.productGrid}>{sellItems.map(item => <SellItemCard key={item.instanceId} item={item} selected={selectedSell?.instanceId === item.instanceId} onSelect={() => { setSelectedSellId(item.instanceId); showDetails(); }} />)}</div> : <EmptyState title="No unequipped equipment available" description="Equipped items must be removed in Inventory before they can be sold." />}
                </section>
                <aside className={styles.detailsPanel} aria-label="Selected item details">
                    <div className={styles.detailHeading}><span className={styles.eyebrow}>{t(tab === "buy" ? "MAKE A PURCHASE" : "SELL AN ITEM")}</span><h2><UiText text="Item details" /></h2></div>
                    {mobileDetailsOpen ? null : detail || <p className={styles.emptyDetail}><UiText text={category === "preview" ? "Green sets are coming soon." : "Select an item to view its details."} /></p>}
                </aside>
            </div>}
        </div>
        <MobileDetails open={mobileDetailsOpen && Boolean(detail)} onClose={closeDetails}>{mobileDetailsOpen ? detail : null}</MobileDetails>
        <ConfirmDialog open={Boolean(confirmItem)} title="Sell equipment?" description={confirmItem ? `${t("Sell")} ${t(confirmItem.name)} ${t("for")} ${formatGold(confirmItem.sellPrice!)}? ${t("This removes this exact item permanently.")}` : ""} confirmLabel="Confirm sale" tone="danger" busy={busy} onConfirm={() => void sell()} onCancel={() => { if (!busyRef.current) setConfirmSellId(null); }} />
    </main>;
}
