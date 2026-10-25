"use client";

import RequireAuth from "@/components/RequireAuth";
import DropGuidePage from "@/components/DropGuidePage";

export default function DropGuideRoute() {
    return <RequireAuth><DropGuidePage /></RequireAuth>;
}
