import { useQuery } from "@connectrpc/connect-query";
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
} from "recharts";
import { StatsService } from "../gen/gateway/v1/stats_pb";
import { useCurrency } from "./CurrencyContext";

export function SpendChart() {
  const { data } = useQuery(StatsService.method.spendSeries, { days: 30 });
  const { fmt } = useCurrency();
  const points = (data?.points ?? []).map((p) => ({
    date: p.date.slice(5), // MM-DD
    cost: p.costUsd,
    calls: Number(p.calls),
  }));
  return (
    <section className="rounded-lg border border-gray-200 p-4">
      <h2 className="mb-3 font-semibold">Spend — last 30 days</h2>
      <ResponsiveContainer width="100%" height={220}>
        <BarChart data={points}>
          <XAxis dataKey="date" fontSize={11} interval={4} />
          <YAxis fontSize={11} tickFormatter={(v: number) => fmt(v)} width={90} />
          <Tooltip
            formatter={(v, name) =>
              name === "cost" ? [fmt(Number(v)), "cost"] : [v, name]
            }
          />
          <Bar dataKey="cost" fill="#111827" radius={[3, 3, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </section>
  );
}
