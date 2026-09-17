import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiBaseURL } from "../lib/transport";
import { summarizeUsage, type UsageCounts } from "../lib/usage";
import { useCurrency } from "./CurrencyContext";

type Row = UsageCounts & { source: string; model: string; estimated_cost_calls: number; fallback_cost_calls: number; first_ts: number; last_ts: number };
type Status = { state: string; last_success: string | null; error?: string; poll_seconds: number };
type Summary = { pricing: { source: string; updated_at: string; stale: boolean; error?: string }; rows: Row[]; sources: Record<string, Status>; cursor_history_days: number; since: string; generated_at: string };
const names: Record<string, string> = { claude_code: "Claude Code", claude_gateway: "Claude gateway", codex: "Codex", cursor: "Cursor" };
const number = (n: number) => n.toLocaleString();
const short = (n: number) => new Intl.NumberFormat("en", { notation: "compact", maximumFractionDigits: 2 }).format(n);
const date = (ts: number) => new Date(ts * 1000).toLocaleDateString();

export function UnifiedUsage() {
  const [period, setPeriod] = useState("month");
  const { fmt } = useCurrency();
  const { data, error, isPending, isFetching } = useQuery<Summary>({
    queryKey: ["unified-usage", period],
    queryFn: async () => {
      const url = new URL("/_usage", apiBaseURL);
      url.searchParams.set(period === "month" ? "period" : "days", period);
      const response = await fetch(url, { cache: "no-store" });
      if (!response.ok) throw new Error(`Collector unavailable (HTTP ${response.status}). Open the local Docker dashboard.`);
      return response.json();
    },
    refetchInterval: 30_000,
    retry: 1,
  });
  const rows = data?.rows ?? [];
  const total = summarizeUsage(rows);
  const healthy = data && Object.values(data.sources).every(s => s.state === "ok");
  return (
    <section className="rounded-2xl border border-sky-200 bg-white p-6 shadow-sm">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-sky-950">All your AI usage</h2>
          <p className="mt-1 text-xs text-slate-500">Claude Code · Codex · Cursor — collected automatically across IDEs on this machine</p>
        </div>
        <label className="text-sm text-slate-600">Period <select aria-label="Usage period" value={period} onChange={e => setPeriod(e.target.value)} className="ml-2 rounded-lg border border-sky-200 bg-white p-2">
          <option value="month">This month</option>
          <option value={1}>Today</option><option value={7}>Last 7 days</option><option value={30}>Last 30 days</option><option value={0}>All collected history</option>
        </select></label>
      </div>
      {isPending && <p className="text-sm text-slate-500">Loading collected usage…</p>}
      {error && <p role="alert" className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">{error.message}</p>}
      {data && <>
        <p className="mb-4 text-xs text-slate-500">{period === "0" ? "All collected history" : `${new Date(data.since).toLocaleDateString("vi-VN", { timeZone: "Asia/Ho_Chi_Minh" })} – ${new Date(data.generated_at).toLocaleDateString("vi-VN", { timeZone: "Asia/Ho_Chi_Minh" })} · Asia/Ho_Chi_Minh`}</p>
        <div className="mb-5 grid gap-3 sm:grid-cols-3">
          {[{ label: "Collected tokens", value: short(total.tokens), detail: `${number(total.tokens)} exact · includes cache` },
            { label: "Recorded responses / events", value: number(total.calls), detail: "Providers group events differently" },
            { label: "Known usage value (USD / estimate)", value: fmt(total.cost), detail: total.unknown ? `${number(total.unknown)} events have no cost data` : "Usage value, not your subscription invoice" },
          ].map(card => <div key={card.label} className="rounded-xl bg-sky-50 p-4"><p className="text-xs text-slate-600">{card.label}</p><p className="my-1 text-2xl font-bold text-sky-950">{card.value}</p><p className="text-xs text-slate-500">{card.detail}</p></div>)}
        </div>
        <div className="mb-5 grid gap-3 md:grid-cols-3">
          {["claude_code", "codex", "cursor"].map(source => {
            const sourceRows = rows.filter(r => r.source === source || (source === "claude_code" && r.source === "claude_gateway"));
            const sum = summarizeUsage(sourceRows); const status = data.sources[source];
            return <div key={source} className="rounded-xl border border-slate-200 p-4">
              <div className="flex items-center justify-between"><h3 className="font-semibold text-slate-800">{names[source]}</h3><span className={`text-xs ${status?.state === "ok" ? "text-emerald-700" : "text-amber-700"}`}>{status?.state ?? "unavailable"}</span></div>
              <p className="mt-2 text-xl font-semibold text-sky-950">{short(sum.tokens)} <span className="text-xs font-normal text-slate-500">tokens</span></p>
              <p className="mt-1 text-xs text-slate-500">{sum.unknown === sum.calls && sum.calls > 0 ? "Cost unavailable" : `${fmt(sum.cost)} ${source === "codex" ? "estimated usage value" : "known usage value"}`}{sum.unknown > 0 && ` · ${number(sum.unknown)} unpriced`}</p>
              {sourceRows.length > 0 && <p className="mt-2 text-xs text-slate-500">Recorded {date(Math.min(...sourceRows.map(r => r.first_ts)))} – {date(Math.max(...sourceRows.map(r => r.last_ts)))}</p>}
              <p className="mt-2 text-xs text-slate-500">Polls every {status?.poll_seconds ?? "?"}s · {status?.last_success ? `last synced ${new Date(status.last_success).toLocaleTimeString()}` : "waiting for first sync"}</p>
              {status?.error && <p className="mt-2 break-words text-xs text-amber-800">{status.error}</p>}
            </div>;
          })}
        </div>
        {!healthy && <p role="status" className="mb-4 rounded-lg bg-amber-50 p-3 text-sm text-amber-900">Some collectors are starting or need attention. Totals show the records collected so far.</p>}
        <p className="mb-4 text-xs leading-relaxed text-slate-500">Codex value = input + cache read + cache write + output tokens × current standard API rates, with context tiers applied per request. This excludes Fast/priority premiums. Unlisted models (including codex-auto-review) use GPT-5.6 Sol as a fallback estimate; missing cache-write rates use regular input rates. Claude costs use configured API rate estimates; Cursor uses reported charges when present. Subscription fees are not included. Claude totals use retained transcripts only once available: older gateway-only history is excluded here and remains in the proxy section below. Cursor refreshes up to {data.cursor_history_days} days of history; retained local logs determine Claude/Codex coverage.</p>
        <p className={`mb-4 text-xs ${data.pricing.stale || data.pricing.error ? "text-amber-700" : "text-slate-500"}`}>
          Codex prices: <a href={data.pricing.source} target="_blank" rel="noreferrer" className="underline">LiteLLM catalog</a> · refreshes hourly · {data.pricing.updated_at.startsWith("0001") ? "bundled fallback rates (17/09/2026)" : `updated ${new Date(data.pricing.updated_at).toLocaleString()}`}
          {data.pricing.stale && " · stale prices; retrying automatically"}{data.pricing.error && ` · ${data.pricing.error}`}. Historical usage is valued at current prices.
        </p>
        <details>
          <summary className="cursor-pointer text-sm font-medium text-sky-800">Models and token breakdown {isFetching && "· updating"}</summary>
          <div className="mt-3 overflow-x-auto"><table className="w-full text-right text-xs tabular-nums"><thead className="border-b border-sky-100 text-slate-500"><tr><th className="py-2 text-left">Source / model</th><th>Events</th><th>Input</th><th>Output</th><th>Cache read</th><th>Cache write</th><th>Known value</th></tr></thead><tbody>
            {rows.map(r => <tr key={`${r.source}:${r.model}`} className="border-b border-sky-50"><td className="py-2 pr-3 text-left">{names[r.source]}<br/><span className="font-mono text-slate-500">{r.model || "unknown"}</span></td><td>{number(r.calls)}</td><td>{number(r.input_tokens)}</td><td>{number(r.output_tokens)}</td><td>{number(r.cache_read_tokens)}</td><td>{number(r.cache_write_tokens)}</td><td>{r.unknown_cost_calls === r.calls ? "Unavailable" : fmt(r.known_cost_usd)}{r.estimated_cost_calls > 0 && " est."}{r.fallback_cost_calls > 0 && " (Sol fallback)"}</td></tr>)}
          </tbody></table></div>
        </details>
      </>}
    </section>
  );
}
