//Tooltip.tsx

"use client";

import React from "react";
import "./tooltip.css";

type TooltipProps = {
  title: string;
  difficulty: number;
  description: string;
};

export default function Tooltip({ title, difficulty, description }: TooltipProps) {
  return (
    <div className="tooltip">
      <h4>{title}</h4>
      <p>Сложность: {difficulty}</p>
      <p>{description}</p>
    </div>
  );
}
