
//=======================
// src/app/mode/page.tsx
//=======================

"use client";

import React from "react";
import ModeSelectionPage from "../../components/ModeSelectionPage";
import RequireAuth from "../../components/RequireAuth";

export default function ModePage() {
  return (
      <RequireAuth>
        <ModeSelectionPage />
      </RequireAuth>
  );
}
