import { useEffect, useState } from "react";
import {
  parseRateResponse,
  loadCachedRate,
  saveCachedRate,
} from "./currency";

const URL = "https://open.er-api.com/v6/latest/USD";
const HOUR = 60 * 60 * 1000;

export function useExchangeRate(): {
  rate: number | null;
  fetchedAt: Date | null;
} {
  const [state, setState] = useState(() => loadCachedRate());

  useEffect(() => {
    let cancelled = false;
    async function refresh() {
      try {
        const res = await fetch(URL);
        const rate = parseRateResponse(await res.json());
        if (rate !== null && !cancelled) {
          const next = { rate, ts: Date.now() };
          saveCachedRate(next.rate, next.ts);
          setState(next);
        }
      } catch {
        // network down — keep last-good rate
      }
    }
    const cached = loadCachedRate();
    if (!cached || Date.now() - cached.ts > HOUR) void refresh();
    const id = setInterval(refresh, HOUR);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  return {
    rate: state?.rate ?? null,
    fetchedAt: state ? new Date(state.ts) : null,
  };
}
