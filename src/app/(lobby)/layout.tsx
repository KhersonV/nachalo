import type { ReactNode } from "react";
import { Suspense } from "react";
import LobbyShell from "@/components/LobbyShell";

export default function LobbyLayout({ children }: { children: ReactNode }) {
    return (
        <Suspense fallback={<main role="status" aria-busy="true">Loading lobby…</main>}>
            <LobbyShell>{children}</LobbyShell>
        </Suspense>
    );
}
