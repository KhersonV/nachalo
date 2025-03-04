// src/components/LoginPage.tsx
"use client";

import React, { useState } from "react";
import { useAuth } from "../contexts/AuthContext";

const LoginPage = () => {
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    try {
      // Отправляем запрос на эндпоинт логина
      const res = await fetch("http://localhost:8000/auth/login", {
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
      const token = data.token;

      // Получаем профиль пользователя, передавая токен
      const profileRes = await fetch("http://localhost:8000/auth/profile", {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${token}`,
        },
      });

      if (!profileRes.ok) {
        const errorText = await profileRes.text();
        throw new Error(errorText || "Ошибка получения профиля");
      }

      const userData = await profileRes.json();
      // Здесь можно добавить токен в userData, если потребуется
      login(userData);
    } catch (err: any) {
      setError(err.message || "Что-то пошло не так");
    }
  };

  return (
    <div>
      <h1>Вход в систему</h1>
      <form onSubmit={handleSubmit}>
        <div>
          <label>Email:</label>
          <input 
            type="email" 
            value={email} 
            onChange={(e) => setEmail(e.target.value)} 
            required 
          />
        </div>
        <div>
          <label>Пароль:</label>
          <input 
            type="password" 
            value={password} 
            onChange={(e) => setPassword(e.target.value)} 
            required 
          />
        </div>
        {error && <p style={{ color: "red" }}>{error}</p>}
        <button type="submit">Войти</button>
      </form>
    </div>
  );
};

export default LoginPage;
