import { createContext, useContext, useState, type ReactNode } from "react";
import { formatMoney, type Currency } from "../lib/currency";
import { useExchangeRate } from "../hooks/useExchangeRate";

type Ctx = {
  currency: Currency;
  toggle: () => void;
  rate: number | null;
  fetchedAt: Date | null;
  fmt: (usd: number) => string;
};

const CurrencyCtx = createContext<Ctx | null>(null);

export function CurrencyProvider({ children }: { children: ReactNode }) {
  const [currency, setCurrency] = useState<Currency>("USD");
  const { rate, fetchedAt } = useExchangeRate();
  const value: Ctx = {
    currency,
    toggle: () => setCurrency((c) => (c === "USD" ? "VND" : "USD")),
    rate,
    fetchedAt,
    fmt: (usd) => formatMoney(usd, currency, rate),
  };
  return <CurrencyCtx.Provider value={value}>{children}</CurrencyCtx.Provider>;
}

export function useCurrency(): Ctx {
  const ctx = useContext(CurrencyCtx);
  if (!ctx) throw new Error("useCurrency outside CurrencyProvider");
  return ctx;
}
