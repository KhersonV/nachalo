"use client";

import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import { API_BASE } from "@/utils/serviceUrls";
import {
    pocketsUsed,
    eligibleItems,
    nextLoadout,
    normalizedLoadout,
    remainingCount,
    SerializedLoadoutSaver,
    type QuantityAction,
    type SaveStatus,
    type StorageState,
} from "@/features/storage/loadoutModel";
import styles from "@/styles/StorageLoadout.module.css";
import { resourceImage } from "@/utils/resourceAssets";

export type StorageLoadoutHandle = { ensureSaved: () => Promise<void>; retry: () => Promise<void> };

type Props = {
    allowSelection?: boolean;
    collapsible?: boolean;
    locked?: boolean;
    forceClose?: boolean;
    languageOverride?: "en" | "ru" | "de";
    compact?: boolean;
    onReadyChange?: (ready: boolean) => void;
    onAvailabilityChange?: (available: boolean) => void;
    onStatusChange?: (status: SaveStatus, error?: string) => void;
};

const copy = {
    en: { title: "Match loadout", configure: "Configure loadout", close: "Close", available: "Available", selected: "In loadout", empty: "No eligible consumables in Storage.", noneSelected: "No items selected.", loading: "Loading loadout…", saved: "Saved", saving: "Saving…", failed: "Save failed", retry: "Retry", carried: "Items move to the match only when it is created.", equipment: "Equipment", equipped: "equipped", stored: "stored", gold: "Gold", inventory: "Storage" },
    ru: { title: "Набор для матча", configure: "Настроить набор", close: "Закрыть", available: "Доступно", selected: "В наборе", empty: "На складе нет доступных расходников.", noneSelected: "Предметы не выбраны.", loading: "Загрузка набора…", saved: "Сохранено", saving: "Сохранение…", failed: "Ошибка сохранения", retry: "Повторить", carried: "Предметы переносятся в матч только при его создании.", equipment: "Экипировка", equipped: "надето", stored: "на складе", gold: "Золото", inventory: "Склад" },
    de: { title: "Matchausrüstung", configure: "Ausrüstung anpassen", close: "Schließen", available: "Verfügbar", selected: "Im Matchset", empty: "Keine zulässigen Verbrauchsgegenstände im Lager.", noneSelected: "Keine Gegenstände ausgewählt.", loading: "Matchset wird geladen…", saved: "Gespeichert", saving: "Wird gespeichert…", failed: "Speichern fehlgeschlagen", retry: "Erneut versuchen", carried: "Gegenstände werden erst bei Matchbeginn übertragen.", equipment: "Ausrüstung", equipped: "angelegt", stored: "im Lager", gold: "Gold", inventory: "Lager" },
};

const ACTIONS: QuantityAction[] = [-5, -1, 1, 5, "zero", "max"];

const StorageLoadout = forwardRef<StorageLoadoutHandle, Props>(function StorageLoadout(
    { allowSelection = false, collapsible = false, locked = false, forceClose = false, languageOverride, compact = false, onReadyChange, onAvailabilityChange, onStatusChange },
    ref,
) {
    const { user } = useAuth();
    const { language } = useGameLanguage();
    const locale = languageOverride ?? language;
    const t = copy[locale];
    const translate = (source: string) => uiText(locale, source);
    const [storage, setStorage] = useState<StorageState | null>(null);
    const [selected, setSelected] = useState<Record<string, number>>({});
    const [status, setStatus] = useState<SaveStatus>("saving");
    const [error, setError] = useState("");
    const [open, setOpen] = useState(false);
    const selectedRef = useRef<Record<string, number>>({});
    const tokenRef = useRef(user?.token);
    const capacityRef = useRef(6);
    const stacksRef = useRef<Record<string, number>>({});
    const callbacksRef = useRef({ onReadyChange, onAvailabilityChange, onStatusChange });
    const saverRef = useRef<SerializedLoadoutSaver | null>(null);
    const dialogRef = useRef<HTMLDialogElement>(null);
    const openerRef = useRef<HTMLButtonElement>(null);
    const closeRef = useRef<HTMLButtonElement>(null);
    tokenRef.current = user?.token;
    callbacksRef.current = { onReadyChange, onAvailabilityChange, onStatusChange };

    if (!saverRef.current) {
        saverRef.current = new SerializedLoadoutSaver(
            async (loadout) => {
                const token = tokenRef.current;
                if (!token) throw new Error("Session unavailable");
                const response = await fetch(`${API_BASE}/game/storage`, {
                    method: "PUT",
                    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
                    body: JSON.stringify({ loadout }),
                });
                if (!response.ok) throw new Error((await response.text()) || "Storage unavailable");
            },
            (nextStatus, reason) => {
                setStatus(nextStatus);
                setError(reason?.message ?? "");
                callbacksRef.current.onReadyChange?.(nextStatus === "saved" && pocketsUsed(selectedRef.current, stacksRef.current) <= capacityRef.current);
                callbacksRef.current.onStatusChange?.(nextStatus, reason?.message);
            },
        );
    }

    const reload = useCallback(async (signal: AbortSignal) => {
        const token = tokenRef.current;
        if (!token) return;
        const response = await fetch(`${API_BASE}/game/storage`, {
            headers: { Authorization: `Bearer ${token}` },
            cache: "no-store",
            signal,
        });
        if (!response.ok) throw new Error((await response.text()) || "Storage unavailable");
        const next = await response.json() as StorageState;
        if (signal.aborted) return;
        const normalized = normalizedLoadout(next);
        const original = next.loadout ?? {};
        capacityRef.current = next.pocketCapacity ?? 6;
        stacksRef.current = next.pocketStacks ?? {};
        selectedRef.current = normalized;
        setStorage(next);
        setSelected(normalized);
        saverRef.current!.initialize(original);
        if (allowSelection && JSON.stringify(normalized) !== JSON.stringify(original)) saverRef.current!.change(normalized);
        callbacksRef.current.onAvailabilityChange?.(true);
    }, [allowSelection]);

    const retryStorage = async (): Promise<void> => {
        if (storage) {
            await saverRef.current!.flush();
            return;
        }
        setError("");
        setStatus("saving");
        callbacksRef.current.onStatusChange?.("saving");
        try {
            await reload(new AbortController().signal);
        } catch (reason) {
            setError(String(reason));
            setStatus("error");
            callbacksRef.current.onStatusChange?.("error", String(reason));
            throw reason;
        }
    };

    useImperativeHandle(ref, () => ({
        ensureSaved: async () => {
            if (!storage) throw new Error("Loadout unavailable");
            await saverRef.current!.flush();
            await reload(new AbortController().signal);
            if (pocketsUsed(selectedRef.current, stacksRef.current) > capacityRef.current) throw new Error(uiText(locale, "Reduce loadout to pocket capacity. Items remain in Storage."));
            await saverRef.current!.flush();
        },
        retry: retryStorage,
    }), [storage, reload, locale]);

    useEffect(() => {
        const controller = new AbortController();
        setStorage(null);
        setError("");
        setStatus("saving");
        callbacksRef.current.onAvailabilityChange?.(false);
        callbacksRef.current.onReadyChange?.(false);
        callbacksRef.current.onStatusChange?.("saving");
        if (user?.token) {
            void reload(controller.signal).catch((reason) => {
                if (controller.signal.aborted) return;
                setError(String(reason));
                setStatus("error");
                callbacksRef.current.onAvailabilityChange?.(false);
                callbacksRef.current.onStatusChange?.("error", String(reason));
            });
        }
        return () => { controller.abort(); saverRef.current?.dispose(); };
    }, [reload, user?.token]);

    useEffect(() => {
        const controller = new AbortController();
        // Refresh capacity without resetting an unsaved selection. The server
        // remains authoritative for writes and match creation in other tabs.
        const refreshCapacity = () => {
            const token = tokenRef.current;
            if (!token) return;
            void fetch(`${API_BASE}/game/storage`, {
                headers: { Authorization: `Bearer ${token}` },
                cache: "no-store",
                signal: controller.signal,
            }).then(async response => {
                if (!response.ok) throw new Error((await response.text()) || "Storage unavailable");
                const next = await response.json() as StorageState;
                if (controller.signal.aborted) return;
                capacityRef.current = next.pocketCapacity ?? 6;
                stacksRef.current = next.pocketStacks ?? {};
                setStorage(next);
                callbacksRef.current.onReadyChange?.(status === "saved" && pocketsUsed(selectedRef.current, stacksRef.current) <= capacityRef.current);
            }).catch(reason => { if (!controller.signal.aborted) setError(String(reason)); });
        };
        window.addEventListener("nachalo:equipment-changed", refreshCapacity);
        window.addEventListener("focus", refreshCapacity);
        return () => {
            controller.abort();
            window.removeEventListener("nachalo:equipment-changed", refreshCapacity);
            window.removeEventListener("focus", refreshCapacity);
        };
    }, [status, user?.token]);

    useEffect(() => {
        const dialog = dialogRef.current;
        if (!dialog) return;
        if (open && !dialog.open) { dialog.showModal(); closeRef.current?.focus(); }
        if (!open && dialog.open) { dialog.close(); openerRef.current?.focus(); }
    }, [open]);

    useEffect(() => { if (forceClose) setOpen(false); }, [forceClose]);

    const entries = useMemo(() => storage ? eligibleItems(storage) : [], [storage]);
    const capacity = storage?.pocketCapacity ?? 6;
    const pocketStacks = storage?.pocketStacks ?? {};
    const used = pocketsUsed(selected, pocketStacks);
    const maxTaking = (key: string, total: number) => Math.max(0, Math.min(total, (capacity - used + Math.ceil((selected[key] ?? 0) / (pocketStacks[key] ?? 1))) * (pocketStacks[key] ?? 1)));
    const totalSelected = Object.values(selected).reduce((sum, count) => sum + count, 0);
    const selectedEntries = entries.filter(([key]) => (selected[key] ?? 0) > 0);
    const filledPockets = selectedEntries.flatMap(([key, item]) => Array.from({ length: Math.ceil((selected[key] ?? 0) / (pocketStacks[key] ?? 1)) }, (_, n) => ({ key, item, count: Math.min(pocketStacks[key] ?? 1, (selected[key] ?? 0) - n * (pocketStacks[key] ?? 1)) })));
    const summary = selectedEntries.slice(0, compact ? 3 : undefined)
        .map(([key, item]) => `${translate(item.name ?? item.item_name ?? key)} ×${selected[key]}`)
        .join(", ");
    const compactSummary = selectedEntries.length > 3 ? `${summary} +${selectedEntries.length - 3} ${translate("more")}` : summary;

    const changeCount = (key: string, action: QuantityAction, total: number) => {
        if (locked || !allowSelection) return;
        const next = nextLoadout(selectedRef.current, key, action, (typeof action === "number" && action < 0) || action === "zero" ? total : Math.max(selectedRef.current[key] ?? 0, maxTaking(key, total)));
        if (next === selectedRef.current) return;
        selectedRef.current = next;
        setSelected(next);
        saverRef.current!.change(next);
    };

    const retry = () => { void retryStorage().catch(() => undefined); };
    const refreshPockets = () => { if (status !== "saved") return; void reload(new AbortController().signal).catch(reason => setError(String(reason))); };
    const statusText = status === "saved" ? t.saved : status === "saving" ? t.saving : t.failed;
    const compactContent = <>
        <span className={styles.compactTopline}><span>{t.title}</span><span className={status === "error" ? styles.error : status === "saved" ? styles.ready : styles.needsSave} role="status">{error ? t.failed : storage ? statusText : t.loading}</span></span>
        <strong className={styles.compactSummary}>{storage ? (compactSummary || (entries.length ? t.noneSelected : t.empty)) : (error || t.loading)}</strong>
        <span className={styles.compactFooter}>{translate("Pockets")}: {used}/{capacity} · {t.selected}: {totalSelected} {allowSelection && <span>{t.configure} ↗</span>}</span>
    </>;

    if (!collapsible) {
        return <section className={styles.panel} aria-label={t.inventory}>
            <div className={styles.heading}><h3>{t.inventory}</h3><span>{t.gold}: {storage?.balance ?? "…"}</span></div>
            {!storage && <p>{error || t.loading}</p>}
            {storage && <>
                <div className={styles.list}>{Object.entries(storage.inventory ?? {}).map(([key, item]) => <div className={styles.item} key={key}><strong>{translate(item.name ?? item.item_name ?? key)}</strong><small>×{item.item_count ?? 0}</small></div>)}</div>
                <h4>{t.equipment}</h4>
                <div className={styles.list}>{(storage.equipment ?? []).map((item, index) => <div className={styles.item} key={`${item.name}-${index}`}><strong>{translate(item.name)}</strong><small>{translate(item.type)} · {item.status === "equipped" ? t.equipped : t.stored}</small></div>)}</div>
            </>}
        </section>;
    }

    return <section className={compact ? styles.compactPanel : styles.panel} aria-label={t.title}>
        {compact ? allowSelection ? <button ref={openerRef} className={styles.compactTrigger} type="button" onClick={() => setOpen(true)} disabled={!storage || locked}>{compactContent}</button> : <div className={`${styles.compactTrigger} ${styles.compactReadOnly}`}>{compactContent}</div> : <>
            <div className={styles.heading}><h3>{t.title}</h3><span className={status === "error" ? styles.error : status === "saved" ? styles.ready : styles.needsSave} role="status">{error ? t.failed : storage ? statusText : t.loading}</span></div>
            <p className={styles.loadoutSummary}>{storage ? (summary || (entries.length ? t.noneSelected : t.empty)) : (error || t.loading)}</p>
            <div className={styles.summaryFooter}><span>{translate("Pockets")}: <strong>{used}/{capacity}</strong> · {t.selected}: {totalSelected}</span><button ref={openerRef} type="button" onClick={() => setOpen(true)} disabled={!storage || locked || !allowSelection}>{t.configure}</button></div>
        </>}
        {error && <div role="alert" className={styles.saveError}>{t.failed}: {error} <button type="button" onClick={retry}>{t.retry}</button></div>}
        <dialog ref={dialogRef} className={styles.dialog} aria-labelledby="loadout-dialog-title" onCancel={(event) => { event.preventDefault(); setOpen(false); }}>
            <div className={styles.dialogHeader}><div><span className={styles.dialogEyebrow}>{translate("STORAGE")}</span><h2 id="loadout-dialog-title">{t.title}</h2><p>{t.carried}</p></div><button ref={closeRef} type="button" className={styles.closeButton} onClick={() => setOpen(false)} aria-label={t.close}>×</button></div>
            <div className={styles.dialogBody}>
                <p>{translate("Pockets")}: {used} / {capacity} <button type="button" disabled={status !== "saved" || locked} onClick={refreshPockets}>{translate("Refresh")}</button></p>
                {used > capacity && <p role="alert">{translate("Reduce loadout to pocket capacity. Items remain in Storage.")}</p>}
                <div className={styles.pockets}>{Array.from({ length: Math.max(capacity, used) }, (_, index) => {
                  const filled = filledPockets[index];
                  return <div key={index} className={styles.pocket}>{filled ? <><img src={filled.item.image} alt="" /><span>{translate(filled.item.name || filled.key)} ×{filled.count}</span></> : <span>—</span>}</div>;
                })}</div>
                {entries.length === 0 && <p className={styles.empty}>{t.empty}</p>}
                <div className={styles.eligibleGrid}>{entries.map(([key, item]) => {
                    const total = Math.max(0, item.item_count ?? 0);
                    const taking = selected[key] ?? 0;
                    const available = remainingCount(total, taking);
                    const image = key.startsWith("resource_") ? resourceImage(item.name ?? item.item_name, item.image) ?? item.image : item.image;
                    return <article key={key} className={styles.eligibleCard}>
                        <div className={styles.itemIdentity}>{image ? <img src={image} alt="" /> : <span className={styles.iconFallback} aria-hidden="true">◇</span>}<strong>{translate(item.name ?? item.item_name ?? key)}</strong></div>
                        <small>{translate("Per pocket")}: {pocketStacks[key] ?? 1}</small><div className={styles.balances}><span>{t.available} <strong>{available}</strong></span><span>{t.selected} <strong>{taking}</strong></span></div>
                        <div className={styles.quantityActions}>{ACTIONS.map((action) => {
                            const label = action === "zero" ? "0" : action === "max" ? translate("Max") : action > 0 ? `+${action}` : String(action);
                            const disabled = locked || !allowSelection || (action === "zero" || (typeof action === "number" && action < 0) ? taking === 0 : taking >= maxTaking(key, total));
                            return <button type="button" key={String(action)} aria-label={`${label} ${translate(item.name ?? item.item_name ?? key)}`} disabled={disabled} onClick={() => changeCount(key, action, total)}>{label}</button>;
                        })}</div>
                    </article>;
                })}</div>
            </div>
            <div className={styles.dialogFooter}><span className={status === "error" ? styles.error : status === "saved" ? styles.ready : styles.needsSave} role="status">{statusText}</span>{error && <span role="alert" className={styles.error}>{error} <button type="button" onClick={retry}>{t.retry}</button></span>}<button type="button" onClick={() => setOpen(false)}>{t.close}</button></div>
        </dialog>
    </section>;
});

export default StorageLoadout;
