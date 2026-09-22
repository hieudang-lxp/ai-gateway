import { useTranslation } from 'react-i18next';
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useCurrency } from "../currency/useCurrency";
import { sessionLink } from "./query";
import type { Session } from "./types";
import { count, sourceName, timestamp, tokens, sessionTitle, shortSessionID } from "./format";
import { formatDate } from '@/i18n/format';
import { IndexedUsageError } from './errors';
export function PeriodSelect({ period, onChange }: { period: string; onChange: (value: string) => void }) {
  const { t } = useTranslation('sessions');
  return <div className="flex min-w-0 flex-wrap items-center gap-3"><Label htmlFor="indexed-period">{t("period")}</Label><Select value={period} onValueChange={onChange}><SelectTrigger id="indexed-period" className="w-48 max-w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="month">{t("month")}</SelectItem><SelectItem value="1">{t("today")}</SelectItem><SelectItem value="7">{t("last7")}</SelectItem><SelectItem value="30">{t("last30")}</SelectItem><SelectItem value="0">{t("history")}</SelectItem></SelectContent></Select></div>;
}
export function LoadError({ error, retry }: { error: Error; retry: () => void }) {
  const { t } = useTranslation('sessions');
  const known = error instanceof IndexedUsageError;
  return <Alert className="border-amber-200 bg-amber-50 text-amber-900"><AlertDescription className="flex flex-wrap items-center justify-between gap-3"><div className="min-w-0"><p>{known ? error.status === 404 ? t('notIndexed') : t('loadFailed', { status: error.status }) : t('unexpectedError')}</p>{!known && <details className="mt-2"><summary className="cursor-pointer">{t('errorDetails')}</summary><p className="break-all">{error.message}</p></details>}</div><Button size="sm" variant="outline" onClick={retry}>{t("retry")}</Button></AlertDescription></Alert>;
}
export function Coverage({ unassigned, indexedAt }: { unassigned: number; indexedAt: string }) {
  const { t } = useTranslation('sessions');
  return <p className="text-xs leading-relaxed text-slate-500">{t('coverage', { count: unassigned, amount: count(unassigned) })} {indexedAt && !indexedAt.startsWith("0001") ? t('indexUpdated', { date: formatDate(indexedAt) }) : t("indexWaiting")} <a className="text-sky-700 underline underline-offset-4" href="#data-pricing">{t("sourceCoverage")}</a></p>;
}
export function SessionValue({ session, alignEnd = false }: { session: Session; alignEnd?: boolean }) {
  const { t } = useTranslation('sessions');
  const { fmt } = useCurrency();
  return (
    <div className={`flex min-w-0 flex-col gap-2 ${alignEnd ? "md:items-end" : "items-start"}`}>
      <span className="break-words font-semibold tabular-nums text-sky-950">
        {session.unknown_cost_calls === session.calls && session.calls > 0 ? t("unavailable") : fmt(session.known_cost_usd)}
      </span>
      <div className={`flex min-h-6 flex-wrap items-center gap-2 ${alignEnd ? "md:justify-end" : ""}`}>
        {session.estimated_cost_calls > 0 && <Badge variant="secondary">{t("estimate")}</Badge>}
        {session.fallback_cost_calls > 0 && <Badge variant="outline">{t("solFallback")}</Badge>}
        {session.unknown_cost_calls > 0 && <span className="text-xs text-slate-500">{t('unpriced', { amount: count(session.unknown_cost_calls) })}</span>}
      </div>
    </div>
  );
}
export function SessionRow({ session }: { session: Session }) {
  const { t } = useTranslation('sessions');
  return (
    <article className="grid min-w-0 gap-4 border-b border-slate-100 py-5 last:border-0 md:grid-cols-[minmax(0,1fr)_18rem] md:gap-6">
      <div className="min-w-0">
        <div className="mb-1 flex flex-wrap items-center gap-2">
          <Badge variant="outline">{sourceName(session.source)}</Badge>
          <span className="text-xs text-slate-500">{timestamp(session.last_at)}</span>
        </div>
        <a href={sessionLink(session.source, session.session_id)} className="break-words font-semibold text-sky-900 underline-offset-4 hover:underline focus-visible:underline">
          {sessionTitle(session)}
        </a>
        {!session.title.trim() && <p className="mt-1 text-xs text-slate-500">{t('noTitle')} · <span className="font-mono" title={session.session_id}>{shortSessionID(session.session_id)}</span></p>}
        <p className="mt-1 break-all text-xs text-slate-500">{session.project || t("workspaceUnavailable")}</p>
        <p className="mt-1 break-words text-xs text-slate-500">{session.models?.join(" · ") || t("modelUnavailable")}</p>
      </div>
      <div className="min-w-0 space-y-2 md:text-right">
        <SessionValue session={session} alignEnd />
        <p className="text-xs tabular-nums text-slate-500">{t('tokensEvents', { tokens: count(tokens(session)), count: session.calls, amount: count(session.calls) })}</p>
      </div>
    </article>
  );
}
