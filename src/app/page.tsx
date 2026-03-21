
//=======================
// src/app/page.tsx
//=======================

"use client";

import React from "react";
import { useRouter } from "next/navigation";
import styles from "../styles/Home.module.css";

export default function HomePage() {
    const router = useRouter();

    return (
        <div className={styles.hero}>
            <div className={styles.overlay}>
                <header className={styles.header}>
                    <h1>Welcome to Nachalo - Exit!</h1>
                    <p>
                        Dive into fast-paced battles and showcase your skills in
                        one-on-one or team modes. Become a champion in the world
                        of Nachalo - Exit!
                    </p>
                    <div className={styles.ctaButtons}>
                        <button
                            onClick={() => router.push("/login")}
                            className={`${styles.btn} ${styles.btnLogin}`}
                        >
                            Sign In
                        </button>
                        <button
                            onClick={() => router.push("/register")}
                            className={`${styles.btn} ${styles.btnRegister}`}
                        >
                            Register
                        </button>
                    </div>
                </header>
                <section className={styles.features}>
                    <div className={styles.featureCard}>
                        <h3>Innovative Gameplay</h3>
                        <p>
                            Unique modes, thrilling battles, and tactical
                            choices.
                        </p>
                    </div>
                    <div className={styles.featureCard}>
                        <h3>Competitive Rankings</h3>
                        <p>
                            Challenge players worldwide and climb the
                            leaderboards.
                        </p>
                    </div>
                    <div className={styles.featureCard}>
                        <h3>Active Community</h3>
                        <p>Join regular tournaments and events.</p>
                    </div>
                </section>
            </div>
        </div>
    );
}
