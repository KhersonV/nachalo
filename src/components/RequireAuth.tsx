
//=================================
// src/components/RequireAuth.tsx
//=================================

"use client";
import React, { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "../contexts/AuthContext";

type Props = {
  children: React.ReactNode;
};

export default function RequireAuth({ children }: Props) {
  const { user, isLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return <div>Загрузка...</div>;
  }
  if (!user) {
    // можно вернуть null или спиннер, т.к. уже редирект
    return null;
  }

  return <>{children}</>;
}
