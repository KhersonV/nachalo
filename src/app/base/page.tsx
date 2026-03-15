"use client";

import React from "react";
import RequireAuth from "../../components/RequireAuth";
import LobbyBasePage from "../../components/LobbyBasePage";

export default function BasePage() {
    return (
        <RequireAuth>
            <LobbyBasePage />
        </RequireAuth>
    );
}
