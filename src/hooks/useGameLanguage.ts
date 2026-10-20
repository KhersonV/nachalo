"use client";

import { useEffect, useState } from "react";
import { hexMessages, type GameLanguage } from "@/i18n/hexGame";

const STORAGE_KEY = "nachalo.game.language";
const isLanguage = (value: string | null): value is GameLanguage => value === "en" || value === "ru" || value === "de";

export function useGameLanguage() {
  const [language, setLanguage] = useState<GameLanguage>("en");
  useEffect(() => {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (isLanguage(stored)) setLanguage(stored);
  }, []);
  const changeLanguage = (next: GameLanguage) => {
    setLanguage(next);
    window.localStorage.setItem(STORAGE_KEY, next);
  };
  return { language, setLanguage: changeLanguage, messages: hexMessages(language) };
}
