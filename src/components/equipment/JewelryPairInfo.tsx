"use client";
import type { EquipmentItem, EquipmentSlot } from "@/types/equipment";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";
export default function JewelryPairInfo({ item, equipped }: { item: EquipmentItem; equipped: Partial<Record<EquipmentSlot, EquipmentItem>> }) {
 const { language } = useGameLanguage();
 if (!item.jewelrySet) return null;
 const { name, bonus } = item.jewelrySet;
 const progress = [equipped.ring, equipped.amulet].filter(v => v && v.setCode === item.setCode && v.classRestriction === item.classRestriction).length;
 return <p>{uiText(language, "Jewelry set")}: {name} · {progress}/2<br />2/2: {[bonus.attack ? `ATK +${bonus.attack}` : "", bonus.defense ? `DEF +${bonus.defense}` : "", bonus.maxHealth ? `Max HP +${bonus.maxHealth}` : ""].filter(Boolean).join(" · ")}</p>;
}
