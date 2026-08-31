import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import { useCurrency } from "./CurrencyContext";

const n = (v: bigint) => Number(v).toLocaleString();

export function ModelTable() {
  const { data } = useQuery(StatsService.method.modelBreakdown, { days: 30 });
  const { fmt } = useCurrency();
  return (
    <section className="rounded-lg border border-gray-200 p-4">
      <h2 className="mb-3 font-semibold">Models — last 30 days</h2>
      <table className="w-full text-sm">
        <thead className="text-left text-gray-500">
          <tr>
            <th className="py-1">Model</th>
            <th className="text-right">Calls</th>
            <th className="text-right">Input</th>
            <th className="text-right">Output</th>
            <th className="text-right">Cache rd</th>
            <th className="text-right">Cache wr</th>
            <th className="text-right">Cost</th>
          </tr>
        </thead>
        <tbody>
          {(data?.rows ?? []).map((r) => (
            <tr key={r.model} className="border-t border-gray-100">
              <td className="py-1 font-mono text-xs">{r.model}</td>
              <td className="text-right">{n(r.calls)}</td>
              <td className="text-right">{n(r.inputTokens)}</td>
              <td className="text-right">{n(r.outputTokens)}</td>
              <td className="text-right">{n(r.cacheReadTokens)}</td>
              <td className="text-right">{n(r.cacheWriteTokens)}</td>
              <td className="text-right">{fmt(r.costUsd)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {(data?.rows ?? []).length === 0 && (
        <div className="py-4 text-center text-sm text-gray-400">No calls yet</div>
      )}
    </section>
  );
}
