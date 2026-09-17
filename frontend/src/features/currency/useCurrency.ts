import { createContext, useContext } from "react";
import type { Currency } from "./currency";

export type CurrencyContextValue = {
  currency: Currency;
  toggle: () => void;
  rate: number | null;
  fetchedAt: Date | null;
  fmt: (usd: number) => string;
};

export const CurrencyCtx = createContext<CurrencyContextValue | null>(null);

export function useCurrency(): CurrencyContextValue {
  const ctx = useContext(CurrencyCtx);
  if (!ctx) throw new Error("useCurrency outside CurrencyProvider");
  return ctx;
}
