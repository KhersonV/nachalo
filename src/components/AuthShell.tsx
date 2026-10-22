import type { ReactNode } from "react";
import Link from "next/link";
import styles from "@/styles/AuthShell.module.css";
import LanguageSwitcher from "./LanguageSwitcher";
import UiText from "./UiText";

export default function AuthShell({ eyebrow, title, description, children, wide = false }: { eyebrow: string; title: string; description: string; children: ReactNode; wide?: boolean }) {
  return <main className={`${styles.page} ${wide ? styles.registrationPage : ""}`}>
    <div className={styles.scene} aria-hidden="true" />
    <header className={styles.topbar}><Link href="/" className={styles.wordmark}>N<span>✦</span>CHALO</Link><div style={{ display: "flex", alignItems: "center", gap: 12 }}><Link href="/" className={styles.back}>← <UiText text="Back to the beginning" /></Link><LanguageSwitcher /></div></header>
    <div className={`${styles.layout} ${wide ? styles.registrationLayout : ""}`}>
      <div className={styles.intro}><p className={styles.introKicker}><UiText text="THE JOURNEY BEGINS HERE" /></p><h2><UiText text="Step into" /><br /><em><UiText text="the unknown." /></em></h2><p><UiText text="Every choice shapes the road ahead." /></p></div>
      <section className={`${styles.card} ${wide ? styles.cardWide : ""}`} aria-labelledby="auth-title">
        <p className={styles.eyebrow}>{eyebrow}</p><h1 id="auth-title">{title}</h1><p className={styles.description}>{description}</p>{children}
      </section>
    </div>
    <footer className={styles.footer}>NACHALO <span aria-hidden="true">✦</span> <UiText text="A DARK FANTASY TACTICAL RPG" /></footer>
  </main>;
}
