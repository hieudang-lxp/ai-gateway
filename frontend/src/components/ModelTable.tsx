import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import { useCurrency } from "./CurrencyContext";

const n = (v: bigint) => Number(v).toLocaleString();

export function ModelTable() {
  const { data } = useQuery(StatsService.method.modelBreakdown, { days: 30 });
  const { fmt } = useCurrency();
  const rows = data?.rows ?? [];
  return (
    <section className="rounded-2xl border border-sky-100 bg-white p-6 shadow-sm">
      <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-sky-900">
        Models — last 30 days
      </h2>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-sky-100 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
              <th className="py-2">Model</th>
              <th className="text-right">Calls</th>
              <th className="text-right">Input</th>
              <th className="text-right">Output</th>
              <th className="text-right">Cache rd</th>
              <th className="text-right">Cache wr</th>
              <th className="text-right">Cost</th>
            </tr>
          </thead>
          <tbody className="tabular-nums">
            {rows.map((r) => (
              <tr
                key={r.model}
                className="border-b border-sky-50 transition-colors last:border-0 hover:bg-sky-50/50"
              >
                <td className="py-2.5 font-mono text-xs text-sky-900">
                  {r.model}
                </td>
                <td className="text-right">{n(r.calls)}</td>
                <td className="text-right">{n(r.inputTokens)}</td>
                <td className="text-right">{n(r.outputTokens)}</td>
                <td className="text-right">{n(r.cacheReadTokens)}</td>
                <td className="text-right">{n(r.cacheWriteTokens)}</td>
                <td className="text-right font-semibold text-sky-950">
                  {fmt(r.costUsd)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {rows.length === 0 && (
        <div className="py-6 text-center text-sm text-slate-400">
          No calls yet
        </div>
      )}
    </section>
  );
}
