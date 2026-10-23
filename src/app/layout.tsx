//=======================
// src/app/layout.tsx
//=======================

import "../styles/globals.css";
import { AuthProvider } from "../contexts/AuthContext";
import GlobalMatchListener from "../components/GlobalMatchListener";
import GameChat from "../components/GameChat";
import { GameLanguageProvider } from "@/hooks/useGameLanguage";

export default function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <html lang="en">
            <head>
                <title>Nachalo</title>
                <meta
                    name="viewport"
                    content="width=device-width, initial-scale=1.0"
                />
            </head>
            <body>
              <GameLanguageProvider>
                <AuthProvider>
                    {children}
                    <GlobalMatchListener />
                    <GameChat />
                </AuthProvider>
                <div id="modal-root" />
              </GameLanguageProvider>
            </body>
        </html>
    );
}
