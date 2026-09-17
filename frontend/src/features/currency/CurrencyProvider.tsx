import { useState, type ReactNode } from "react";
import { formatMoney, type Currency } from "./currency";
import { useExchangeRate } from "./useExchangeRate";

import { CurrencyCtx, type CurrencyContextValue } from "./useCurrency";

export function CurrencyProvider({ children }: { children: ReactNode }) {
  const [currency, setCurrency] = useState<Currency>("USD");
  const { rate, fetchedAt } = useExchangeRate();
  const value: CurrencyContextValue = {
    currency,
    toggle: () => setCurrency((c) => (c === "USD" ? "VND" : "USD")),
    rate,
    fetchedAt,
    fmt: (usd) => formatMoney(usd, currency, rate),
  };
  return <CurrencyCtx.Provider value={value}>{children}</CurrencyCtx.Provider>;
}
