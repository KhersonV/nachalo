//==================================
// src/components/LoginPage.tsx
//==================================

"use client";

import React, { useState } from "react";
import { useAuth } from "../contexts/AuthContext";
import { useRouter } from "next/navigation";
import styles from "../styles/LoginPage.module.css";

const AUTH_BASE = process.env.NEXT_PUBLIC_AUTH_BASE || "http://localhost:8000";

const LoginPage = () => {
    const { login } = useAuth();
    const router = useRouter();
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState("");
    const [showPassword, setShowPassword] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError("");
        setIsSubmitting(true);

        try {
            const res = await fetch(`${AUTH_BASE}/auth/login`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ email, password }),
            });

            if (!res.ok) {
                const errorText = await res.text();
                throw new Error(errorText || "Ошибка входа");
            }

            const data = await res.json();
            console.log("Received login response:", data);
            login(data);
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
                <p className={styles.kicker}>Welcome back</p>
                <h1 className={styles.title}>Вход в систему</h1>
                <p className={styles.subtitle}>
                    Продолжи игру с сохраненным прогрессом.
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
                        <label className={styles.label}>Пароль</label>
                        <div className={styles.passwordWrap}>
                            <input
                                className={styles.input}
                                type={showPassword ? "text" : "password"}
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                placeholder="Введите пароль"
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

                    {error && <p className={styles.error}>{error}</p>}

                    <button
                        type="submit"
                        disabled={isSubmitting}
                        className={styles.submitButton}
                    >
                        {isSubmitting ? "Вход..." : "Войти"}
                    </button>

                    <button
                        type="button"
                        className={styles.linkButton}
                        onClick={() => router.push("/register")}
                    >
                        Нет аккаунта? Зарегистрироваться
                    </button>
                </form>
            </div>
        </section>
    );
};

export default LoginPage;
