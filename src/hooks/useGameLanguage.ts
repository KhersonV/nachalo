"use client";

import { createContext, createElement, useContext, useEffect, useSyncExternalStore, type ReactNode } from "react";
import { hexMessages, type GameLanguage } from "@/i18n/hexGame";

const STORAGE_KEY = "nachalo.game.language";
const CHANGE_EVENT = "nachalo:language-change";
const isLanguage = (value: string | null): value is GameLanguage => value === "en" || value === "ru" || value === "de";
const LanguageContext = createContext<GameLanguage>("en");

function currentLanguage(): GameLanguage {
  if (typeof window === "undefined") return "en";
  const stored = window.localStorage.getItem(STORAGE_KEY);
  return isLanguage(stored) ? stored : "en";
}

function subscribe(listener: () => void) {
  window.addEventListener(CHANGE_EVENT, listener);
  window.addEventListener("storage", listener);
  return () => {
    window.removeEventListener(CHANGE_EVENT, listener);
    window.removeEventListener("storage", listener);
  };
}

export function setGameLanguage(next: GameLanguage) {
  window.localStorage.setItem(STORAGE_KEY, next);
  document.documentElement.lang = next;
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

export function GameLanguageProvider({ children }: { children: ReactNode }) {
  const language = useSyncExternalStore(subscribe, currentLanguage, () => "en" as GameLanguage);
  useEffect(() => { document.documentElement.lang = language; }, [language]);
  return createElement(LanguageContext.Provider, { value: language }, children);
}

export function useGameLanguage() {
  const language = useContext(LanguageContext);
  return { language, setLanguage: setGameLanguage, messages: hexMessages(language) };
}
