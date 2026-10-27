"use client";

import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

const labels: Record<string, string> = {
 auraRadius: "Aura radius", auraTick: "Aura turn damage", auraExit: "Aura exit damage",
 healPercent: "Reflex healing coefficient", followupDamage: "Follow-up damage",
 criticalPercent: "Critical damage coefficient", overloadSecond: "Second Overload damage",
 overloadThird: "Third Overload damage", mysticReflexDamage: "Mystic Reflex damage",
};

export default function CombatItemStats({ bonuses }: { bonuses?: Record<string, number> }) {
 const { language } = useGameLanguage();
 const entries = Object.entries(bonuses ?? {}).filter(([key,value]) => labels[key] && value > 0);
 if (!entries.length) return null;
 return <span>{entries.map(([key,value],index) => <span key={key}>{uiText(language,labels[key])}: +{value}{key.endsWith("Percent") ? " " + uiText(language,"percentage points") : ""}{index < entries.length-1 && <br />}</span>)}</span>;
}
