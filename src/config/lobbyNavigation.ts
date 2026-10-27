export type LobbyIcon = "swords" | "castle" | "hero" | "trade" | "book" | "more" | "backpack" | "tools" | "shop" | "auction" | "exchange" | "profile" | "history" | "logout";
export type LobbyGroupId = "hero" | "trade" | "knowledge" | "more";
export type MarketNavigationMode = "auction" | "exchange";
export const marketNavigationHref = (mode: MarketNavigationMode) => `/market?mode=${mode}`;
export type LobbyNavigationItem = {
  id: string;
  label: string;
  icon: LobbyIcon;
  group: "primary" | LobbyGroupId;
  order: number;
  requiresUser?: boolean;
} & ({ href: string; action?: never } | { action: "logout"; href?: never });
export const lobbyNavigationItems: readonly LobbyNavigationItem[] = [
  { id: "play", label: "Play", icon: "swords", href: "/mode", group: "primary", order: 10 },
  { id: "base", label: "Base", icon: "castle", href: "/base", group: "primary", order: 20 },
  // Inventory and equipped items share the same existing screen.
  { id: "equipment", label: "Inventory & equipment", icon: "backpack", href: "/equipment", group: "hero", order: 10 },
  { id: "professions", label: "Professions", icon: "tools", href: "/professions", group: "hero", order: 20 },
  { id: "shop", label: "Shop", icon: "shop", href: "/shop", group: "trade", order: 10 },
  { id: "auction", label: "Auctions", icon: "auction", href: marketNavigationHref("auction"), group: "trade", order: 20 },
  { id: "exchange", label: "Exchange", icon: "exchange", href: marketNavigationHref("exchange"), group: "trade", order: 30 },
  { id: "drop-guide", label: "Хроники NACHALO", icon: "book", href: "/drop-guide", group: "knowledge", order: 10 },
  { id: "profile", label: "Profile", icon: "profile", href: "/profile", group: "more", order: 10 },
  { id: "history", label: "Match History", icon: "history", href: "/matches", group: "more", order: 20 },
  { id: "logout", label: "Log out", icon: "logout", action: "logout", group: "more", order: 30, requiresUser: true },
];
export const lobbyNavigationGroups: ReadonlyArray<{ id: LobbyGroupId; label: string; icon: LobbyIcon; order: number; foldBelow?: number }> = [
  { id: "hero", label: "Hero", icon: "hero", order: 30 },
  { id: "trade", label: "Trade", icon: "trade", order: 40, foldBelow: 720 },
  { id: "knowledge", label: "Knowledge", icon: "book", order: 50, foldBelow: 940 },
  { id: "more", label: "More", icon: "more", order: 60 },
];
export function lobbyItemIsActive(item: LobbyNavigationItem, pathname: string) {
  if (!item.href) return false;
  const path = item.href.split("?")[0];
  return pathname === path || pathname.startsWith(path + "/");
}
