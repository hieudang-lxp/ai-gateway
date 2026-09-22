import { formatDate } from '@/i18n/format';
import { useTranslation } from 'react-i18next';
import { ArrowUpRight, ArrowLeft } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useCurrency } from "../currency/useCurrency";
import { useInsights } from "../sessions/hooks";
import { changeLabel } from "../sessions/query";
import { Coverage, LoadError, PeriodSelect, SessionRow } from "../sessions/shared";
import { count } from "../sessions/format";
import { UsageNetwork } from "./UsageNetwork";
export function Insights({ period, setPeriod, expanded=false }: { period: string; setPeriod: (value: string) => void; expanded?: boolean }) {
  const { t } = useTranslation('sessions');
  const { fmt } = useCurrency(); const result = useInsights(period); const data = result.data;
  if (expanded) {
    const requested = new URLSearchParams(window.location.hash.split('?')[1]).get('metric');
    const initialMetric = requested === 'calls' || requested === 'total_tokens' ? requested : 'known_cost_usd';
    return <div className="space-y-4"><div data-motion="insights-network-header" className="flex flex-wrap items-center justify-between gap-4"><div className="flex flex-wrap items-center gap-4"><Button asChild variant="ghost" className="text-slate-200 hover:bg-white/10 hover:text-white"><a href="#insights"><ArrowLeft className="size-4"/>{t("backInsights")}</a></Button><h1 className="text-lg font-semibold text-white">{t("networkExplorer")}</h1></div><div className="rounded-lg bg-white px-3 py-2"><PeriodSelect period={period} onChange={setPeriod}/></div></div>{result.isPending&&<p role="status" className="motion-loading text-slate-200">{t("networkLoading")}</p>}{result.error&&<LoadError error={result.error} retry={()=>void result.refetch()}/>} {data&&<UsageNetwork key={period} rows={data.network??[]} expanded initialMetric={initialMetric}/>}</div>;
  }
  return <div className="space-y-6"><div data-motion="insights-header" className="flex flex-wrap items-end justify-between gap-4"><div><p className="mb-2 text-xs font-semibold uppercase tracking-widest text-sky-700">{t("insights")}</p><h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">{t("networkExplorer")}</h1><p className="mt-2 text-sm text-slate-500">{t("exploreSessions")}</p></div><PeriodSelect period={period} onChange={setPeriod} /></div>{result.isPending && <p role="status" className="motion-loading text-sm text-slate-500">{t("insightsLoading")}</p>}{result.error && <LoadError error={result.error} retry={() => void result.refetch()} />}{data && <>
    <p className="text-xs text-slate-500">{formatDate(data.since)} – {formatDate(data.until)}{period !== "0" && <> · {t("comparedInterval", { date: formatDate(data.previous_since) })}</>}</p>
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{[{ id: "sessions", label: t("indexedSessions"), value: count(data.totals.sessions), current: data.totals.sessions, previous: data.previous.sessions }, { id: "calls", label: t("recordedEvents"), value: count(data.totals.calls), current: data.totals.calls, previous: data.previous.calls }, { id: "tokens", label: t("tokensIncludingCache"), value: count(data.totals.total_tokens), current: data.totals.total_tokens, previous: data.previous.total_tokens }, { id: "value", label: t("knownUsage"), value: fmt(data.totals.known_cost_usd), current: data.totals.known_cost_usd, previous: data.previous.known_cost_usd }].map(stat => <Card key={stat.id} data-motion={`insights-stat-${stat.id}`} data-motion-update={`${stat.current}:${stat.previous}`} className="motion-card min-w-0 gap-2 p-4"><p className="text-xs text-slate-500">{stat.label}</p><p className="break-words text-xl font-semibold tabular-nums text-sky-950">{stat.value}</p><p className="text-xs text-slate-500">{period === "0" ? t("indexedHistory") : changeLabel(stat.current, stat.previous)}</p></Card>)}</div>
    <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500"><Badge variant="secondary">{t("apiEstimates")}</Badge><span>{t("pricingHelp", { count: data.totals.unknown_cost_calls, amount: count(data.totals.unknown_cost_calls) })}</span><a href="#data-pricing" className="text-sky-700 underline underline-offset-4">{t("pricingAssumptions")}</a></div>
    <UsageNetwork key={period} rows={data.network ?? []} />
    <section data-motion="insights-top-sessions" aria-labelledby="top-sessions-title" className="space-y-3"><div className="flex flex-wrap items-center justify-between gap-3"><h2 id="top-sessions-title" className="font-semibold text-sky-950">{t("exploreSessions")}</h2><Button asChild variant="ghost" size="sm"><a href="#sessions">{t("allSessions")} <ArrowUpRight className="size-4" /></a></Button></div><Card className="gap-0 px-4 py-1 sm:px-5">{data.top_sessions.length ? data.top_sessions.map(session => <SessionRow key={`${session.source}:${session.session_id}`} session={session} />) : <p className="py-6 text-sm text-slate-500">{t("noAttributed")}</p>}</Card></section><Coverage unassigned={data.unassigned_events} indexedAt={data.indexed_at} />
  </>}</div>;
}
