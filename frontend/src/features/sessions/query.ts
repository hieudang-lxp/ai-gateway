import type { SessionFilters } from "./types";
import i18n from '@/i18n';
import { formatNumber } from '@/i18n/format';
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
  if (previous === 0) return i18n.t(current === 0 ? 'noChange' : 'noBaseline', { ns: 'sessions' });
  const change = (current - previous) / previous;
  return i18n.t('change', { ns: 'sessions', value: formatNumber(change, { style: 'percent', maximumFractionDigits: 1, signDisplay: 'exceptZero' }) });
}
