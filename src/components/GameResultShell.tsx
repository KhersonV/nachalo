import type { ReactNode } from "react";
import styles from "@/styles/GameStatsPage.module.css";

export default function GameResultShell({ children }: { children: ReactNode }) {
  return <main className={styles.page}>{children}</main>;
}
