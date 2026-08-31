import { useState } from "react";
import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import { useCurrency } from "./CurrencyContext";

export function RecentCalls() {
  const [beforeId, setBeforeId] = useState(0n);
  const { data } = useQuery(StatsService.method.recentCalls, {
    limit: 50,
    beforeId,
  });
  const { fmt } = useCurrency();
  const calls = data?.calls ?? [];
  return (
    <section className="rounded-lg border border-gray-200 p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="font-semibold">Recent calls</h2>
        <div className="flex gap-2 text-sm">
          {beforeId !== 0n && (
            <button
              className="rounded border border-gray-300 px-2 py-0.5"
              onClick={() => setBeforeId(0n)}
            >
              Latest
            </button>
          )}
          {calls.length === 50 && (
            <button
              className="rounded border border-gray-300 px-2 py-0.5"
              onClick={() => setBeforeId(calls[calls.length - 1].id)}
            >
              Older →
            </button>
          )}
        </div>
      </div>
      <table className="w-full text-sm">
        <thead className="text-left text-gray-500">
          <tr>
            <th className="py-1">Time</th>
            <th>Model</th>
            <th className="text-right">In</th>
            <th className="text-right">Out</th>
            <th className="text-right">Cost</th>
            <th className="text-right">ms</th>
            <th className="text-right">Status</th>
          </tr>
        </thead>
        <tbody>
          {calls.map((c) => (
            <tr key={String(c.id)} className="border-t border-gray-100">
              <td className="py-1 whitespace-nowrap">
                {new Date(Number(c.tsUnix) * 1000).toLocaleString("vi-VN")}
              </td>
              <td className="font-mono text-xs">
                {c.model}
                {c.routedFrom && (
                  <span className="ml-1 rounded bg-blue-50 px-1 text-blue-700">
                    ← {c.routedFrom}
                  </span>
                )}
                {c.cacheHit && (
                  <span className="ml-1 rounded bg-emerald-50 px-1 text-emerald-700">
                    cache {c.savedUsd > 0 && `+${fmt(c.savedUsd)}`}
                  </span>
                )}
              </td>
              <td className="text-right">{Number(c.inputTokens).toLocaleString()}</td>
              <td className="text-right">{Number(c.outputTokens).toLocaleString()}</td>
              <td className="text-right">{fmt(c.costUsd)}</td>
              <td className="text-right">{Number(c.latencyMs)}</td>
              <td
                className={`text-right ${c.status >= 400 ? "text-red-600" : ""}`}
              >
                {c.status}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {calls.length === 0 && (
        <div className="py-4 text-center text-sm text-gray-400">No calls yet</div>
      )}
    </section>
  );
}
