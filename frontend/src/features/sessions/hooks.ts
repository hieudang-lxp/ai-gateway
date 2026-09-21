import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { apiBaseURL } from "../../lib/transport";
import { periodParams, sessionParams } from "./query";
import type { InsightsResponse, SessionDetail, SessionFilters, SessionsResponse } from "./types";
import type { ModelOption } from "./models";
async function request<T>(path: string, params: URLSearchParams, signal: AbortSignal): Promise<T> {
  const url = new URL(path, apiBaseURL); url.search = params.toString();
  const response = await fetch(url, { signal, cache: "no-store" });
  if (!response.ok) throw new Error(response.status === 404 ? "This indexed data is not available yet. Check collection status in Data & Pricing." : `Unable to load indexed usage (HTTP ${response.status}). Please retry.`);
  return response.json();
}
export function useSessions(filters: SessionFilters) {
  return useInfiniteQuery({ queryKey: ["sessions", filters], initialPageParam: "", queryFn: ({ pageParam, signal }) => {
    const params = sessionParams(filters); if (pageParam) params.set("cursor", pageParam);
    return request<SessionsResponse>("/_sessions", params, signal);
  }, getNextPageParam: page => page.next_cursor || undefined, staleTime: 30_000, retry: 1 });
}
export function useSessionDetail(source: string, sessionID: string) {
  return useInfiniteQuery({ queryKey: ["session-detail", source, sessionID], initialPageParam: "", queryFn: ({ pageParam, signal }) => {
    const params = new URLSearchParams({ source, session_id: sessionID, limit: "50" });
    if (pageParam) params.set("cursor", pageParam);
    return request<SessionDetail>("/_sessions/detail", params, signal);
  }, getNextPageParam: page => page.next_cursor || undefined, staleTime: 30_000, retry: 1 });
}
export function useInsights(period: string) {
  return useQuery({ queryKey: ["insights", period], queryFn: ({ signal }) => request<InsightsResponse>("/_insights", periodParams(period), signal), staleTime: 30_000, refetchInterval: 60_000, retry: 1 });
}

export function useSessionModels() {
  return useQuery({ queryKey: ["session-models"], queryFn: ({ signal }) => request<{ models: ModelOption[] }>("/_sessions/models", new URLSearchParams(), signal), staleTime: 30_000, refetchInterval: 60_000, retry: 1 });
}
