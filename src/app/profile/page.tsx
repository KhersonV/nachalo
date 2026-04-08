"use client";

import React from "react";
import RequireAuth from "../../components/RequireAuth";
import ProfilePage from "../../components/ProfilePage";

export default function ProfileRoutePage() {
    return (
        <RequireAuth>
            <ProfilePage />
        </RequireAuth>
    );
}
