export type Currency = "USD" | "VND";

const usdOptions: Intl.NumberFormatOptions = {
  style: "currency",
  currency: "USD",
  minimumFractionDigits: 2,
  maximumFractionDigits: 4,
};
const vndOptions: Intl.NumberFormatOptions = {
  style: "currency",
  currency: "VND",
  maximumFractionDigits: 0,
};

export function formatMoney(
  usd: number,
  currency: Currency,
  rate: number | null,
  locale?: string,
): string {
  if (currency === "VND" && rate !== null) return new Intl.NumberFormat(locale ?? "vi-VN", vndOptions).format(usd * rate);
  return new Intl.NumberFormat(locale ?? "en-US", usdOptions).format(usd);
}

export function parseRateResponse(json: unknown): number | null {
  if (typeof json !== "object" || json === null) return null;
  const rates = (json as { rates?: unknown }).rates;
  if (typeof rates !== "object" || rates === null) return null;
  const vnd = (rates as { VND?: unknown }).VND;
  return typeof vnd === "number" && Number.isFinite(vnd) ? vnd : null;
}

const CACHE_KEY = "usd_vnd_rate";

export function loadCachedRate(): { rate: number; ts: number } | null {
  try {
    const raw = localStorage.getItem(CACHE_KEY);
    if (!raw) return null;
    const v = JSON.parse(raw);
    if (typeof v?.rate === "number" && typeof v?.ts === "number") return v;
  } catch {
  }
  return null;
}

export function saveCachedRate(rate: number, ts: number): void {
  try { localStorage.setItem(CACHE_KEY, JSON.stringify({ rate, ts })); } catch { /* Display still works without cached preferences. */ }
}
