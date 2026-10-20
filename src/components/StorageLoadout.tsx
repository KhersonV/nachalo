"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { API_BASE } from "@/utils/serviceUrls";
import styles from "@/styles/StorageLoadout.module.css";

type StoredItem = { name?: string; item_name?: string; item_count?: number; image?: string };
type StorageState = {
  inventory: Record<string, StoredItem>;
  loadout: Record<string, number>;
  eligibleKeys: string[];
  balance: number;
  equipment: { name: string; type: string; status: string }[];
};

const copy = {
  en: { storage: "Storage", loadout: "Loadout for next match", available: "In Storage", selected: "Taking", remaining: "Remaining", empty: "Storage is empty.", equipment: "Equipment", save: "Save loadout", saving: "Saving…", saved: "Loadout saved", loading: "Loading Storage…", unavailable: "Storage unavailable", carried: "Items move only when a match is created. Leaving the queue costs nothing.", gold: "Gold", carriedEquipment: "Equipped gear stays with the hero.", equipped: "equipped", stored: "stored" },
  ru: { storage: "Склад", loadout: "Набор для следующего матча", available: "На складе", selected: "Взять", remaining: "Останется", empty: "Склад пуст.", equipment: "Экипировка", save: "Сохранить набор", saving: "Сохранение…", saved: "Набор сохранён", loading: "Загрузка склада…", unavailable: "Склад недоступен", carried: "Предметы переходят в матч только при его создании. Отмена поиска ничего не списывает.", gold: "Золото", carriedEquipment: "Надетая экипировка остаётся с героем.", equipped: "надето", stored: "на складе" },
  de: { storage: "Lager", loadout: "Ausrüstung für das nächste Match", available: "Im Lager", selected: "Mitnehmen", remaining: "Verbleibt", empty: "Lager ist leer.", equipment: "Ausrüstung", save: "Auswahl speichern", saving: "Speichern…", saved: "Auswahl gespeichert", loading: "Lager wird geladen…", unavailable: "Lager nicht verfügbar", carried: "Gegenstände werden erst bei Matchbeginn übertragen. Warteschlange verlassen kostet nichts.", gold: "Gold", carriedEquipment: "Angelegte Ausrüstung bleibt beim Helden.", equipped: "angelegt", stored: "im Lager" },
};

export default function StorageLoadout({ allowSelection = false, onReadyChange }: { allowSelection?: boolean; onReadyChange?: (ready: boolean) => void }) {
  const { user } = useAuth();
  const { language } = useGameLanguage();
  const t = copy[language];
  const [storage, setStorage] = useState<StorageState | null>(null);
  const [selected, setSelected] = useState<Record<string, number>>({});
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async (signal: AbortSignal) => {
    if (!user?.token) return;
    const response = await fetch(`${API_BASE}/game/storage`, { headers: { Authorization: `Bearer ${user.token}` }, signal });
    if (!response.ok) throw new Error(await response.text());
    const next = await response.json() as StorageState;
    if (signal.aborted) return;
    setError("");
    const valid = Object.entries(next.loadout ?? {}).every(([key, count]) => next.eligibleKeys?.includes(key) && count > 0 && count <= (next.inventory[key]?.item_count ?? 0));
    setStorage(next);
    setSelected(next.loadout ?? {});
    setSaved(valid);
    onReadyChange?.(valid);
  }, [user?.token, onReadyChange]);

  useEffect(() => {
    const controller = new AbortController();
    setStorage(null);
    setSaved(false);
    onReadyChange?.(false);
    if (!user?.token) return () => controller.abort();
    void reload(controller.signal).catch(reason => { if (!controller.signal.aborted) { setError(String(reason)); onReadyChange?.(false); } });
    return () => controller.abort();
  }, [reload, onReadyChange, user?.token]);

  const entries = useMemo(() => Object.entries(storage?.inventory ?? {}).filter(([, item]) => (item.item_count ?? 0) > 0).sort(([a], [b]) => a.localeCompare(b)), [storage]);
  const eligible = useMemo(() => new Set(storage?.eligibleKeys ?? []), [storage]);
  const setCount = (key: string, count: number) => {
    const max = storage?.inventory[key]?.item_count ?? 0;
    const value = Math.max(0, Math.min(max, Number.isFinite(count) ? Math.floor(count) : 0));
    setSelected(current => { const next = { ...current }; if (value) next[key] = value; else delete next[key]; return next; });
    setSaved(false);
    onReadyChange?.(false);
  };
  const save = async () => {
    if (!user?.token || busy) return;
    setBusy(true); setError("");
    try {
      const response = await fetch(`${API_BASE}/game/storage`, { method: "PUT", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ loadout: selected }) });
      if (!response.ok) throw new Error(await response.text());
      const next = await response.json() as StorageState;
      setStorage(next); setSelected(next.loadout ?? {}); setSaved(true); onReadyChange?.(true);
    } catch (reason) { setError(String(reason)); onReadyChange?.(false); } finally { setBusy(false); }
  };

  return <section className={styles.panel} aria-label={t.storage}>
    <div className={styles.heading}><h3>{t.storage}</h3><span>{t.gold}: {storage?.balance ?? "…"}</span></div>
    {!storage && !error && <p>{t.loading}</p>}
    {error && <p role="alert" className={styles.error}>{t.unavailable}: {error}</p>}
    {storage && <>
      {allowSelection && <><h4>{t.loadout}</h4><p>{t.carried}</p></>}
      {entries.length === 0 && <p>{t.empty}</p>}
      <div className={styles.list}>{entries.map(([key, item]) => {
        const count = item.item_count ?? 0;
        const taking = Math.min(count, selected[key] ?? 0);
        return <div className={styles.item} key={key}>
          {item.image && <img src={item.image} alt="" />}
          <div><strong>{item.name ?? item.item_name ?? key}</strong><small>{key.split("_")[0]} · {t.available}: {count}</small>{allowSelection && eligible.has(key) && <small>{t.remaining}: {count - taking}</small>}</div>
          {allowSelection && eligible.has(key) && <label>{t.selected}<input type="number" min={0} max={count} value={taking} onChange={event => setCount(key, Number(event.target.value))} disabled={busy} /></label>}
        </div>;
      })}</div>
      <h4>{t.equipment}</h4><p>{t.carriedEquipment}</p>
      <div className={styles.list}>{(storage.equipment ?? []).map((item, index) => <div className={styles.item} key={`${item.name}-${index}`}><strong>{item.name}</strong><small>{item.type} · {item.status === "equipped" ? t.equipped : item.status === "inventory" ? t.stored : item.status}</small></div>)}</div>
      {allowSelection && <div className={styles.footer}><button type="button" onClick={() => void save()} disabled={busy || saved}>{busy ? t.saving : t.save}</button>{saved && <span>{t.saved}</span>}</div>}
    </>}
  </section>;
}
