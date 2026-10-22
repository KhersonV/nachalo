"use client";

import Link from "next/link";
import { useAuth } from "@/contexts/AuthContext";
import { entryDestination } from "@/features/auth/publicEntry";
import styles from "@/styles/Home.module.css";

export default function HomePage() {
  const { user, isLoading } = useAuth();
  const entryHref = entryDestination(user, isLoading);

  return <main className={styles.page}>
    <div className={styles.scene} aria-hidden="true" />
    <header className={styles.topbar}>
      <span className={styles.wordmark}>N<span>✦</span>CHALO</span>
      <Link href="/login" className={styles.signIn}>Sign In <span aria-hidden="true">↗</span></Link>
    </header>
    <section className={styles.hero} aria-labelledby="landing-title">
      <div className={styles.copy}>
        <p className={styles.eyebrow}><span aria-hidden="true">✧</span> A DARK FANTASY TACTICAL RPG</p>
        <h1 id="landing-title">NACHALO</h1>
        <p className={styles.tagline}>Every journey begins at the edge of darkness.</p>
        <p className={styles.description}>Choose your hero. Explore the unknown. Make every turn count.</p>
        <div className={styles.actions}>
          {entryHref ? <Link className={`${styles.button} ${styles.primary}`} href={entryHref}>Enter the Game <span aria-hidden="true">↗</span></Link> : <span className={`${styles.button} ${styles.primary} ${styles.loading}`} role="status" aria-busy="true">Checking your journey…</span>}
          <Link className={`${styles.button} ${styles.secondary}`} href="/register">Create Account</Link>
        </div>
        <p className={styles.smallLink}>Already have an account? <Link href="/login">Sign In</Link></p>
      </div>
    </section>
    <footer className={styles.features} aria-label="Game features">
      <span>HEX TACTICAL COMBAT</span><i aria-hidden="true" />
      <span>PVE &amp; PVP</span><i aria-hidden="true" />
      <span>FOUR HERO CLASSES</span><i aria-hidden="true" />
      <span>GEAR &amp; STRONGHOLD</span>
    </footer>
  </main>;
}
