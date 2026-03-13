//=====================================
// src/components/RegisterPage.tsx
//=====================================

"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import styles from "../styles/RegisterPage.module.css";

const AUTH_BASE = process.env.NEXT_PUBLIC_AUTH_BASE || "http://localhost:8000";

// Массив доступных картинок
const avatarOptions = [
    "/Character_1.webp",
    "/Character_2.webp",
    "/player-1.webp",
    "/player-2.webp",
];

const RegisterPage = () => {
    const router = useRouter();
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [name, setName] = useState("");
    const [image, setImage] = useState(avatarOptions[0]);
    const [error, setError] = useState("");
    const [showPassword, setShowPassword] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);

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
                body: JSON.stringify({ email, password, name, image }),
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
                        <label className={styles.label}>Выберите аватар</label>
                        <div className={styles.avatarGrid}>
                            {avatarOptions.map((src) => (
                                <label
                                    key={src}
                                    className={`${styles.avatarOption} ${image === src ? styles.avatarOptionActive : ""}`}
                                >
                                    <input
                                        type="radio"
                                        name="avatar"
                                        value={src}
                                        checked={image === src}
                                        onChange={() => setImage(src)}
                                        className={styles.avatarRadio}
                                    />
                                    <img
                                        src={src}
                                        alt="avatar"
                                        className={styles.avatarImage}
                                    />
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
