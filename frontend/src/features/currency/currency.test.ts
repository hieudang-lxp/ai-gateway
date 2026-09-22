import { describe, it, expect, beforeEach } from "vitest";
import {
  formatMoney,
  parseRateResponse,
  loadCachedRate,
  saveCachedRate,
} from "./currency";

beforeEach(() => {
  const store = new Map<string, string>();
  (globalThis as Record<string, unknown>).localStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
  };
});

describe("formatMoney", () => {
  it("uses the display locale without changing currency or conversion", () => {
    expect(formatMoney(12.5, "USD", null, "de")).toBe("12,50 $");
    expect(formatMoney(2, "VND", 25000, "en")).toContain("50,000");
  });
  it("formats USD with up to 4 fraction digits", () => {
    expect(formatMoney(0.0034, "USD", null)).toBe("$0.0034");
    expect(formatMoney(12.5, "USD", null)).toBe("$12.50");
  });
  it("converts to VND with vi-VN grouping, no decimals", () => {
    const s = formatMoney(2, "VND", 25000);
    expect(s).toContain("50.000");
    expect(s).toContain("₫");
  });
  it("falls back to USD when VND has no rate", () => {
    expect(formatMoney(1, "VND", null)).toBe("$1.00");
  });
});

describe("parseRateResponse", () => {
  it("extracts rates.VND", () => {
    expect(parseRateResponse({ rates: { VND: 25123.4 } })).toBe(25123.4);
  });
  it("returns null on junk", () => {
    expect(parseRateResponse(null)).toBeNull();
    expect(parseRateResponse({ rates: { VND: "x" } })).toBeNull();
  });
});

describe("rate cache", () => {
  it("does not crash the dashboard when preference storage is blocked", () => {
    (globalThis as Record<string, unknown>).localStorage = {
      getItem: () => { throw new Error("blocked"); },
      setItem: () => { throw new Error("blocked"); },
    };
    expect(() => loadCachedRate()).not.toThrow();
    expect(loadCachedRate()).toBeNull();
    expect(() => saveCachedRate(25000, 1)).not.toThrow();
  });
  it("round-trips", () => {
    expect(loadCachedRate()).toBeNull();
    saveCachedRate(25000, 1756600000000);
    expect(loadCachedRate()).toEqual({ rate: 25000, ts: 1756600000000 });
  });
  it("survives corrupt JSON", () => {
    localStorage.setItem("usd_vnd_rate", "{nope");
    expect(loadCachedRate()).toBeNull();
  });
});
