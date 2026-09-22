import { useTranslation } from "react-i18next";
import { ProxyQueryStatus } from "./ProxyQueryStatus";
import { useQuery } from "@connectrpc/connect-query";
import { Card } from "@/components/ui/card";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import { formatNumber } from "@/i18n/format";
const compactTokens = (value: bigint) => formatNumber(Number(value), { notation: "compact", maximumFractionDigits: 1 });
import { useCurrency } from "../currency/useCurrency";

function Tile({
  id,
  update,
  label,
  value,
  sub,
  title,
}: {
  id: string;
  update: string | number | bigint;
  label: string;
  value: string;
  sub?: string;
  title?: string;
}) {
  return (
    <Card data-motion={`proxy-cache-${id}`} data-motion-update={String(update)} className="motion-card min-w-0 gap-0 border-0 bg-sky-50/60 p-4 shadow-none" title={title}>
      <div className="text-xs font-medium text-slate-500">{label}</div>
      <div className="mt-1 text-2xl font-bold tabular-nums tracking-tight text-sky-950 [overflow-wrap:anywhere]">
        {value}
      </div>
      {sub && <div className="mt-0.5 text-[13px] text-slate-400">{sub}</div>}
    </Card>
  );
}

export function CacheStats() {
  const { t } = useTranslation("usage");
  const { data, error, isPending } = useQuery(StatsService.method.overview, {});
  const { data: models, error: modelsError, isPending: modelsPending } = useQuery(StatsService.method.modelBreakdown, {
    days: 30,
  });
  const { fmt } = useCurrency();

  let input = 0n,
    output = 0n,
    cacheRd = 0n,
    cacheWr = 0n;
  for (const r of models?.rows ?? []) {
    input += r.inputTokens;
    output += r.outputTokens;
    cacheRd += r.cacheReadTokens;
    cacheWr += r.cacheWriteTokens;
  }

  const total = Number(data?.totalCalls ?? 0n);
  const hits = Number(data?.cacheHits ?? 0n);
  const rate = total > 0 ? formatNumber(hits / total, { style: "percent", minimumFractionDigits: 1, maximumFractionDigits: 1 }) : "—";

  if (!data || !models) return <ProxyQueryStatus error={error ?? modelsError} pending={isPending || modelsPending} />;
  return (
    <Card className="grid h-full grid-cols-[repeat(auto-fit,minmax(min(100%,11rem),1fr))] gap-4 p-6">
      <ProxyQueryStatus error={error ?? modelsError} />
      <Tile id="calls" update={total} label={t("totalCalls")} value={formatNumber(total)} sub={t("allTime")} />
      <Tile
        id="input" update={input}
        label={t("inputTokens")}
        value={compactTokens(input)}
        sub={t("last30")}
        title={t("inputTooltip")}
      />
      <Tile
        id="output" update={output}
        label={t("outputTokens")}
        value={compactTokens(output)}
        sub={t("last30")}
        title={t("outputTooltip")}
      />
      <Tile
        id="read" update={cacheRd}
        label={t("cacheRead")}
        value={compactTokens(cacheRd)}
        sub={t("last30")}
        title={t("readTooltip")}
      />
      <Tile
        id="write" update={cacheWr}
        label={t("cacheWrite")}
        value={compactTokens(cacheWr)}
        sub={t("last30")}
        title={t("writeTooltip")}
      />
      <Tile
        id="savings" update={`${data.cacheSavedUsd}:${hits}:${total}`}
        label={t("gatewayCache")}
        value={fmt(data?.cacheSavedUsd ?? 0)}
        sub={t("cacheHits", { hits: formatNumber(hits), rate })}
        title={t("gatewayCacheTooltip")}
      />
    </Card>
  );
}
