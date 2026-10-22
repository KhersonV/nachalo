import type { ReactNode } from "react";
import Link from "next/link";
import styles from "@/styles/AuthShell.module.css";

export default function AuthShell({ eyebrow, title, description, children, wide = false }: { eyebrow: string; title: string; description: string; children: ReactNode; wide?: boolean }) {
  return <main className={`${styles.page} ${wide ? styles.registrationPage : ""}`}>
    <div className={styles.scene} aria-hidden="true" />
    <header className={styles.topbar}><Link href="/" className={styles.wordmark}>N<span>✦</span>CHALO</Link><Link href="/" className={styles.back}>← Back to the beginning</Link></header>
    <div className={`${styles.layout} ${wide ? styles.registrationLayout : ""}`}>
      <div className={styles.intro}><p className={styles.introKicker}>THE JOURNEY BEGINS HERE</p><h2>Step into<br /><em>the unknown.</em></h2><p>Every choice shapes the road ahead.</p></div>
      <section className={`${styles.card} ${wide ? styles.cardWide : ""}`} aria-labelledby="auth-title">
        <p className={styles.eyebrow}>{eyebrow}</p><h1 id="auth-title">{title}</h1><p className={styles.description}>{description}</p>{children}
      </section>
    </div>
    <footer className={styles.footer}>NACHALO <span aria-hidden="true">✦</span> A DARK FANTASY TACTICAL RPG</footer>
  </main>;
}
