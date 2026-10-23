"use client";

import RequireAuth from "@/components/RequireAuth";
import ProfessionsPage from "@/components/ProfessionsPage";

export default function ProfessionsRoute() {
    return <RequireAuth><ProfessionsPage /></RequireAuth>;
}
