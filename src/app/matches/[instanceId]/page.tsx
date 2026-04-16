"use client";

import React from "react";
import { useParams } from "next/navigation";
import MatchHistoryDetailsPage from "../../../components/MatchHistoryDetailsPage";
import RequireAuth from "../../../components/RequireAuth";

export default function MatchDetailsRoutePage() {
    const params = useParams<{ instanceId: string }>();
    const instanceId = Array.isArray(params?.instanceId)
        ? params.instanceId[0]
        : params?.instanceId ?? "";

    return (
        <RequireAuth>
            <MatchHistoryDetailsPage instanceId={instanceId} />
        </RequireAuth>
    );
}
