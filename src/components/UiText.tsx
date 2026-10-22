"use client";

import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

export default function UiText({ text }: { text: string }) {
  const { language } = useGameLanguage();
  return <>{uiText(language, text)}</>;
}
