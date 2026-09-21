import type { SourceStatus } from "./useUsageSummary";
import { useEffect, useState } from "react";

export function useSyncClock() {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(timer);
  }, []);
  return now;
}

export const collectorIds = ["claude_code", "codex", "cursor"] as const;

export function sourceIsFresh(source: SourceStatus | undefined, now: number) {
  const synced = Date.parse(source?.last_success ?? "");
  return source?.state === "ok" && !source.error && source.poll_seconds > 0 &&
    Number.isFinite(synced) && now - synced <= source.poll_seconds * 3_000;
}

export function syncStatus(sources: Record<string, SourceStatus> | undefined, failed: boolean, now: number) {
  if (failed) return { label: "Connection issue", healthy: false, lastSync: null };
  if (!sources) return { label: "Connecting…", healthy: false, lastSync: null };
  const ready = collectorIds.filter(id => sourceIsFresh(sources[id], now)).length;
  const dates = collectorIds.map(id => Date.parse(sources[id]?.last_success ?? ""));
  return {
    label: ready === 3 ? "All collectors synced" : `${ready}/3 collectors ready`,
    healthy: ready === 3,

    lastSync: dates.every(Number.isFinite) ? new Date(Math.min(...dates)).toISOString() : null,
  };
}
