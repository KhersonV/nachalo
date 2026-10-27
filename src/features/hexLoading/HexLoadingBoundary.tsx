"use client";
import { Component, type ReactNode } from "react";
import HexLoading from "./HexLoading";

// Chunk download/render errors must leave the surrounding HUD and 2D switch usable.
export default class HexLoadingBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    return this.state.failed
      ? <HexLoading local error="visual" onRetry={() => window.location.reload()} />
      : this.props.children;
  }
}
