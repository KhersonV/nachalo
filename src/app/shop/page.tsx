"use client";

import React from "react";
import RequireAuth from "../../components/RequireAuth";
import LobbyShopPage from "../../components/LobbyShopPage";

export default function ShopPage() {
    return (
        <RequireAuth>
            <LobbyShopPage />
        </RequireAuth>
    );
}
