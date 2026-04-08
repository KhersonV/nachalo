//=======================
// src/app/layout.tsx
//=======================

import "../styles/globals.css";
import { AuthProvider } from "../contexts/AuthContext";
import ReduxProvider from "@/providers/ReduxProvider";
import GlobalMatchListener from "../components/GlobalMatchListener";

export default function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <html lang="en">
            <head>
                <title>Игра "Начало"</title>
                <meta
                    name="viewport"
                    content="width=device-width, initial-scale=1.0"
                />
            </head>
            <body>
                <AuthProvider>
                    <ReduxProvider>{children}</ReduxProvider>
                    <GlobalMatchListener />
                </AuthProvider>
                <div id="modal-root" />
            </body>
        </html>
    );
}
