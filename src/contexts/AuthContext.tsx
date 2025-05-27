
//=============================
// src/contexts/AuthContext.tsx
//=============================

"use client";

import React, { createContext, useContext, useState, useEffect } from "react";
import { useRouter } from "next/navigation";

type User = {
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
};

const AuthContext = createContext<AuthContextType | undefined>(undefined);

function parseJwt(token: string): { exp?: number } {
  try {
    const base = token.split('.')[1];
    const json = atob(base.replace(/-/g, '+').replace(/_/g, '/'));
    return JSON.parse(json);
  } catch {
    return {};
  }
}


export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const router = useRouter();

  useEffect(() => {
    let stored: string | null = null;
    try {
      stored = localStorage.getItem("user");
    } catch (e) {
      console.warn("Не удалось прочитать localStorage:", e);
    }
    if (!stored) return;

    try {
      const u: User = JSON.parse(stored);
      // Проверяем срок жизни токена
      const { exp } = parseJwt(u.token);
      if (exp && Date.now() >= exp * 1000) {
        // просрочен
        console.log("Токен истёк, вынужденный logout");
        logout();
      } else {
        setUser(u);
      }
    } catch (e) {
      console.warn("Не удалось распарсить user из localStorage:", e);
      logout();
    }
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
    <AuthContext.Provider value={{ user, login, logout }}>
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
