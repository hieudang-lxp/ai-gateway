import { useState } from "react";
import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import { compactTokens } from "../../lib/format";
import { useCurrency } from "../currency/useCurrency";

const pagerBtn =
  "rounded-lg border border-sky-200 bg-white px-3 py-1 text-xs font-medium text-sky-800 shadow-sm transition-colors hover:bg-sky-50";

export function RecentCalls() {
  const [beforeId, setBeforeId] = useState(0n);
  const { data } = useQuery(StatsService.method.recentCalls, {
    limit: 50,
    beforeId,
  });
  const { fmt } = useCurrency();
  const calls = data?.calls ?? [];
  return (
    <section className="rounded-2xl border border-sky-100 bg-white p-6 shadow-sm">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-sky-900">
          Recent calls
        </h2>
        <div className="flex gap-2">
          {beforeId !== 0n && (
            <button className={pagerBtn} onClick={() => setBeforeId(0n)}>
              ↩ Latest
            </button>
          )}
          {calls.length === 50 && (
            <button
              className={pagerBtn}
              onClick={() => setBeforeId(calls[calls.length - 1].id)}
            >
              Older →
            </button>
          )}
        </div>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-sky-100 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
              <th className="py-2">Time</th>
              <th>Model</th>
              <th className="text-right" title="Input tokens (không tính cache)">
                Input
              </th>
              <th className="text-right" title="Output tokens">
                Output
              </th>
              <th className="text-right" title="Prompt-cache read tokens">
                Cache rd
              </th>
              <th className="text-right" title="Prompt-cache write tokens">
                Cache wr
              </th>
              <th className="text-right">Cost</th>
              <th className="text-right">ms</th>
              <th className="text-right">Status</th>
            </tr>
          </thead>
          <tbody className="tabular-nums">
            {calls.map((c) => (
              <tr
                key={String(c.id)}
                className="border-b border-sky-50 transition-colors last:border-0 hover:bg-sky-50/50"
              >
                <td className="py-2.5 whitespace-nowrap text-xs text-slate-500">
                  {new Date(Number(c.tsUnix) * 1000).toLocaleString("vi-VN")}
                </td>
                <td className="font-mono text-xs text-sky-900">
                  {c.model}
                  {c.routedFrom && (
                    <span className="ml-1.5 rounded-full bg-sky-100 px-2 py-0.5 text-[10px] font-semibold text-sky-800">
                      ← {c.routedFrom}
                    </span>
                  )}
                  {c.cacheHit && (
                    <span className="ml-1.5 rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-800">
                      cache{c.savedUsd > 0 && ` +${fmt(c.savedUsd)}`}
                    </span>
                  )}
                </td>
                <td className="text-right">
                  {Number(c.inputTokens).toLocaleString()}
                </td>
                <td className="text-right">
                  {Number(c.outputTokens).toLocaleString()}
                </td>
                <td className="text-right text-slate-500">
                  {compactTokens(c.cacheReadTokens)}
                </td>
                <td className="text-right text-slate-500">
                  {compactTokens(c.cacheWriteTokens)}
                </td>
                <td className="text-right font-semibold text-sky-950">
                  {fmt(c.costUsd)}
                </td>
                <td className="text-right text-slate-500">
                  {Number(c.latencyMs)}
                </td>
                <td className="text-right">
                  <span
                    className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                      c.status >= 400
                        ? "bg-red-100 text-red-700"
                        : "bg-sky-50 text-sky-700"
                    }`}
                  >
                    {c.status}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {calls.length === 0 && (
        <div className="py-6 text-center text-sm text-slate-400">
          No calls yet
        </div>
      )}
    </section>
  );
}
