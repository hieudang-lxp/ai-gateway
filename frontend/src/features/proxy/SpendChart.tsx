import { useQuery } from "@connectrpc/connect-query";
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from "recharts";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import { useCurrency } from "../currency/useCurrency";

// single-series mark: #0369a1 (validated vs white surface)
const OCEAN = "#0369a1";

export function SpendChart() {
  const { data } = useQuery(StatsService.method.spendSeries, { days: 30 });
  const { fmt } = useCurrency();
  const points = (data?.points ?? []).map((p) => ({
    date: p.date.slice(5), // MM-DD
    cost: p.costUsd,
    calls: Number(p.calls),
  }));
  return (
    <section className="rounded-2xl border border-sky-100 bg-white p-6 shadow-sm">
      <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-sky-900">
        Spend — last 30 days
      </h2>
      <ResponsiveContainer width="100%" height={240}>
        <BarChart data={points} barCategoryGap="25%">
          <CartesianGrid vertical={false} stroke="#e0f2fe" />
          <XAxis
            dataKey="date"
            fontSize={11}
            interval={4}
            tickLine={false}
            axisLine={{ stroke: "#bae6fd" }}
            tick={{ fill: "#64748b" }}
          />
          <YAxis
            fontSize={11}
            tickFormatter={(v: number) => fmt(v)}
            width={90}
            tickLine={false}
            axisLine={false}
            tick={{ fill: "#64748b" }}
          />
          <Tooltip
            cursor={{ fill: "#f0f9ff" }}
            contentStyle={{
              borderRadius: 12,
              border: "1px solid #bae6fd",
              boxShadow: "0 4px 12px rgba(12,74,110,0.08)",
              fontSize: 12,
            }}
            labelStyle={{ color: "#0c4a6e", fontWeight: 600 }}
            formatter={(v, name) =>
              name === "cost" ? [fmt(Number(v)), "cost"] : [v, name]
            }
          />
          <Bar
            dataKey="cost"
            fill={OCEAN}
            radius={[4, 4, 0, 0]}
            isAnimationActive={false}
          />
        </BarChart>
      </ResponsiveContainer>
    </section>
  );
}
