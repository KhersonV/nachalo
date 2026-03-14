//=====================================
// src/components/RegisterPage.tsx
//=====================================

"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import styles from "../styles/RegisterPage.module.css";
import {
    characterArchetypes,
    type CharacterArchetype,
} from "../constants/characterArchetypes";

const AUTH_BASE = process.env.NEXT_PUBLIC_AUTH_BASE || "http://localhost:8000";

const RegisterPage = () => {
    const router = useRouter();
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [name, setName] = useState("");
    const [characterType, setCharacterType] = useState<
        CharacterArchetype["id"]
    >(characterArchetypes[0].id);
    const [error, setError] = useState("");
    const [showPassword, setShowPassword] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);

    const selectedArchetype =
        characterArchetypes.find((item) => item.id === characterType) ??
        characterArchetypes[0];

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError("");
        setIsSubmitting(true);

        try {
            const res = await fetch(`${AUTH_BASE}/auth/register`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    email,
                    password,
                    name,
                    image: selectedArchetype.image,
                    characterType,
                }),
            });

            if (!res.ok) {
                const errorText = await res.text();
                throw new Error(errorText || "Ошибка регистрации");
            }

            // Можно сразу перейти на страницу логина или автоматически войти
            router.push("/login");
        } catch (err: any) {
            setError(err.message || "Что-то пошло не так");
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <section className={styles.page}>
            <div className={styles.glowA} aria-hidden="true" />
            <div className={styles.glowB} aria-hidden="true" />

            <div className={styles.card}>
                <p className={styles.kicker}>Create account</p>
                <h1 className={styles.title}>Регистрация</h1>
                <p className={styles.subtitle}>
                    Создай персонажа и присоединяйся к игре.
                </p>

                <form onSubmit={handleSubmit} className={styles.form}>
                    <div className={styles.field}>
                        <label className={styles.label}>Email</label>
                        <input
                            className={styles.input}
                            type="email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            placeholder="you@example.com"
                            required
                        />
                    </div>

                    <div className={styles.field}>
                        <label className={styles.label}>Имя</label>
                        <input
                            className={styles.input}
                            type="text"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder="Твой ник"
                            required
                        />
                    </div>

                    <div className={styles.field}>
                        <label className={styles.label}>Пароль</label>
                        <div className={styles.passwordWrap}>
                            <input
                                className={styles.input}
                                type={showPassword ? "text" : "password"}
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                placeholder="Минимум 6 символов"
                                minLength={6}
                                required
                            />
                            <button
                                type="button"
                                className={styles.passwordToggle}
                                onClick={() => setShowPassword((v) => !v)}
                            >
                                {showPassword ? "Скрыть" : "Показать"}
                            </button>
                        </div>
                    </div>

                    <div className={styles.field}>
                        <div className={styles.characterHeader}>
                            <label className={styles.label}>
                                Выберите архетип персонажа
                            </label>
                            <p className={styles.characterHint}>
                                Класс определяет стартовые характеристики. Позже
                                можно будет расширить набор архетипов без
                                переписывания формы.
                            </p>
                            <div className={styles.statLegend}>
                                <p className={styles.statLegendTitle}>
                                    Как читать параметры:
                                </p>
                                <p className={styles.statLegendRow}>
                                    HP: запас здоровья персонажа.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Энергия: ресурс на действия;
                                    восстанавливается каждый ход.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Атака/Защита: исходящий и входящий урон в
                                    бою.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Мобильность/Ловкость: базовые параметры
                                    подвижности архетипа.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Обзор: сколько клеток вокруг видно на карте.
                                </p>
                                <p className={styles.statLegendRow}>
                                    Тип боя и дальность атаки: ближний или
                                    дальний режим и максимальная дистанция
                                    удара.
                                </p>
                            </div>
                        </div>
                        <div className={styles.avatarGrid}>
                            {characterArchetypes.map((item) => (
                                <label
                                    key={item.id}
                                    className={`${styles.avatarOption} ${characterType === item.id ? styles.avatarOptionActive : ""}`}
                                >
                                    <input
                                        type="radio"
                                        name="characterType"
                                        value={item.id}
                                        checked={characterType === item.id}
                                        onChange={() =>
                                            setCharacterType(item.id)
                                        }
                                        className={styles.avatarRadio}
                                    />
                                    <img
                                        src={item.image}
                                        alt={item.name}
                                        className={styles.avatarImage}
                                    />
                                    <div className={styles.avatarContent}>
                                        <div className={styles.avatarTitleRow}>
                                            <strong>{item.name}</strong>
                                            <span className={styles.avatarTag}>
                                                {item.title}
                                            </span>
                                        </div>
                                        <p className={styles.avatarDescription}>
                                            {item.description}
                                        </p>
                                        <div className={styles.statsGrid}>
                                            {item.stats.map((stat) => (
                                                <span
                                                    key={`${item.id}-${stat.label}`}
                                                    className={styles.statChip}
                                                >
                                                    {stat.label}: {stat.value}
                                                </span>
                                            ))}
                                        </div>
                                    </div>
                                </label>
                            ))}
                        </div>
                    </div>

                    {error && <p className={styles.error}>{error}</p>}

                    <button
                        type="submit"
                        disabled={isSubmitting}
                        className={styles.submitButton}
                    >
                        {isSubmitting ? "Создание..." : "Зарегистрироваться"}
                    </button>

                    <button
                        type="button"
                        className={styles.linkButton}
                        onClick={() => router.push("/login")}
                    >
                        Уже есть аккаунт? Войти
                    </button>
                </form>
            </div>
        </section>
    );
};

export default RegisterPage;
