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

const number = (n: number) => n.toLocaleString();
const short = (n: number) => new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 2 }).format(n);
const date = (ts: number) => new Date(ts * 1000).toLocaleDateString();

export function UnifiedUsage({ period, setPeriod }: { period: string; setPeriod: (period: string) => void }) {
  const { fmt } = useCurrency();
  const { data, error, isPending, isFetching } = useUsageSummary(period);
  const rows = data?.rows ?? [];
  const total = summarizeUsage(rows);
  const healthy = data && Object.values(data.sources).every(s => s.state === "ok");
  return (
    <Card className="gap-0 border-sky-200 p-6">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-sky-950">All your AI usage</h2>
          <p className="mt-1 text-xs text-slate-500">Claude Code · Codex · Cursor — collected automatically across IDEs on this machine</p>
        </div>
        <div className="flex max-w-full items-center gap-3">
          <Label htmlFor="usage-period" className="text-sm text-slate-600">Period</Label>
          <Select value={period} onValueChange={setPeriod}>
            <SelectTrigger id="usage-period" aria-label="Usage period" className="h-11 w-56 max-w-full px-4 text-sm"><SelectValue /></SelectTrigger>
            <SelectContent position="popper" sideOffset={6}>
              <SelectItem value="month">This month</SelectItem>
              <SelectItem value="1">Today</SelectItem>
              <SelectItem value="7">Last 7 days</SelectItem>
              <SelectItem value="30">Last 30 days</SelectItem>
              <SelectItem value="0">All collected history</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      {isPending && <p className="text-sm text-slate-500">Loading collected usage…</p>}
      {error && <Alert className="bg-amber-50 text-amber-900"><AlertDescription>{error.message}</AlertDescription></Alert>}
      {data && <>
        <p className="mb-4 text-xs text-slate-500">{period === "0" ? "All collected history" : `${new Date(data.since).toLocaleDateString("vi-VN", { timeZone: "Asia/Ho_Chi_Minh" })} – ${new Date(data.generated_at).toLocaleDateString("vi-VN", { timeZone: "Asia/Ho_Chi_Minh" })} · Asia/Ho_Chi_Minh`}</p>
        <div className="mb-5 grid gap-3 sm:grid-cols-3">
          {[{ label: "Collected tokens", value: short(total.tokens), detail: `${number(total.tokens)} exact · includes cache` },
            { label: "Recorded responses / events", value: number(total.calls), detail: "Providers group events differently" },
            { label: "Known usage value (USD / estimate)", value: fmt(total.cost), detail: total.unknown ? `${number(total.unknown)} events have no cost data` : "Usage value, not your subscription invoice" },
          ].map(card => <Card key={card.label} className="gap-0 border-0 bg-sky-50 p-4 shadow-none"><p className="text-xs text-slate-600">{card.label}</p><p className="my-1 text-2xl font-bold text-sky-950">{card.value}</p><p className="text-xs text-slate-500">{card.detail}</p></Card>)}
        </div>
        <div className="mb-5 grid gap-3 md:grid-cols-3">
          {["claude_code", "codex", "cursor"].map(source => {
            const sourceRows = rows.filter(r => r.source === source || (source === "claude_code" && r.source === "claude_gateway"));
            const sum = summarizeUsage(sourceRows); const status = data.sources[source];
            return <Card key={source} className="gap-0 p-4 shadow-none">
              <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-semibold text-slate-800">{names[source]}</h3><Badge variant="secondary" className={status?.state === "ok" ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-700"}>{status?.state ?? "unavailable"}</Badge></div>
              <p className="mt-2 text-xl font-semibold text-sky-950">{short(sum.tokens)} <span className="text-xs font-normal text-slate-500">tokens</span></p>
              <p className="mt-1 text-xs text-slate-500">{sum.unknown === sum.calls && sum.calls > 0 ? "Cost unavailable" : `${fmt(sum.cost)} ${source === "codex" ? "estimated usage value" : "known usage value"}`}{sum.unknown > 0 && ` · ${number(sum.unknown)} unpriced`}</p>
              {sourceRows.length > 0 && <p className="mt-2 text-xs text-slate-500">Recorded {date(Math.min(...sourceRows.map(r => r.first_ts)))} – {date(Math.max(...sourceRows.map(r => r.last_ts)))}</p>}
              <p className="mt-2 text-xs text-slate-500">Polls every {status?.poll_seconds ?? "?"}s · {status?.last_success ? `last synced ${new Date(status.last_success).toLocaleTimeString()}` : "waiting for first sync"}</p>
              {status?.error && <p className="mt-2 break-words text-xs text-amber-800">{status.error}</p>}
            </Card>;
          })}
        </div>
        {!healthy && <Alert role="status" className="mb-4 bg-amber-50 text-amber-900"><AlertDescription>Some collectors are starting or need attention. Totals show the records collected so far.</AlertDescription></Alert>}
        <p className="mb-4 text-xs leading-relaxed text-slate-500">Usage value includes API estimates and reported charges; subscription fees are excluded. <a href="#data-pricing" className="font-medium text-sky-700 underline underline-offset-4">Check sources, coverage & pricing assumptions →</a></p>
        <Accordion type="single" collapsible><AccordionItem value="models" className="border-0">
          <AccordionTrigger className="text-sm font-medium text-sky-800">Models and token breakdown {isFetching && "· updating"}</AccordionTrigger>
          <AccordionContent>
          <div className="mt-3 overflow-x-auto"><Table className="w-full text-right text-xs tabular-nums"><TableHeader className="border-b border-sky-100 text-slate-500"><TableRow><TableHead className="py-2 text-left">Source / model</TableHead><TableHead className="text-right">Events</TableHead><TableHead className="text-right">Input</TableHead><TableHead className="text-right">Output</TableHead><TableHead className="text-right">Cache read</TableHead><TableHead className="text-right">Cache write</TableHead><TableHead className="text-right">Known value</TableHead></TableRow></TableHeader><TableBody>
            {rows.map(r => <TableRow key={`${r.source}:${r.model}`} className="border-b border-sky-50"><TableCell className="py-2 pr-3 text-left">{names[r.source]}<br/><span className="font-mono text-slate-500">{r.model || "unknown"}</span></TableCell><TableCell>{number(r.calls)}</TableCell><TableCell>{number(r.input_tokens)}</TableCell><TableCell>{number(r.output_tokens)}</TableCell><TableCell>{number(r.cache_read_tokens)}</TableCell><TableCell>{number(r.cache_write_tokens)}</TableCell><TableCell>{r.unknown_cost_calls === r.calls ? "Unavailable" : fmt(r.known_cost_usd)}{r.estimated_cost_calls > 0 && " est."}{r.fallback_cost_calls > 0 && " (Sol fallback)"}</TableCell></TableRow>)}
          </TableBody></Table></div>
          </AccordionContent>
        </AccordionItem></Accordion>
      </>}
    </Card>
  );
}
