import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useCurrency } from "../currency/useCurrency";
import { sessionLink } from "./query";
import type { Session } from "./types";
import { count, sourceName, timestamp, tokens } from "./format";
export function PeriodSelect({ period, onChange }: { period: string; onChange: (value: string) => void }) {
  return <div className="flex items-center gap-3"><Label htmlFor="indexed-period">Period</Label><Select value={period} onValueChange={onChange}><SelectTrigger id="indexed-period" className="w-48"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="month">This month</SelectItem><SelectItem value="1">Today</SelectItem><SelectItem value="7">Last 7 days</SelectItem><SelectItem value="30">Last 30 days</SelectItem><SelectItem value="0">All history</SelectItem></SelectContent></Select></div>;
}
export function LoadError({ error, retry }: { error: Error; retry: () => void }) {
  return <Alert className="border-amber-200 bg-amber-50 text-amber-900"><AlertDescription className="flex flex-wrap items-center justify-between gap-3"><span>{error.message}</span><Button size="sm" variant="outline" onClick={retry}>Retry</Button></AlertDescription></Alert>;
}
export function Coverage({ unassigned, indexedAt }: { unassigned: number; indexedAt: string }) {
  return <p className="text-xs leading-relaxed text-slate-500">Session metadata only; prompts and replies are not shown. {count(unassigned)} events have no session attribution, including Cursor events without a session ID. They remain included in usage totals but are not grouped into sessions. {indexedAt && !indexedAt.startsWith("0001") ? `Index updated ${new Date(indexedAt).toLocaleString()}.` : "Waiting for the first index update."} <a className="text-sky-700 underline underline-offset-4" href="#data-pricing">Source coverage</a></p>;
}
export function SessionValue({ session }: { session: Session }) {
  const { fmt } = useCurrency();
  return <div className="flex flex-wrap items-center gap-2"><span className="font-semibold tabular-nums text-sky-950">{session.unknown_cost_calls === session.calls && session.calls > 0 ? "Unavailable" : fmt(session.known_cost_usd)}</span>{session.estimated_cost_calls > 0 && <Badge variant="secondary">Estimate</Badge>}{session.fallback_cost_calls > 0 && <Badge variant="outline">Sol fallback</Badge>}{session.unknown_cost_calls > 0 && <span className="text-xs text-slate-500">{count(session.unknown_cost_calls)} unpriced</span>}</div>;
}
export function SessionRow({ session }: { session: Session }) {
  return <article className="flex min-w-0 flex-col gap-3 border-b border-slate-100 py-4 last:border-0 sm:flex-row sm:items-start sm:justify-between"><div className="min-w-0 flex-1"><div className="mb-1 flex flex-wrap items-center gap-2"><Badge variant="outline">{sourceName(session.source)}</Badge><span className="text-xs text-slate-500">{timestamp(session.last_at)}</span></div><a href={sessionLink(session.source, session.session_id)} className="break-words font-semibold text-sky-900 underline-offset-4 hover:underline focus-visible:underline">{session.title || session.session_id}</a><p className="mt-1 break-all text-xs text-slate-500">{session.project || "Workspace unavailable"}</p><p className="mt-1 break-words text-xs text-slate-500">{session.models?.join(" · ") || "Model unavailable"}</p></div><div className="shrink-0 space-y-1 sm:max-w-64"><SessionValue session={session} /><p className="text-xs tabular-nums text-slate-500">{count(tokens(session))} tokens · {count(session.calls)} events</p></div></article>;
}
