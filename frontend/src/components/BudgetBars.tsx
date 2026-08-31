import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import type { BudgetWindow } from "../gen/gateway/v1/stats_pb";
import { barState } from "../lib/budget";
import { useCurrency } from "./CurrencyContext";

// validated status colors: ok #0369a1 · warn #d97706 · over #dc2626
const FILL = {
  ok: "bg-sky-700",
  warn: "bg-amber-600",
  over: "bg-red-600",
};
const LABEL = { ok: "", warn: "near limit", over: "over limit" };

function Bar({ label, w }: { label: string; w?: BudgetWindow }) {
  const { fmt } = useCurrency();
  const spent = w?.spentUsd ?? 0;
  const { pct, level, cap } = barState(spent, w?.warnUsd ?? 0, w?.hardUsd ?? 0);
  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between text-sm">
        <span className="font-medium text-slate-600">
          {label}
          {level !== "ok" && (
            <span
              className={`ml-2 rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-white ${FILL[level]}`}
            >
              {LABEL[level]}
            </span>
          )}
        </span>
        <span className="font-semibold tabular-nums text-sky-950">
          {fmt(spent)}
          {cap !== null && (
            <span className="font-normal text-slate-400"> / {fmt(cap)}</span>
          )}
        </span>
      </div>
      {cap !== null ? (
        <div className="h-2.5 overflow-hidden rounded-full bg-sky-100">
          <div
            className={`h-full rounded-full ${FILL[level]} transition-[width] duration-500`}
            style={{ width: `${pct}%` }}
          />
        </div>
      ) : (
        <div className="text-xs text-slate-400">no limit set</div>
      )}
    </div>
  );
}

export function BudgetBars() {
  const { data } = useQuery(StatsService.method.overview, {});
  return (
    <section className="grid h-full gap-5 rounded-2xl border border-sky-100 bg-white p-6 shadow-sm">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-sky-900">
        Budget
      </h2>
      <Bar label="Today" w={data?.today} />
      <Bar label="This week" w={data?.week} />
      <Bar label="This month" w={data?.month} />
    </section>
  );
}
