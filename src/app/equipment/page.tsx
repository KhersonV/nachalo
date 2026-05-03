"use client";

import React from "react";
import RequireAuth from "../../components/RequireAuth";
import EquipmentPage from "../../components/EquipmentPage";

export default function EquipmentRoutePage() {
    return (
        <RequireAuth>
            <EquipmentPage />
        </RequireAuth>
    );
}
