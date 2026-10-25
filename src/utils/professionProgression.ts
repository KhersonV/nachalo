// Rank names only: XP, max level and unlock requirements come from the API.
export function professionRankName(level: number): string {
  return level === 3 ? "Master" : level === 2 ? "Craftsperson" : "Apprentice";
}
