"use client";

import Link from "next/link";
import { useAuth } from "@/contexts/AuthContext";
import { entryDestination } from "@/features/auth/publicEntry";
import styles from "@/styles/Home.module.css";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import UiText from "@/components/UiText";

export default function HomePage() {
  const { user, isLoading } = useAuth();
  const entryHref = entryDestination(user, isLoading);

  return <main className={styles.page}>
    <div className={styles.scene} aria-hidden="true" />
    <header className={styles.topbar}>
      <span className={styles.wordmark}>N<span>✦</span>CHALO</span>
      <div style={{ display: "flex", alignItems: "center", gap: 14 }}><LanguageSwitcher /><Link href="/login" className={styles.signIn}><UiText text="Sign In" /> <span aria-hidden="true">↗</span></Link></div>
    </header>
    <section className={styles.hero} aria-labelledby="landing-title">
      <div className={styles.copy}>
        <p className={styles.eyebrow}><span aria-hidden="true">✧</span> <UiText text="A DARK FANTASY TACTICAL RPG" /></p>
        <h1 id="landing-title">NACHALO</h1>
        <p className={styles.tagline}><UiText text="Every journey begins at the edge of darkness." /></p>
        <p className={styles.description}><UiText text="Choose your hero. Explore the unknown. Make every turn count." /></p>
        <div className={styles.actions}>
          {entryHref ? <Link className={`${styles.button} ${styles.primary}`} href={entryHref}><UiText text="Enter the Game" /> <span aria-hidden="true">↗</span></Link> : <span className={`${styles.button} ${styles.primary} ${styles.loading}`} role="status" aria-busy="true"><UiText text="Checking your journey…" /></span>}
          <Link className={`${styles.button} ${styles.secondary}`} href="/register"><UiText text="Create Account" /></Link>
        </div>
        <p className={styles.smallLink}><UiText text="Already have an account?" /> <Link href="/login"><UiText text="Sign In" /></Link></p>
      </div>
    </section>
    <footer className={styles.features} aria-label="Game features">
      <span><UiText text="HEX TACTICAL COMBAT" /></span><i aria-hidden="true" />
      <span>PVE &amp; PVP</span><i aria-hidden="true" />
      <span><UiText text="FOUR HERO CLASSES" /></span><i aria-hidden="true" />
      <span><UiText text="GEAR & STRONGHOLD" /></span>
    </footer>
  </main>;
}
