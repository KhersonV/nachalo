"use client";

import React from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import { professionRankName } from "@/utils/professionProgression";
import styles from "@/styles/ProfessionsPage.module.css";
import { API_BASE } from "@/utils/serviceUrls";

export type CraftRecipe = {
 code: string; name: string; ru?: string; de?: string; slot?: string; heroClass?: string; rarity?: string; jewelryProgress?: number; jewelrySet?: {name: string; bonus: {attack?:number;defense?:number;maxHealth?:number}; required:number}; profession: string; minLevel: number; output: number; xp: number;
 tradable: boolean; image: string; category: string; effect: string; description: string; available: boolean; reason?: string;
 durability?: number; pocketBonus?: number; bonuses?: { attack: number; defense: number; maxHealth: number; mobility: number; agility: number };
 ingredients: { code: string; resourceId: number; name: string; quantity: number; owned: number; image: string }[];
};
export default function ProfessionCrafting({ characterId, profession, currentLevel, maxLevel, onCrafted, onBusyChange }: { characterId: number; profession: string; currentLevel: number; maxLevel: number; onCrafted: () => Promise<void>; onBusyChange: (busy: boolean) => void }) {
 const { user } = useAuth(); const { language } = useGameLanguage();
 const t = (key: string) => uiText(language, key);
 const [category, setCategory] = React.useState("all");
 const [slot, setSlot] = React.useState(""); const [rarity, setRarity] = React.useState(""); const [availableOnly, setAvailableOnly] = React.useState(false);
 const recipeName = (r: CraftRecipe) => language === "ru" ? r.ru || t(r.name) : language === "de" ? r.de || t(r.name) : r.name;
 const [recipes, setRecipes] = React.useState<CraftRecipe[]>([]);
 const [error, setError] = React.useState(""); const [loading, setLoading] = React.useState(true);
 const [busy, setBusy] = React.useState(""); const [notice, setNotice] = React.useState("");
 const lock = React.useRef(false); const pending = React.useRef<{ code: string; commandId: string } | null>(null);
 const load = React.useCallback(async (signal?: AbortSignal) => {
  if (!user?.token) return;
  const response = await fetch(`${API_BASE}/game/professions/recipes?characterId=${characterId}`, { headers: { Authorization: `Bearer ${user.token}` }, cache: "no-store", signal });
  if (!response.ok) throw new Error("profession_unavailable");
  const result: { recipes: CraftRecipe[] } = await response.json();
  if (!signal?.aborted) { setRecipes(result.recipes); setError(""); }
 }, [characterId, user?.token]);
 React.useEffect(() => { const controller = new AbortController(); setLoading(true);
  void load(controller.signal).catch(reason => { if (!controller.signal.aborted) setError(String(reason)); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
  const refresh = () => { if (!lock.current) void load(controller.signal).catch(reason => { if (!controller.signal.aborted) setError(String(reason)); }); };
  window.addEventListener("focus", refresh);
  return () => { controller.abort(); window.removeEventListener("focus", refresh); };
 }, [load]);
 const craft = async (recipe: CraftRecipe) => {
  if (lock.current || !recipe.available || !user?.token) return;
  lock.current = true; onBusyChange(true); setBusy(recipe.code); setError(""); setNotice("");
  try {
   let attempt = pending.current;
   if (!attempt || attempt.code !== recipe.code) {
    attempt = { code: recipe.code, commandId: globalThis.crypto?.randomUUID?.() ?? `craft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}` };
    pending.current = attempt;
   }
   const response = await fetch(`${API_BASE}/game/professions/craft`, { method: "POST", headers: { Authorization: `Bearer ${user.token}`, "Content-Type": "application/json" }, body: JSON.stringify({ characterId, recipeCode: recipe.code, quantity: 1, commandId: attempt.commandId }) });
   const result: { error?: string; level: number } = await response.json();
   if (!response.ok) throw new Error(result.error || "profession_unavailable");
   pending.current = null; setNotice(`${t("Crafted")}: ${recipeName(recipe)} ×${recipe.output}${result.level > currentLevel ? ` · ${t("Profession level increased")}: ${result.level} (${t(professionRankName(result.level))})` : ""}`);
   await load(); await onCrafted();
  } catch (reason) { setError(reason instanceof Error ? reason.message : String(reason)); }
  finally { lock.current = false; onBusyChange(false); setBusy(""); }
 };
 return <section className={styles.production}>
  <button type="button" disabled={!!busy} onClick={() => void load().catch(reason => setError(String(reason)))}>{t("Refresh")}</button>
  <p>{t("Crafting uses permanent Storage materials. Match loot becomes available only after extraction.")}</p>
  {loading && <p role="status">{t("Working…")}</p>}
  {error && <p role="alert" className={styles.error}>{t(error)} <button type="button" onClick={() => void load().catch(reason => setError(String(reason)))}>{t("Refresh")}</button></p>}
  {notice && <p role="status">{notice}</p>}
  <div>
   <button type="button" aria-pressed={category === "all"} disabled={!!busy} onClick={() => setCategory("all")}>{t("All products")}</button>
   <button type="button" aria-pressed={category === "jewelry"} disabled={!!busy} onClick={() => setCategory("jewelry")}>{t("Jewelry")}</button>
   {category === "jewelry" && <>
    <select aria-label={t("Slot")} value={slot} onChange={e => setSlot(e.target.value)}><option value="">{t("All jewelry")}</option><option value="ring">{t("Rings")}</option><option value="amulet">{t("Amulets")}</option></select>
    <select aria-label={t("Rarity")} value={rarity} onChange={e => setRarity(e.target.value)}><option value="">{t("All rarities")}</option><option value="green">{t("Uncommon")}</option><option value="blue">{t("Rare")}</option></select>
    <label><input type="checkbox" checked={availableOnly} onChange={e => setAvailableOnly(e.target.checked)} />{t("Available materials")}</label>
   </>}
  </div>
  <div className={styles.recipeGrid}>{recipes.filter(recipe => (category === "jewelry" ? recipe.category === "jewelry" && (!slot || recipe.slot === slot) && (!rarity || recipe.rarity === rarity) && (!availableOnly || recipe.ingredients.every(v => v.owned >= v.quantity)) : recipe.profession === profession || recipe.category === "jewelry")).map(recipe => <article className={styles.recipe} key={recipe.code}>
   <header><CraftImage src={recipe.image} /><div><h3>{recipeName(recipe)}</h3><small>{t("Required level")}: {recipe.minLevel} ({t(professionRankName(recipe.minLevel))}) · +{currentLevel < maxLevel ? recipe.xp : 0} XP / {t("Batch")} · ×{recipe.output}</small></div></header>
   <p>{t(recipe.description)}</p><small>{t(recipe.tradable ? "Tradable" : "Not tradable.")}</small>
   {(recipe.category === "equipment" || recipe.category === "jewelry") && <p>{t(recipe.rarity === "green" ? "Uncommon" : "Rare")} · HP +{recipe.bonuses?.maxHealth} · ATK +{recipe.bonuses?.attack} · DEF +{recipe.bonuses?.defense} · MOB +{recipe.bonuses?.mobility} · Reflex +{recipe.bonuses?.agility} · {t("Durability")}: {recipe.durability}{(recipe.pocketBonus ?? 0) > 0 && <> · {t("Additional pockets")}: +{recipe.pocketBonus}</>}</p>}
   {recipe.jewelrySet && <p>{t(recipe.heroClass || "")} · {t(recipe.slot || "")} · {t("Jewelry set")}: {recipe.jewelrySet.name} · {recipe.jewelryProgress ?? 0}/2<br />2/2: {[recipe.jewelrySet.bonus.attack ? `ATK +${recipe.jewelrySet.bonus.attack}` : "", recipe.jewelrySet.bonus.defense ? `DEF +${recipe.jewelrySet.bonus.defense}` : "", recipe.jewelrySet.bonus.maxHealth ? `Max HP +${recipe.jewelrySet.bonus.maxHealth}` : ""].filter(Boolean).join(" · ")}</p>}
   <ul>{recipe.ingredients.map(ingredient => <li key={ingredient.code} data-short={ingredient.owned < ingredient.quantity}><CraftImage src={ingredient.image} /><span>{t(ingredient.name)}</span><strong>{ingredient.owned} / {ingredient.quantity}{ingredient.owned < ingredient.quantity && <> · {t("Missing")}: {ingredient.quantity - ingredient.owned}</>}</strong></li>)}</ul>
   <button type="button" disabled={!recipe.available || !!busy} onClick={() => void craft(recipe)}>{busy === recipe.code ? t("Working…") : t("Craft")}</button>
   {!recipe.available && <small>{t(recipe.reason || "profession_unavailable")}</small>}
  </article>)}</div>
 </section>;
}

function CraftImage({ src }: { src?: string }) {
 const [failed,setFailed] = React.useState(false);
 React.useEffect(() => setFailed(false),[src]);
 return src && !failed ? <img src={src} alt="" loading="lazy" onError={() => setFailed(true)} /> : <span aria-hidden="true">✦</span>;
}
