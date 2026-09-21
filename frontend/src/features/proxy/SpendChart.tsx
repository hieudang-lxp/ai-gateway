import { useQuery } from "@connectrpc/connect-query";
import { Card } from "@/components/ui/card";
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
} from "recharts";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import { useCurrency } from "../currency/useCurrency";

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
    <Card className="gap-0 p-6">
      <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-sky-900">
        Spend — last 30 days
      </h2>
      <ChartContainer config={{ cost: { label: "Cost", color: OCEAN } }} className="h-60 w-full">
        <BarChart data={points} barCategoryGap="25%">
          <CartesianGrid vertical={false} stroke="#e0f2fe" />
          <XAxis
            dataKey="date"
            fontSize={13}
            interval={4}
            tickLine={false}
            axisLine={{ stroke: "#bae6fd" }}
            tick={{ fill: "#64748b" }}
          />
          <YAxis
            fontSize={13}
            tickFormatter={(v: number) => fmt(v)}
            width={105}
            tickLine={false}
            axisLine={false}
            tick={{ fill: "#64748b" }}
          />
          <ChartTooltip
            cursor={{ fill: "#f0f9ff" }}
            content={<ChartTooltipContent formatter={value => <span className="flex w-full justify-between gap-4"><span>Cost</span><span className="font-semibold tabular-nums">{fmt(Number(value))}</span></span>} />}
          />
          <Bar
            dataKey="cost"
            fill={OCEAN}
            radius={[4, 4, 0, 0]}
            isAnimationActive={false}
          />
        </BarChart>
      </ChartContainer>
    </Card>
  );
}
