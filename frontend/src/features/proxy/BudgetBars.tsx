import { useTranslation } from "react-i18next";
import { ProxyQueryStatus } from "./ProxyQueryStatus";
import { useQuery } from "@connectrpc/connect-query";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import type { BudgetWindow } from "../../gen/gateway/v1/stats_pb";
import { barState } from "./budget";
import { useCurrency } from "../currency/useCurrency";

const FILL = {
  ok: "bg-sky-700",
  warn: "bg-amber-600",
  over: "bg-red-600",
};
const LABEL = { ok: "", warn: "nearLimit", over: "overLimit" };

function Bar({ label, w }: { label: string; w?: BudgetWindow }) {
  const { t } = useTranslation("usage");
  const { fmt } = useCurrency();
  const spent = w?.spentUsd ?? 0;
  const { pct, level, cap } = barState(spent, w?.warnUsd ?? 0, w?.hardUsd ?? 0);
  return (
    <div>
      <div className="mb-1.5 flex flex-wrap items-baseline justify-between gap-2 text-sm">
        <span className="font-medium text-slate-600">
          {label}
          {level !== "ok" && (
            <Badge
              className={`ml-2 rounded-full px-2 py-0.5 text-[12px] font-semibold uppercase tracking-wide text-white ${FILL[level]}`}
            >
              {t(LABEL[level])}
            </Badge>
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
        <Progress value={pct} aria-label={t("budgetLabel", { label })} aria-valuetext={t("budgetValue", { spent: fmt(spent), cap: fmt(cap) })} className={`h-2.5 bg-sky-100 ${level === "over" ? "[&>[data-slot=progress-indicator]]:bg-red-600" : level === "warn" ? "[&>[data-slot=progress-indicator]]:bg-amber-600" : "[&>[data-slot=progress-indicator]]:bg-sky-700"}`} />
      ) : (
        <div className="text-xs text-slate-400">{t("noLimit")}</div>
      )}
    </div>
  );
}

export function BudgetBars() {
  const { t } = useTranslation("usage");
  const { data, error, isPending } = useQuery(StatsService.method.overview, {});
  if (!data) return <ProxyQueryStatus error={error} pending={isPending} />;
  return (
    <Card data-motion="proxy-budget" data-motion-update={[data.today, data.week, data.month].map(window => `${window?.spentUsd ?? 0}:${window?.warnUsd ?? 0}:${window?.hardUsd ?? 0}`).join(";")} className="motion-card grid h-full gap-5 p-6">
      <ProxyQueryStatus error={error} />
      <h2 className="text-sm font-semibold uppercase tracking-wide text-sky-900">
        {t("budget")}
      </h2>
      <Bar label={t("today")} w={data?.today} />
      <Bar label={t("week")} w={data?.week} />
      <Bar label={t("month")} w={data?.month} />
    </Card>
  );
}
