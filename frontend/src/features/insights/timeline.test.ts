import { expect, it } from "vitest";
import { dayTotal, timelineDays, type DailyUsage } from "./timeline";

it("fills missing days without shifting source-local dates and preserves sums", () => {
  const rows: DailyUsage[] = [
    {date:"2026-09-01",source:"codex",calls:2,total_tokens:100,known_cost_usd:3,unknown_cost_calls:0},
    {date:"2026-09-03",source:"cursor",calls:1,total_tokens:20,known_cost_usd:0,unknown_cost_calls:1},
  ];
  const days = timelineDays(rows,"2026-09-01","2026-09-03");
  expect(days.map(day=>day.date)).toEqual(["2026-09-01","2026-09-02","2026-09-03"]);
  expect(dayTotal(days[1],["codex","cursor"],"calls")).toBe(0);
  expect(days.reduce((sum,day)=>sum+dayTotal(day,["codex","cursor"],"total_tokens"),0)).toBe(120);
  expect(dayTotal(days[2],["codex"],"calls")).toBe(0);
  expect(days[2].sources.cursor.unknown_cost_calls).toBe(1);
});
