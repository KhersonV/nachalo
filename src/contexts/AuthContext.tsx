//=============================
// src/contexts/AuthContext.tsx
//=============================

"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { debugLog } from "../utils/log";
import { useRouter } from "next/navigation";

const AUTH_BASE = process.env.NEXT_PUBLIC_AUTH_BASE || "http://localhost:8000";

export type User = {
    id: number;
    email: string;
    name: string;
    token: string;
    rating: number;
    created_at: string;
    level: number;
};

type AuthContextType = {
    user: User | null;
    login: (userData: User) => void;
    logout: () => void;
    isLoading: boolean;
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

function parseJwt(token: string): { exp?: number } {
    try {
        const base = token.split(".")[1];
        const json = atob(base.replace(/-/g, "+").replace(/_/g, "/"));
        return JSON.parse(json);
    } catch {
        return {};
    }
}

async function validateSessionToken(token: string): Promise<boolean> {
    try {
        const res = await fetch(`${AUTH_BASE}/auth/profile`, {
            method: "GET",
            headers: {
                Authorization: `Bearer ${token}`,
            },
            cache: "no-store",
        });
        return res.ok;
    } catch {
        return false;
    }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
    const [user, setUser] = useState<User | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const router = useRouter();

    useEffect(() => {
        async function restoreUser() {
            let stored: string | null = null;
            try {
                stored = localStorage.getItem("user");
            } catch (e) {
                console.warn("Не удалось прочитать localStorage:", e);
            }
            if (!stored) {
                setIsLoading(false);
                return;
            }

            try {
                const u: User = JSON.parse(stored);
                // Проверяем срок жизни токена
                const { exp } = parseJwt(u.token);
                if (exp && Date.now() >= exp * 1000) {
                    // просрочен
                    debugLog("Токен истёк, вынужденный logout");
                    localStorage.removeItem("user");
                    setUser(null);
                } else {
                    const isValid = await validateSessionToken(u.token);
                    if (isValid) {
                        setUser(u);
                    } else {
                        localStorage.removeItem("user");
                        setUser(null);
                    }
                }
            } catch (e) {
                console.warn("Не удалось распарсить user из localStorage:", e);
                localStorage.removeItem("user");
                setUser(null);
            } finally {
                setIsLoading(false);
            }
        }
        restoreUser();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const login = (userData: User) => {
        setUser(userData);
        try {
            localStorage.setItem("user", JSON.stringify(userData));
        } catch (e) {
            console.warn("Не удалось записать в localStorage:", e);
        }
        router.push("/mode");
    };

    const logout = () => {
        setUser(null);
        try {
            localStorage.removeItem("user");
        } catch (e) {
            console.warn("Не удалось очистить localStorage:", e);
        }
        router.push("/login");
    };

    return (
        <AuthContext.Provider value={{ user, login, logout, isLoading }}>
            {children}
        </AuthContext.Provider>
    );
}

export function useAuth() {
    const context = useContext(AuthContext);
    if (!context) {
        throw new Error("useAuth must be used within an AuthProvider");
    }
    return context;
}
