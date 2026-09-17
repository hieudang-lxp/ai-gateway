export type Currency = "USD" | "VND";

const usdFmt = new Intl.NumberFormat("en-US", {
  style: "currency",
  currency: "USD",
  minimumFractionDigits: 2,
  maximumFractionDigits: 4,
});
const vndFmt = new Intl.NumberFormat("vi-VN", {
  style: "currency",
  currency: "VND",
  maximumFractionDigits: 0,
});

export function formatMoney(
  usd: number,
  currency: Currency,
  rate: number | null,
): string {
  if (currency === "VND" && rate !== null) return vndFmt.format(usd * rate);
  return usdFmt.format(usd);
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
  const raw = localStorage.getItem(CACHE_KEY);
  if (!raw) return null;
  try {
    const v = JSON.parse(raw);
    if (typeof v?.rate === "number" && typeof v?.ts === "number") return v;
  } catch {
    // corrupt cache — ignore
  }
  return null;
}

export function saveCachedRate(rate: number, ts: number): void {
  localStorage.setItem(CACHE_KEY, JSON.stringify({ rate, ts }));
}
