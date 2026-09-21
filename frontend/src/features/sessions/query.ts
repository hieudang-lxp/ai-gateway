import type { SessionFilters } from "./types";
export function periodParams(period: string) {
  return new URLSearchParams(period === "month" ? { period: "month" } : { days: period });
}
export function sessionParams(filters: SessionFilters) {
  const params = periodParams(filters.period);
  for (const key of ["q", "source", "model"] as const) {
    const value = filters[key].trim();
    if (value) params.set(key, value);
  }
  params.set("limit", "25");
  return params;
}
export function sessionLink(source: string, sessionID: string) {
  return `#sessions?${new URLSearchParams({ source, session_id: sessionID })}`;
}
export function sessionSelection(hash: string) {
  const [page, query] = hash.split("?");
  if (page !== "#sessions") return null;
  const params = new URLSearchParams(query);
  const source = params.get("source"); const sessionID = params.get("session_id");
  return source && sessionID ? { source, sessionID } : null;
}
export function changeLabel(current: number, previous: number) {
  if (previous === 0) return current === 0 ? "No change" : "No prior baseline";
  const change = (current - previous) / previous * 100;
  return `${change > 0 ? "+" : ""}${change.toLocaleString(undefined, { maximumFractionDigits: 1 })}% vs previous period`;
}
