"use client";

import React, { useState } from "react";
import Tooltip from "./Tooltip";
import "./resource.css";

type ResourceProps = {
  type: string;
  difficulty: number;
  description: string;
  onCollect: () => void;
};

export default function Resource({ type, difficulty, description, onCollect }: ResourceProps) {
  const [showTooltip, setShowTooltip] = useState(false);

  return (
    <div
      className="resource"
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
      onClick={onCollect}
    >
      <div className="resource-icon" />
      {showTooltip && <Tooltip title={type} difficulty={difficulty} description={description} />}
    </div>
  );
}
