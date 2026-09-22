import { useQuery } from "@tanstack/react-query";
import { apiBaseURL } from "../../lib/transport";
import type { UsageCounts } from "./usage";
import type { TFunction } from "i18next";

export class UsageFetchError extends Error {
  readonly status: number;
  constructor(status: number) { super(`HTTP ${status}`); this.status = status; }
}

export function usageErrorMessage(error: Error, t: TFunction) {
  return error instanceof UsageFetchError ? t("collectorUnavailable", { status: error.status }) : t("loadError");
}

export function sourceStateLabel(state: string | undefined, t: TFunction) {
  const keys: Record<string, string> = { ok: "synced", starting: "starting", error: "error", stale: "stale", unavailable: "unavailable" };
  return t(keys[state ?? "unavailable"] ?? "unknown");
}

export type UsageRow = UsageCounts & {
  source: string;
  model: string;
  estimated_cost_calls: number;
  fallback_cost_calls: number;
  first_ts: number;
  last_ts: number;
};
export type SourceStatus = {
  state: string;
  last_success: string | null;
  error?: string;
  poll_seconds: number;
  files?: number;
};
export type UsageSummary = {
  pricing: { source: string; updated_at: string; stale: boolean; error?: string; basis: string; fallback_model: string; refresh_seconds: number };
  rows: UsageRow[];
  sources: Record<string, SourceStatus>;
  cursor_history_days: number;
  since: string;
  generated_at: string;
};
export const sourceNames: Record<string, string> = { claude_code: "Claude Code", claude_gateway: "Claude gateway", codex: "Codex", cursor: "Cursor" };

export function useUsageSummary(period: string) {
  return useQuery<UsageSummary>({
    queryKey: ["unified-usage", period],
    queryFn: async () => {
      const url = new URL("/_usage", apiBaseURL);
      url.searchParams.set(period === "month" ? "period" : "days", period);
      const response = await fetch(url, { cache: "no-store" });
      if (!response.ok) throw new UsageFetchError(response.status);
      return response.json();
    },
    refetchInterval: 30_000,
    retry: 1,
    throwOnError: false,
  });
}
