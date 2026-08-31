import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import { useCurrency } from "./CurrencyContext";

export function CacheStats() {
  const { data } = useQuery(StatsService.method.overview, {});
  const { fmt } = useCurrency();
  const items = [
    ["Total calls", Number(data?.totalCalls ?? 0n).toLocaleString()],
    ["Cache hits", Number(data?.cacheHits ?? 0n).toLocaleString()],
    ["Saved", fmt(data?.cacheSavedUsd ?? 0)],
  ] as const;
  return (
    <section className="grid grid-cols-3 gap-4 rounded-lg border border-gray-200 p-4">
      {items.map(([label, value]) => (
        <div key={label}>
          <div className="text-sm text-gray-500">{label}</div>
          <div className="text-xl font-semibold">{value}</div>
        </div>
      ))}
    </section>
  );
}
