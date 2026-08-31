import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import { useCurrency } from "./CurrencyContext";

export function CacheStats() {
  const { data } = useQuery(StatsService.method.overview, {});
  const { fmt } = useCurrency();
  const total = Number(data?.totalCalls ?? 0n);
  const hits = Number(data?.cacheHits ?? 0n);
  const rate = total > 0 ? ((hits / total) * 100).toFixed(1) + "%" : "—";
  const items = [
    ["Total calls", total.toLocaleString()],
    ["Cache hit rate", rate],
    ["Cache hits", hits.toLocaleString()],
    ["Saved by cache", fmt(data?.cacheSavedUsd ?? 0)],
  ] as const;
  return (
    <section className="grid h-full grid-cols-2 gap-4 rounded-2xl border border-sky-100 bg-white p-6 shadow-sm">
      {items.map(([label, value]) => (
        <div key={label} className="rounded-xl bg-sky-50/60 p-4">
          <div className="text-xs font-medium text-slate-500">{label}</div>
          <div className="mt-1 text-2xl font-bold tabular-nums tracking-tight text-sky-950">
            {value}
          </div>
        </div>
      ))}
    </section>
  );
}
