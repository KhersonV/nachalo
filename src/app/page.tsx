// app/page.tsx
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
          <h1>Добро пожаловать в Nachalo - Exit!</h1>
          <p>
            Погрузитесь в динамичные сражения и продемонстрируйте свои навыки в битвах один на один или в командных режимах. Станьте чемпионом в мире Nachalo - Exit!
          </p>
          <div className={styles.ctaButtons}>
            <button
              onClick={() => router.push("/login")}
              className={`${styles.btn} ${styles.btnLogin}`}
            >
              Вход
            </button>
            <button
              onClick={() => router.push("/register")}
              className={`${styles.btn} ${styles.btnRegister}`}
            >
              Регистрация
            </button>
          </div>
        </header>
        <section className={styles.features}>
          <div className={styles.featureCard}>
            <h3>Инновационный геймплей</h3>
            <p>Уникальные режимы, захватывающие битвы и тактические решения.</p>
          </div>
          <div className={styles.featureCard}>
            <h3>Соревнования и рейтинги</h3>
            <p>Бросьте вызов игрокам со всего мира и поднимитесь в рейтинге.</p>
          </div>
          <div className={styles.featureCard}>
            <h3>Активное сообщество</h3>
            <p>Участвуйте в регулярных турнирах и мероприятиях.</p>
          </div>
        </section>
      </div>
    </div>
  );
}
