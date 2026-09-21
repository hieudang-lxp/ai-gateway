import type { ModelUsage } from "../insights/network";
import type { DailyUsage } from "../insights/timeline";
export type Session = {
  source: string; session_id: string; title: string; project: string; models: string[];
  started_at: number; last_at: number; calls: number; input_tokens: number; output_tokens: number;
  cache_read_tokens: number; cache_write_tokens: number; known_cost_usd: number;
  estimated_cost_calls: number; fallback_cost_calls: number; unknown_cost_calls: number; cache_ratio: number | null;
};
export type SessionEvent = { event_id: string; ts: number; model: string; input_tokens: number; output_tokens: number; cache_read_tokens: number; cache_write_tokens: number; known_cost_usd: number | null; cost_kind: string; fallback: boolean };
export type SessionsResponse = { sessions: Session[]; next_cursor: string; total: number; unassigned_events: number; indexed_at: string };
export type SessionDetail = { session: Session; events: SessionEvent[]; next_cursor: string };
export type SessionFilters = { period: string; q: string; source: string; model: string };
export type InsightTotals = { sessions: number; calls: number; total_tokens: number; known_cost_usd: number; unknown_cost_calls: number };
export type InsightsResponse = { network: ModelUsage[]; daily: DailyUsage[]; time_zone: string; date_from: string; date_to: string; since: string; until: string; previous_since: string; totals: InsightTotals; previous: InsightTotals; top_sessions: Session[]; findings: { id: string; title: string; description: string; metric: string; source?: string; session_id?: string }[]; unassigned_events: number; generated_at: string; indexed_at: string };
