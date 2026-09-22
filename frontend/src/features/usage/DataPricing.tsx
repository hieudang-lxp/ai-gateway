import { useTranslation } from "react-i18next";
import { formatDate, formatNumber } from "@/i18n/format";
import { sourceStateLabel, usageErrorMessage } from "./useUsageSummary";
import { useCurrency } from "../currency/useCurrency";
import { sourceNames, useUsageSummary } from "./useUsageSummary";
import { collectorIds, sourceIsFresh, useSyncClock } from "./syncStatus";
import { summarizeUsage } from "./usage";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

const card = "gap-0 p-5 sm:p-6";
const methods = {
  claude_code: { label: "estimated", explanation: "methodClaude" },
  codex: { label: "estimated", explanation: "methodCodex" },
  cursor: { label: "reportedCursor", explanation: "methodCursor" },
};

export function DataPricing({ period }: { period: string }) {
  const { t } = useTranslation("usage");
  const dateTime = (value: string | null | undefined) => value && !value.startsWith("0001") ? formatDate(value, { dateStyle: "medium", timeStyle: "short" }) : t("waitingSync");
  const now = useSyncClock();
  const { fmt } = useCurrency();
  const { data, error, isPending, isFetching, refetch } = useUsageSummary(period);
  const rows = data?.rows ?? [];
  const fallbackRows = rows.filter(row => row.fallback_cost_calls > 0);
  const fallbackCount = fallbackRows.reduce((sum, row) => sum + row.fallback_cost_calls, 0);
  const unpriced = rows.reduce((sum, row) => sum + row.unknown_cost_calls, 0);
  const ready = collectorIds.filter(id => !error && sourceIsFresh(data?.sources[id], now)).length;
  const priceIssue = data && (data.pricing.stale || !!data.pricing.error);

  return (
    <div className="grid grid-cols-1 gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">{t("pricingTitle")}</h1>
        </div>
        <Button variant="outline" disabled={isFetching} onClick={() => void refetch()}>{isFetching ? t("refreshing") : t("refreshStatus")}</Button>
      </div>

      {isPending && <Card role="status" className={`motion-loading ${card}`}>{t("checkingSources")}</Card>}
      {error && <Alert className="bg-amber-50 text-amber-900"><AlertDescription>{usageErrorMessage(error, t)}{data && ` ${t("snapshotWarning")}`}<details className="mt-2"><summary>{t("details")}</summary><p className="break-words">{error.message}</p></details></AlertDescription></Alert>}
      {data && <>
        <Alert role="status" aria-label={t("syncSummary")} className={`flex flex-wrap items-center justify-between gap-4 px-5 py-4 sm:px-6 ${ready === 3 ? "border-emerald-200 bg-emerald-50" : "border-amber-200 bg-amber-50"}`}>
          <div>
            <AlertTitle className={`font-semibold ${ready === 3 ? "text-emerald-900" : "text-amber-900"}`}>{error ? t("syncUnverified") : t("syncTools", { ready: formatNumber(ready) })}</AlertTitle>
            <AlertDescription className={`mt-1 text-xs ${ready === 3 ? "text-emerald-800" : "text-amber-800"}`}>{ready === 3 ? t("syncRunning") : t("syncCheck")}</AlertDescription>
          </div>
          <a href="#overview" className="shrink-0 text-sm font-medium text-sky-800 underline underline-offset-4">{t("viewUsage")}</a>
        </Alert>

        <section aria-labelledby="amounts-heading">
          <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
            <div><h2 id="amounts-heading" className="text-lg font-semibold text-sky-950">{t("amountSources")}</h2><p className="mt-1 text-xs text-slate-600">{period === "0" ? t("allHistory") : `${formatDate(data.since, { dateStyle: "medium", timeZone: "Asia/Ho_Chi_Minh" })} – ${formatDate(data.generated_at, { dateStyle: "medium", timeZone: "Asia/Ho_Chi_Minh" })}`} · {t("notInvoice")} · Asia/Ho_Chi_Minh</p></div>
            <a href="#overview" className="text-xs font-medium text-sky-700 underline underline-offset-4">{t("changePeriod")}</a>
          </div>
          <div className="grid gap-4 lg:grid-cols-3">
            {collectorIds.map(id => {
              const source = data.sources[id];
              const healthy = !error && sourceIsFresh(source, now);
              const sourceRows = rows.filter(row => row.source === id || (id === "claude_code" && row.source === "claude_gateway"));
              const sum = summarizeUsage(sourceRows);
              return <Card key={id} data-motion={`pricing-source-${id}`} data-motion-update={`${sum.calls}:${sum.cost}:${sum.unknown}`} className={`motion-card min-w-0 ${card}`}>
                <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-semibold text-sky-950">{sourceNames[id]}</h3><Badge variant="secondary" className={healthy ? "bg-emerald-50 text-emerald-800" : "bg-amber-50 text-amber-800"}>{healthy ? t("synced") : error ? t("unverified") : t("checkConnection")}</Badge></div>
                <p className="mt-5 break-words text-2xl font-semibold tracking-tight text-sky-950">{sum.calls > 0 && sum.unknown === sum.calls ? t("costUnavailable") : fmt(sum.cost)}</p>
                <p className={`mt-2 text-xs font-semibold ${id === "cursor" ? "text-emerald-800" : "text-sky-700"}`}>{t(methods[id].label)}</p>
                <p className="mt-3 text-sm leading-relaxed text-slate-600">{t(methods[id].explanation)}</p>
                {sum.unknown > 0 && <p className="mt-3 text-xs text-amber-800">{t("incompleteAmount", { value: formatNumber(sum.unknown) })}</p>}
                {sum.calls === 0 && <p className="mt-3 text-xs text-slate-600">{t("noRecords")}</p>}
                <p className="mt-5 border-t border-slate-100 pt-3 text-xs text-slate-500">{source?.last_success ? t("lastSynced", { time: dateTime(source.last_success) }) : t("waitingSync")}</p>
                {!healthy && !error && <p className="mt-3 text-xs text-amber-900">{t(id === "cursor" ? "checkCursor" : "checkLocal")}</p>}
              </Card>;
            })}
          </div>
        </section>

        <Card data-motion="pricing-accuracy" data-motion-update={`${unpriced}:${fallbackCount}`} aria-labelledby="accuracy-heading" className={card}>
          <h2 id="accuracy-heading" className="text-lg font-semibold text-sky-950">{t("accuracy")}</h2>
          <div className="mt-4 divide-y divide-slate-100">
            <div className="py-4 first:pt-0"><h3 className="font-medium text-slate-800">{unpriced === 0 ? t("allPriced") : t("unpricedEvents", { value: formatNumber(unpriced) })}</h3><p className="mt-1 text-sm text-slate-600">{t(unpriced === 0 ? "someEstimated" : "unpricedExplanation")}</p></div>
            <div className="py-4"><h3 className={`font-medium ${fallbackCount > 0 ? "text-amber-900" : "text-slate-800"}`}>{fallbackCount > 0 ? t("fallbackEvents", { value: formatNumber(fallbackCount) }) : t("noFallback")}</h3><p className="mt-1 text-sm text-slate-600">{fallbackCount > 0 ? t("fallbackExplanation", { model: data.pricing.fallback_model }) : t("noFallbackExplanation")}</p>{fallbackRows.length > 0 && <p className="mt-2 break-words text-xs text-amber-900">{t("affectedModels", { models: fallbackRows.map(row => t("modelEvents", { model: row.model, value: formatNumber(row.fallback_cost_calls) })).join(", ") })}</p>}</div>
            <div className="py-4 last:pb-0"><h3 className={`font-medium ${priceIssue ? "text-amber-900" : "text-slate-800"}`}>{t(priceIssue ? "pricesStale" : "pricesCurrent")}</h3><p className="mt-1 text-sm text-slate-600">{priceIssue ? t("pricesRetry") : t("priceRefresh", { time: dateTime(data.pricing.updated_at), minutes: formatNumber(data.pricing.refresh_seconds / 60) })}</p><p className="mt-2 text-xs text-slate-500">{t("priceHistory")}</p></div>
          </div>
        </Card>

        <Card data-motion="pricing-technical" className={card}><Accordion type="single" collapsible><AccordionItem value="technical" className="border-0">
          <AccordionTrigger className="text-sm font-semibold text-sky-800">{t("collectionDetails")}</AccordionTrigger>
          <AccordionContent>
          <div className="mt-5 grid gap-5 text-xs text-slate-600">
            <div className="grid gap-5 md:grid-cols-3">{collectorIds.map(id => { const source = data.sources[id]; return <div key={id}><h3 className="font-semibold text-slate-800">{sourceNames[id]}</h3><p className="mt-2">{id === "cursor" ? t("accountHistory", { days: formatNumber(data.cursor_history_days) }) : t("localFiles", { files: source?.files === undefined ? t("unknown") : formatNumber(source.files) })}</p><p className="mt-1">{t("polling", { seconds: source ? formatNumber(source.poll_seconds) : t("unknown") })}</p><p className="mt-1">{t("collectorState", { state: sourceStateLabel(source?.state, t) })}</p>{source?.error && <p className="mt-2 break-words text-amber-900">{source.error}</p>}</div>; })}</div>
            <p>{t("codexLimits")}</p>
            <p>{t("sourceLimits")}</p>
            <p>{t("collectionDelay")}</p>
            {data.pricing.error && <p className="text-amber-900">{t("priceError")}: {data.pricing.error}</p>}
            <a className="font-medium text-sky-700 underline underline-offset-4" href="https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json" target="_blank" rel="noreferrer">{t("openCatalog")} ↗</a>
          </div>
          </AccordionContent>
        </AccordionItem></Accordion></Card>
      </>}
    </div>
  );
}
