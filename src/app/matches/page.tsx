"use client";

import React from "react";
import MatchHistoryPage from "../../components/MatchHistoryPage";
import RequireAuth from "../../components/RequireAuth";

export default function MatchesPage() {
    return (
        <RequireAuth>
            <MatchHistoryPage />
        </RequireAuth>
    );
}
