import { formatNumber } from '@/i18n/format';
import { useTranslation } from 'react-i18next';
import { useState } from "react";
import { Search, ArrowLeft, RefreshCw } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useHash } from "../../lib/navigation";
import { useCurrency } from "../currency/useCurrency";
import { sessionSelection } from "./query";
import { useSessions, useSessionDetail, useSessionModels } from "./hooks";
import { Coverage, LoadError, PeriodSelect, SessionRow, SessionValue } from "./shared";
import { count, sourceName, timestamp, tokens, sessionTitle } from "./format";

import { ModelSelect } from "./ModelSelect";
import { modelsForSource, modelForSource } from "./models";

export function Sessions({ period, setPeriod }: { period: string; setPeriod: (value: string) => void }) {
  const { t } = useTranslation('sessions');
  const hash = useHash();
  const selected = sessionSelection(hash);
  return <div className="space-y-6"><div data-motion="sessions-header"><p className="mb-2 text-xs font-semibold uppercase tracking-widest text-sky-700">{t("sessions")}</p><h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">{t("searchMetadata")}</h1><p className="mt-2 text-sm text-slate-500">{t("searchHelp")}</p></div>{selected ? <SessionDetailView key={`${selected.source}:${selected.sessionID}`} source={selected.source} sessionID={selected.sessionID} /> : <SessionList period={period} setPeriod={setPeriod} />}</div>;
}
function SessionList({ period, setPeriod }: { period: string; setPeriod: (value: string) => void }) {
  const { t } = useTranslation('sessions');
  const [draft, setDraft] = useState(""); const [q, setQuery] = useState("");
  const [source, setSource] = useState("all"); const [model, setModel] = useState("");
  const catalog = useSessionModels();
  const options = catalog.data?.models ?? [];
  const models = modelsForSource(options, source);
  function changeSource(next: string) {
    setSource(next);
    setModel(current => modelForSource(options, next, current));
  }
  const result = useSessions({ period, q, source: source === "all" ? "" : source, model });
  const first = result.data?.pages[0];
  const sessions = result.data?.pages.flatMap(page => page.sessions) ?? [];
  return <><Card data-motion="sessions-filters" className="gap-4 p-4 sm:p-5"><div className="flex flex-wrap items-center justify-between gap-3"><PeriodSelect period={period} onChange={setPeriod} /><Button variant="ghost" size="sm" disabled={result.isFetching} onClick={() => { void result.refetch(); void catalog.refetch(); }}><RefreshCw className="size-4" /> {t("refresh")}</Button></div><form className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-[minmax(0,2fr)_10rem_minmax(0,1fr)_auto]" onSubmit={event => { event.preventDefault(); setQuery(draft.trim()); }}><div className="grid gap-2"><Label htmlFor="session-search">{t("searchMetadata")}</Label><Input id="session-search" value={draft} onChange={event => setDraft(event.target.value)} placeholder={t("searchPlaceholder")} /></div><div className="grid gap-2"><Label htmlFor="session-source">{t("source")}</Label><Select value={source} onValueChange={changeSource}><SelectTrigger id="session-source" className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t("allSources")}</SelectItem><SelectItem value="codex">Codex</SelectItem><SelectItem value="claude_code">Claude Code</SelectItem><SelectItem value="cursor">Cursor</SelectItem></SelectContent></Select></div><div className="grid gap-2"><Label htmlFor="session-model">{t("model")}</Label><ModelSelect models={models} value={model} onChange={setModel} loading={catalog.isPending} failed={catalog.isError} /></div><Button type="submit"><Search className="size-4" /> {t("search")}</Button></form><p className="text-xs text-slate-500">{t("searchHelp")}</p>{(q || model || source !== "all") && <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500"><span>{t("filters", { filters: [q && `“${q}”`, model, source !== "all" && sourceName(source)].filter(Boolean).join(" · ") })}</span><Button size="sm" variant="ghost" onClick={() => { setDraft(""); setQuery(""); setModel(""); setSource("all"); }}>{t("clearFilters")}</Button></div>}</Card>
    {catalog.error && <LoadError error={catalog.error} retry={() => void catalog.refetch()} />}
    {result.isPending && <p role="status" className="motion-loading text-sm text-slate-500">{t("sessionsLoading")}</p>}
    {result.error && <LoadError error={result.error} retry={() => void result.refetch()} />}
    {first && <><div className="flex flex-wrap items-center justify-between gap-2"><h2 data-motion="sessions-match-count" data-motion-reveal="false" data-motion-update={first.total} className="font-semibold text-sky-950">{t("matchingSessions", { count: first.total, amount: count(first.total) })}</h2><span className="text-xs text-slate-500">{t("recentFirst")}</span></div><Card data-motion="sessions-results" className="gap-0 px-4 py-1 sm:px-5">{sessions.length ? sessions.map(session => <SessionRow key={`${session.source}:${session.session_id}`} session={session} />) : <div className="py-9 text-center"><p className="font-medium text-slate-700">{t("noSessions")}</p><p className="mt-2 text-sm text-slate-500">{t("noSessionsHelp")}</p></div>}</Card>{result.hasNextPage && <div className="flex justify-center"><Button variant="outline" className={result.isFetchingNextPage ? "motion-loading" : undefined} disabled={result.isFetchingNextPage} onClick={() => void result.fetchNextPage()}>{result.isFetchingNextPage ? t("loading") : t("moreSessions")}</Button></div>}<Coverage unassigned={first.unassigned_events} indexedAt={first.indexed_at} /></>}
  </>;
}
function SessionDetailView({ source, sessionID }: { source: string; sessionID: string }) {
  const { t } = useTranslation('sessions');
  const { fmt } = useCurrency(); const result = useSessionDetail(source, sessionID);
  const session = result.data?.pages[0]?.session; const events = result.data?.pages.flatMap(page => page.events) ?? [];
  return <div className="space-y-4"><Button asChild variant="ghost" className="-ml-3"><a href="#sessions"><ArrowLeft className="size-4" /> {t("allSessions")}</a></Button>{result.isPending && <p role="status" className="motion-loading">{t("detailsLoading")}</p>}{result.error && <LoadError error={result.error} retry={() => void result.refetch()} />}{session && <><Card data-motion="session-summary" className="gap-4 p-5"><div className="flex flex-wrap items-start justify-between gap-3"><div className="min-w-0 flex-1"><Badge variant="outline">{sourceName(session.source)}</Badge><h2 className="mt-3 break-words text-xl font-semibold text-sky-950">{sessionTitle(session)}</h2><p className="mt-2 break-all text-xs text-slate-500">{session.session_id}</p><p className="mt-1 break-all text-sm text-slate-600">{session.project || t("workspaceUnavailable")}</p></div><SessionValue session={session} /></div><div data-motion="session-summary-stats" data-motion-reveal="false" data-motion-update={JSON.stringify([tokens(session), session.calls, session.cache_ratio, session.known_cost_usd, session.unknown_cost_calls])} className="grid gap-4 border-t pt-4 sm:grid-cols-3"><div><p className="text-xs text-slate-500">{t("totalTokens")}</p><p className="mt-1 font-semibold tabular-nums">{count(tokens(session))}</p></div><div><p className="text-xs text-slate-500">{t("recordedEvents")}</p><p className="mt-1 font-semibold tabular-nums">{count(session.calls)}</p></div><div><p className="text-xs text-slate-500">{t("cacheShare")}</p><p className="mt-1 font-semibold">{session.cache_ratio == null ? t("unavailable") : formatNumber(session.cache_ratio, { style: "percent", maximumFractionDigits: 1 })}</p></div></div><p className="text-xs text-slate-500">{timestamp(session.started_at)} – {timestamp(session.last_at)} · {t("fullHistory")}</p><p className="break-words text-xs text-slate-500">{session.models?.join(" · ") || t("modelUnavailable")}</p></Card><Card data-motion="session-timeline" className="gap-3 p-4 sm:p-5"><h3 className="font-semibold text-sky-950">{t("timeline")}</h3><p className="text-xs text-slate-500">{t("accountingHelp")}</p>{events.length ? <Table><TableHeader><TableRow><TableHead>{t("timeModel")}</TableHead><TableHead className="text-right">{t("input")}</TableHead><TableHead className="text-right">{t("output")}</TableHead><TableHead className="text-right">{t("cacheRead")}</TableHead><TableHead className="text-right">{t("cacheWrite")}</TableHead><TableHead className="text-right">{t("usageValue")}</TableHead></TableRow></TableHeader><TableBody>{events.map(event => <TableRow key={event.event_id}><TableCell><p className="text-xs">{timestamp(event.ts)}</p><p className="max-w-64 break-words text-xs text-slate-500">{event.model || t("unknownModel")}</p></TableCell>{[event.input_tokens, event.output_tokens, event.cache_read_tokens, event.cache_write_tokens].map((value, index) => <TableCell key={index} className="text-right text-xs tabular-nums">{count(value)}</TableCell>)}<TableCell className="text-right text-xs tabular-nums"><p>{event.known_cost_usd == null || !event.cost_kind || event.cost_kind === "unknown" ? t("unavailable") : fmt(event.known_cost_usd)}</p><p className="text-slate-500">{event.fallback ? t("solEstimate") : event.cost_kind === "estimated" ? t("estimate") : event.cost_kind === "reported" ? t("reported") : ""}</p></TableCell></TableRow>)}</TableBody></Table> : <p className="py-4 text-sm text-slate-500">{t("noEvents")}</p>}{result.hasNextPage && <Button variant="outline" className={result.isFetchingNextPage ? "motion-loading" : undefined} disabled={result.isFetchingNextPage} onClick={() => void result.fetchNextPage()}>{result.isFetchingNextPage ? t("loading") : t("moreEvents")}</Button>}</Card></>}</div>;
}
