export type BarState = {
  pct: number;
  level: "ok" | "warn" | "over";
  cap: number | null;
};

export function barState(spent: number, warn: number, hard: number): BarState {
  const cap = hard > 0 ? hard : warn > 0 ? warn : null;
  const pct = cap ? Math.min(100, Math.max(0, (spent / cap) * 100)) : 0;
  const level =
    hard > 0 && spent >= hard ? "over" : warn > 0 && spent >= warn ? "warn" : "ok";
  return { pct, level, cap };
}
