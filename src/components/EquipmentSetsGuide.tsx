"use client";

import React from "react";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
import { equipmentRarityName } from "@/utils/equipmentAssets";
import { professionRankName } from "@/utils/professionProgression";
import { BONUS_FIELDS } from "@/features/equipment/equipmentView";
import type { EquipmentBonuses } from "@/types/equipment";
import type { CraftRecipe } from "./ProfessionCrafting";
import { Card, EmptyState } from "./ui";
import styles from "@/styles/DropGuidePage.module.css";

export type EquipmentGuideSet = {
 code: string; name: string; heroClass: string; rarity: string; source: string;
 items: { code: string; templateId: number; name: string; slot: string; image: string; bonuses: EquipmentBonuses }[];
};
type DropMonster = { code: string; name: string; equipmentChance: number };
const slotNames: Record<string, string> = { main_hand: "Main Hand", off_hand: "Off Hand", helmet: "Helmet", chest: "Chest", pants: "Pants", gloves: "Gloves", boots: "Boots" };

export default function EquipmentSetsGuide({ sets, recipes, monsters, heroClass }: { sets: EquipmentGuideSet[]; recipes: CraftRecipe[]; monsters: DropMonster[]; heroClass: string }) {
 const { language } = useGameLanguage();
 const t = (text: string) => uiText(language, text);
 const [rarity, setRarity] = React.useState("all");
 const [setCode, setSetCode] = React.useState("all");
 React.useEffect(() => setSetCode("all"), [heroClass, rarity]);
 const eligible = sets.filter(set => (heroClass === "all" || set.heroClass === heroClass) && (rarity === "all" || set.rarity === rarity));
 const visible = eligible.filter(set => setCode === "all" || set.code === setCode);
 return <section className={styles.gatheringSection} aria-label={t("Equipment sets")}>
  <h2>{t("Equipment sets")}</h2>
  <p>{t("The class filter above also applies to equipment sets.")}</p>
  <div className={styles.filters}>
   <label>{t("Rarity")} <select value={rarity} onChange={event => setRarity(event.target.value)}><option value="all">{t("All rarities")}</option>{["green", "blue"].map(value => <option key={value} value={value}>{t(equipmentRarityName(value))}</option>)}</select></label>
   <label>{t("Equipment Set")} <select value={setCode} onChange={event => setSetCode(event.target.value)}><option value="all">{t("All sets")}</option>{eligible.map(set => <option key={set.code} value={set.code}>{set.name}</option>)}</select></label>
  </div>
  {visible.map(set => <section key={set.code} className={styles.gatheringCategory} data-rarity={set.rarity}>
   <h3>{set.name} · {t(set.heroClass)} · {t(equipmentRarityName(set.rarity))}</h3>
   <p>{t("Additional pockets from items")}: +{set.items.reduce((total, item) => total + (item.bonuses.pocketBonus ?? 0), 0)}. {t("Pocket bonuses apply individually; a full set is not required.")}</p>
   <p>{t(set.source === "monster" ? "Dropped by monsters" : "Crafted through professions")}</p>
   {set.source === "monster" && <p>{monsters.map(monster => `${monster.name}: ${(monster.equipmentChance * 100).toLocaleString(language, { maximumFractionDigits: 2 })}%`).join(" · ")}<br />{t("This is the equipment roll chance; each item is selected uniformly from the shared pool.")}</p>}
   <div className={styles.grid}>{set.items.map(item => {
    const recipe = recipes.find(value => value.code === item.code);
    return <Card as="article" key={item.code} className={styles.card} data-rarity={set.rarity}>
     <img className={styles.materialImage} src={item.image} alt={item.name} loading="lazy" />
     <h4>{t(item.name)}</h4><p>{t(slotNames[item.slot] || item.slot)}</p>
     <small>Template ID: {item.templateId}</small>
     <ul>{BONUS_FIELDS.filter(field => item.bonuses[field.key]).map(field => <li key={field.key}>{t(field.label)}: {field.key === "pocketBonus" ? "+" : ""}{item.bonuses[field.key]}</li>)}{Boolean(item.bonuses.maxEnergy) && <li>{t("Max Energy")}: {item.bonuses.maxEnergy}</li>}</ul>
     {set.source === "craft" && (recipe ? <div>
      <p>{t(recipe.profession)} · {t("Required level")}: {recipe.minLevel} ({t(professionRankName(recipe.minLevel))}) · ×{recipe.output}</p>
      <ul>{recipe.ingredients.map(ingredient => <li key={ingredient.code}><img className={styles.recipeIcon} src={ingredient.image} alt="" loading="lazy" /> {t(ingredient.name)} ×{ingredient.quantity}</li>)}</ul>
     </div> : <p>{t("Recipe temporarily unavailable")}</p>)}
    </Card>;
   })}</div>
  </section>)}
  {visible.length === 0 && <EmptyState title={t("No matching equipment sets")} />}
 </section>;
}
