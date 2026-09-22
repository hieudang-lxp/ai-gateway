import { useTranslation } from "react-i18next";
import { formatDate, formatNumber } from "@/i18n/format";
import { sourceStateLabel, usageErrorMessage } from "./useUsageSummary";
import { collectorIds, sourceIsFresh, useSyncClock } from "./syncStatus";
import { summarizeUsage } from "./usage";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useCurrency } from "../currency/useCurrency";
import { sourceNames as names, useUsageSummary } from "./useUsageSummary";

const number = formatNumber;
const short = (n: number) => formatNumber(n, { notation: "compact", maximumFractionDigits: 2 });
const date = (ts: number) => formatDate(ts * 1000, { dateStyle: "medium" });

export function UnifiedUsage({ period, setPeriod }: { period: string; setPeriod: (period: string) => void }) {
  const { t } = useTranslation("usage");
  const now = useSyncClock();
  const { fmt } = useCurrency();
  const { data, error, isPending, isFetching } = useUsageSummary(period);
  const rows = data?.rows ?? [];
  const total = summarizeUsage(rows);
  const healthy = !error && data && collectorIds.every(id => sourceIsFresh(data.sources[id], now));
  return (
    <Card className="gap-0 border-sky-200 p-6">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-sky-950">{t("usageTitle")}</h2>
        </div>
        <div className="flex max-w-full flex-wrap items-center gap-3">
          <Label htmlFor="usage-period" className="text-sm text-slate-600">{t("period")}</Label>
          <Select value={period} onValueChange={setPeriod}>
            <SelectTrigger id="usage-period" aria-label={t("usagePeriod")} className="h-11 w-56 max-w-full px-4 text-sm"><SelectValue /></SelectTrigger>
            <SelectContent position="popper" sideOffset={6}>
              <SelectItem value="month">{t("month")}</SelectItem>
              <SelectItem value="1">{t("today")}</SelectItem>
              <SelectItem value="7">{t("last7")}</SelectItem>
              <SelectItem value="30">{t("last30")}</SelectItem>
              <SelectItem value="0">{t("allHistory")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      {isPending && <p className="motion-loading text-sm text-slate-500" role="status">{t("loadingUsage")}</p>}
      {error && <Alert className="bg-amber-50 text-amber-900"><AlertDescription>{usageErrorMessage(error, t)}<details className="mt-2"><summary>{t("details")}</summary><p className="break-words">{error.message}</p></details></AlertDescription></Alert>}
      {data && <>
        <p className="mb-4 text-xs text-slate-500">{period === "0" ? t("allHistory") : `${formatDate(data.since, { dateStyle: "medium", timeZone: "Asia/Ho_Chi_Minh" })} – ${formatDate(data.generated_at, { dateStyle: "medium", timeZone: "Asia/Ho_Chi_Minh" })} · Asia/Ho_Chi_Minh`}</p>
        <div className="mb-5 grid gap-3 sm:grid-cols-3">
          {[{ id: "tokens", update: total.tokens, label: t("collectedTokens"), value: short(total.tokens), detail: t("exactTokens", { value: number(total.tokens) }) },
            { id: "events", update: total.calls, label: t("recordedEvents"), value: number(total.calls), detail: t("eventGrouping") },
            { id: "cost", update: `${total.cost}:${total.unknown}`, label: t("knownValue"), value: fmt(total.cost), detail: total.unknown ? t("unpricedEvents", { value: number(total.unknown) }) : t("notInvoice") },
          ].map(card => <Card key={card.id} data-motion={`usage-summary-${card.id}`} data-motion-update={card.update} className="motion-card min-w-0 gap-0 border-0 bg-sky-50 p-4 shadow-none"><p className="text-xs text-slate-600">{card.label}</p><p className="my-1 text-2xl font-bold text-sky-950">{card.value}</p><p className="text-xs text-slate-500">{card.detail}</p></Card>)}
        </div>
        <div className="mb-5 grid gap-3 md:grid-cols-3">
          {["claude_code", "codex", "cursor"].map(source => {
            const sourceRows = rows.filter(r => r.source === source || (source === "claude_code" && r.source === "claude_gateway"));
            const sum = summarizeUsage(sourceRows); const status = data.sources[source];
            return <Card key={source} data-motion={`usage-source-${source}`} data-motion-update={`${sum.tokens}:${sum.calls}:${sum.cost}:${sum.unknown}`} className="motion-card min-w-0 gap-0 p-4 shadow-none">
              <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-semibold text-slate-800">{names[source]}</h3><Badge variant="secondary" className={!error && sourceIsFresh(status, now) ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-700"}>{error ? t("unverified") : status?.state === "ok" && !sourceIsFresh(status, now) ? t("stale") : sourceStateLabel(status?.state, t)}</Badge></div>
              <p className="mt-2 text-xl font-semibold text-sky-950">{short(sum.tokens)} <span className="text-xs font-normal text-slate-500">{t("tokens")}</span></p>
              <p className="mt-1 text-xs text-slate-500">{sum.unknown === sum.calls && sum.calls > 0 ? t("costUnavailable") : t(source === "codex" ? "estimatedValue" : "reportedValue", { value: fmt(sum.cost) })}{sum.unknown > 0 && ` · ${t("unpriced", { value: number(sum.unknown) })}`}</p>
              {sourceRows.length > 0 && <p className="mt-2 text-xs text-slate-500">{t("recordedRange", { start: date(Math.min(...sourceRows.map(r => r.first_ts))), end: date(Math.max(...sourceRows.map(r => r.last_ts))) })}</p>}
              <p className="mt-2 text-xs text-slate-500">{t("polling", { seconds: status ? number(status.poll_seconds) : t("unknown") })} · {status?.last_success ? t("lastSynced", { time: formatDate(status.last_success, { timeStyle: "medium" }) }) : t("waitingSync")}</p>
              {status?.error && <details className="mt-2 text-xs text-amber-800"><summary>{t("details")}</summary><p className="break-words">{status.error}</p></details>}
            </Card>;
          })}
        </div>
        {!healthy && <Alert role="status" className="mb-4 bg-amber-50 text-amber-900"><AlertDescription>{t("collectorsAttention")}</AlertDescription></Alert>}
        <p className="mb-4 text-xs leading-relaxed text-slate-500">{t("notInvoice")}. <a href="#data-pricing" className="font-medium text-sky-700 underline underline-offset-4">{t("pricingLink")}</a></p>
        <Accordion type="single" collapsible><AccordionItem value="models" className="border-0">
          <AccordionTrigger className="text-sm font-medium text-sky-800">{t("modelBreakdown")} {isFetching && `· ${t("updating")}`}</AccordionTrigger>
          <AccordionContent>
          <div className="mt-3 overflow-x-auto"><Table className="w-full text-right text-xs tabular-nums"><TableHeader className="border-b border-sky-100 text-slate-500"><TableRow><TableHead className="py-2 text-left">{t("sourceModel")}</TableHead><TableHead className="text-right">{t("events")}</TableHead><TableHead className="text-right">{t("input")}</TableHead><TableHead className="text-right">{t("output")}</TableHead><TableHead className="text-right">{t("cacheRead")}</TableHead><TableHead className="text-right">{t("cacheWrite")}</TableHead><TableHead className="text-right">{t("knownValue")}</TableHead></TableRow></TableHeader><TableBody>
            {rows.map(r => <TableRow key={`${r.source}:${r.model}`} className="border-b border-sky-50"><TableCell className="py-2 pr-3 text-left">{names[r.source] ?? r.source}<br/><span className="font-mono text-slate-500">{r.model || t("unknown")}</span></TableCell><TableCell>{number(r.calls)}</TableCell><TableCell>{number(r.input_tokens)}</TableCell><TableCell>{number(r.output_tokens)}</TableCell><TableCell>{number(r.cache_read_tokens)}</TableCell><TableCell>{number(r.cache_write_tokens)}</TableCell><TableCell>{r.unknown_cost_calls === r.calls ? t("unavailable") : fmt(r.known_cost_usd)}{r.estimated_cost_calls > 0 && ` · ${t("estimated")}`}{r.fallback_cost_calls > 0 && ` · ${t("fallback")}`}</TableCell></TableRow>)}
          </TableBody></Table></div>
          {rows.length === 0 && <p className="py-4 text-sm text-slate-500">{t("noRecords")}</p>}
          </AccordionContent>
        </AccordionItem></Accordion>
      </>}
    </Card>
  );
}
