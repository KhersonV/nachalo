//=======================
// src/app/game/page.tsx
//=======================

"use client";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Provider } from "react-redux";
import { store } from "../../store";
import GameWrapper from "../../features/game/GameWrapper";
import GameController from "../../components/GameController";

function GameContent() {
    const searchParams = useSearchParams();
    const instanceId = searchParams.get("instance_id") ?? "";

    if (!instanceId) {
        return <div>Не задан instance_id</div>;
    }

    return (
        <Provider store={store}>
            <GameWrapper instanceId={instanceId}>
                <GameController instanceId={instanceId} />
            </GameWrapper>
        </Provider>
    );
}

export default function GamePage() {
    return (
        <Suspense fallback={null}>
            <GameContent />
        </Suspense>
    );
}
