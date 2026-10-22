"use client";

import { useGameLanguage } from "@/hooks/useGameLanguage";

export default function LanguageSwitcher({ className }: { className?: string }) {
  const { language, setLanguage, messages } = useGameLanguage();
  return <select className={className} aria-label={messages.language} value={language}
    style={{ minWidth: 64, height: 38, border: "1px solid #8b7855", borderRadius: 6, padding: "0 8px", background: "#10202f", color: "#f1dfb9", font: "inherit", fontSize: 12, cursor: "pointer" }}
    onChange={event => setLanguage(event.target.value as typeof language)}>
    <option value="ru">RU</option><option value="en">EN</option><option value="de">DE</option>
  </select>;
}
