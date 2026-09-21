export type DailyUsage = {
  date: string; source: string; calls: number; total_tokens: number;
  known_cost_usd: number; unknown_cost_calls: number;
};
export type Metric = "known_cost_usd" | "total_tokens" | "calls";
export type TimelineDay = { date: string; sources: Record<string, DailyUsage>; };

export function timelineDays(rows: DailyUsage[], from: string, to: string): TimelineDay[] {
  const byDate = new Map<string, TimelineDay>();
  for (const row of rows) {
    if (!byDate.has(row.date)) byDate.set(row.date, { date: row.date, sources: {} });
    byDate.get(row.date)!.sources[row.source] = row;
  }
  const result: TimelineDay[] = [];
  const start = Date.parse(`${from}T00:00:00Z`), end = Date.parse(`${to}T00:00:00Z`);
  for (let time = start; time <= end; time += 86400000) {
    const date = new Date(time).toISOString().slice(0, 10);
    result.push(byDate.get(date) ?? { date, sources: {} });
  }
  return result;
}

export const dayTotal = (day: TimelineDay, sources: string[], metric: Metric) => sources.reduce((sum, source) => sum + (day.sources[source]?.[metric] ?? 0), 0);
