import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import type { BudgetWindow } from "../gen/gateway/v1/stats_pb";
import { barState } from "../lib/budget";
import { useCurrency } from "./CurrencyContext";

const COLORS = { ok: "bg-emerald-500", warn: "bg-amber-500", over: "bg-red-600" };

function Bar({ label, w }: { label: string; w?: BudgetWindow }) {
  const { fmt } = useCurrency();
  const spent = w?.spentUsd ?? 0;
  const { pct, level, cap } = barState(spent, w?.warnUsd ?? 0, w?.hardUsd ?? 0);
  return (
    <div>
      <div className="mb-1 flex justify-between text-sm">
        <span className="font-medium">{label}</span>
        <span>
          {fmt(spent)}
          {cap !== null && <span className="text-gray-400"> / {fmt(cap)}</span>}
        </span>
      </div>
      {cap !== null && (
        <div className="h-2 rounded bg-gray-200">
          <div
            className={`h-2 rounded ${COLORS[level]}`}
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
    </div>
  );
}

export function BudgetBars() {
  const { data } = useQuery(StatsService.method.overview, {});
  return (
    <section className="grid gap-4 rounded-lg border border-gray-200 p-4">
      <h2 className="font-semibold">Budget</h2>
      <Bar label="Today" w={data?.today} />
      <Bar label="This week" w={data?.week} />
      <Bar label="This month" w={data?.month} />
    </section>
  );
}
