import { formatDate } from "@/i18n/format";
import { useTranslation } from "react-i18next";
import { ProxyQueryStatus } from "./ProxyQueryStatus";
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
  const { t } = useTranslation("usage");
  const { data, error, isPending } = useQuery(StatsService.method.spendSeries, { days: 30 });
  const { fmt } = useCurrency();
  const points = (data?.points ?? []).map((p) => ({
    date: formatDate(new Date(`${p.date}T12:00:00`), { month: "short", day: "numeric" }),
    cost: p.costUsd,
    calls: Number(p.calls),
  }));
  if (!data) return <ProxyQueryStatus error={error} pending={isPending} />;
  return (
    <Card data-motion="proxy-spend-chart" className="gap-0 p-6">
      <ProxyQueryStatus error={error} />
      <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-sky-900">
        {t("spend30")}
      </h2>
      {points.length === 0 && <p className="py-4 text-sm text-slate-500">{t("noCalls")}</p>}
      <ChartContainer config={{ cost: { label: t("cost"), color: OCEAN } }} className="h-60 w-full">
        <BarChart data={points} barCategoryGap="25%" aria-label={t("spend30")}>
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
            content={<ChartTooltipContent formatter={value => <span className="flex w-full justify-between gap-4"><span>{t("cost")}</span><span className="font-semibold tabular-nums">{fmt(Number(value))}</span></span>} />}
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
