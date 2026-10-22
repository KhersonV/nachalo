import type { ReactNode } from "react";
import styles from "@/styles/GameStatsPage.module.css";
import LanguageSwitcher from "./LanguageSwitcher";

export default function GameResultShell({ children }: { children: ReactNode }) {
  return <main className={styles.page}><div className={styles.languageControl}><LanguageSwitcher /></div>{children}</main>;
}
