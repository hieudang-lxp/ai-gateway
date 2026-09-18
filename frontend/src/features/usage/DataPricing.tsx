import { useCurrency } from "../currency/useCurrency";
import { sourceNames, useUsageSummary } from "./useUsageSummary";
import { collectorIds, sourceIsFresh, useSyncClock } from "./syncStatus";
import { summarizeUsage } from "./usage";

const dateTime = (value: string | null | undefined) => value && !value.startsWith("0001") ? new Date(value).toLocaleString() : "Not synced yet";
const card = "rounded-2xl border border-sky-100 bg-white p-5 shadow-sm sm:p-6";
const methods = {
  claude_code: { label: "Estimated", explanation: "Tokens × configured API prices. This is an estimate of usage value." },
  codex: { label: "Estimated", explanation: "Tokens × current model prices. Some models use a substitute price." },
  cursor: { label: "Reported by Cursor", explanation: "Charges supplied by Cursor. Events without a charge are excluded from the amount." },
};

export function DataPricing({ period }: { period: string }) {
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
          <p className="mb-2 text-xs font-semibold uppercase tracking-widest text-sky-700">Data & Pricing</p>
          <h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">Check your data and costs.</h1>
          <p className="mt-3 max-w-2xl text-sm leading-relaxed text-slate-600">See what’s synced, how each amount is calculated, and where estimates are used.</p>
        </div>
        <button type="button" disabled={isFetching} onClick={() => void refetch()} className="rounded-xl border border-sky-200 bg-white px-4 py-2.5 text-sm font-medium text-sky-800 hover:bg-sky-50 focus-visible:outline-2 focus-visible:outline-sky-600 disabled:opacity-50">{isFetching ? "Refreshing…" : "Refresh status"}</button>
      </div>

      {isPending && <p role="status" className={card}>Checking your sources…</p>}
      {error && <p role="alert" className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">{error.message}{data && " Amounts below are from the last available snapshot."}</p>}
      {data && <>
        <section aria-label="Sync summary" className={`flex flex-wrap items-center justify-between gap-4 rounded-2xl border px-5 py-4 sm:px-6 ${ready === 3 ? "border-emerald-200 bg-emerald-50" : "border-amber-200 bg-amber-50"}`}>
          <div>
            <h2 className={`font-semibold ${ready === 3 ? "text-emerald-900" : "text-amber-900"}`}>{error ? "Cannot confirm sync status" : ready === 3 ? "All 3 tools are synced" : `${ready} of 3 tools are synced`}</h2>
            <p className={`mt-1 text-xs ${ready === 3 ? "text-emerald-800" : "text-amber-800"}`}>{ready === 3 ? "Automatic collection is running. The latest available records have been imported." : "Check the source marked below. Saved usage is still available."}</p>
          </div>
          <a href="#overview" className="shrink-0 text-sm font-medium text-sky-800 underline underline-offset-4">View usage →</a>
        </section>

        <section aria-labelledby="amounts-heading">
          <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
            <div><h2 id="amounts-heading" className="text-lg font-semibold text-sky-950">Where the amounts come from</h2><p className="mt-1 text-xs text-slate-600">{period === "0" ? "All collected history" : `${new Date(data.since).toLocaleDateString("vi-VN", { timeZone: "Asia/Ho_Chi_Minh" })} – ${new Date(data.generated_at).toLocaleDateString("vi-VN", { timeZone: "Asia/Ho_Chi_Minh" })}`} · Usage value, excluding subscription fees.</p></div>
            <a href="#overview" className="text-xs font-medium text-sky-700 underline underline-offset-4">Change period</a>
          </div>
          <div className="grid gap-4 lg:grid-cols-3">
            {collectorIds.map(id => {
              const source = data.sources[id];
              const healthy = !error && sourceIsFresh(source, now);
              const sourceRows = rows.filter(row => row.source === id || (id === "claude_code" && row.source === "claude_gateway"));
              const sum = summarizeUsage(sourceRows);
              return <article key={id} className={card}>
                <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-semibold text-sky-950">{sourceNames[id]}</h3><span className={`rounded-full px-2.5 py-1 text-xs font-medium ${healthy ? "bg-emerald-50 text-emerald-800" : "bg-amber-50 text-amber-800"}`}>{healthy ? "Synced" : error ? "Unverified" : "Check connection"}</span></div>
                <p className="mt-5 break-words text-2xl font-semibold tracking-tight text-sky-950">{fmt(sum.cost)}</p>
                <p className={`mt-2 text-xs font-semibold ${id === "cursor" ? "text-emerald-800" : "text-sky-700"}`}>{methods[id].label}</p>
                <p className="mt-3 text-sm leading-relaxed text-slate-600">{methods[id].explanation}</p>
                {sum.unknown > 0 && <p className="mt-3 text-xs text-amber-800">{sum.unknown.toLocaleString()} events have no price; the amount is incomplete.</p>}
                {sum.calls === 0 && <p className="mt-3 text-xs text-slate-600">No records in this period.</p>}
                <p className="mt-5 border-t border-slate-100 pt-3 text-xs text-slate-500">Last synced {dateTime(source?.last_success)}</p>
                {!healthy && !error && <p className="mt-3 text-xs text-amber-900">{id === "cursor" ? "Check your Cursor sign-in and the collector status below." : "Check the local session folder and Docker access in the details below."}</p>}
              </article>;
            })}
          </div>
        </section>

        <section aria-labelledby="accuracy-heading" className={card}>
          <h2 id="accuracy-heading" className="text-lg font-semibold text-sky-950">What affects accuracy?</h2>
          <div className="mt-4 divide-y divide-slate-100">
            <div className="py-4 first:pt-0"><h3 className="font-medium text-slate-800">{unpriced === 0 ? "Every recorded event has a price" : `${unpriced.toLocaleString()} events have no price`}</h3><p className="mt-1 text-sm text-slate-600">{unpriced === 0 ? "Some prices are estimates, as shown above." : "These events count toward tokens, but are excluded from the cost total."}</p></div>
            <div className="py-4"><h3 className={`font-medium ${fallbackCount > 0 ? "text-amber-900" : "text-slate-800"}`}>{fallbackCount > 0 ? `${fallbackCount.toLocaleString()} Codex events use a substitute price` : "No substitute Codex model prices used"}</h3><p className="mt-1 text-sm text-slate-600">{fallbackCount > 0 ? `Their model has no matching price in the catalog, so we use ${data.pricing.fallback_model}. Those amounts are rough estimates.` : "Codex models in this period did not need the substitute rate."}</p>{fallbackRows.length > 0 && <p className="mt-2 break-words text-xs text-amber-900">Affected models: {fallbackRows.map(row => `${row.model} (${row.fallback_cost_calls.toLocaleString()} events)`).join(", ")}</p>}</div>
            <div className="py-4 last:pb-0"><h3 className={`font-medium ${priceIssue ? "text-amber-900" : "text-slate-800"}`}>{priceIssue ? "Codex prices need a refresh" : "Codex price updates are working"}</h3><p className="mt-1 text-sm text-slate-600">{priceIssue ? "The gateway is using saved prices and will retry automatically. Check its network connection if this continues." : `Last fetched ${dateTime(data.pricing.updated_at)}. Prices refresh every ${data.pricing.refresh_seconds / 60} minutes.`}</p><p className="mt-2 text-xs text-slate-500">Older usage is recalculated with today’s prices. Your subscription bill can differ.</p></div>
          </div>
        </section>

        <details className={card}>
          <summary className="cursor-pointer text-sm font-semibold text-sky-800">Technical details & collection limits</summary>
          <div className="mt-5 grid gap-5 text-xs text-slate-600">
            <div className="grid gap-5 md:grid-cols-3">{collectorIds.map(id => { const source = data.sources[id]; return <div key={id}><h3 className="font-semibold text-slate-800">{sourceNames[id]}</h3><p className="mt-2">{id === "cursor" ? `Account API · up to ${data.cursor_history_days} days checked` : `Local session logs · ${source?.files ?? 0} files found`}</p><p className="mt-1">Poll interval: {source?.poll_seconds ?? "unknown"} seconds</p><p className="mt-1">Collector state: {source?.state ?? "unavailable"}</p>{source?.error && <p className="mt-2 break-words text-amber-900">{source.error}</p>}</div>; })}</div>
            <p>Codex estimates apply standard API rates per request, including cache tokens and context tiers. Fast/priority premiums are excluded. Missing cache-write prices use input prices. A history of previous price catalogs is not stored yet.</p>
            <p>Claude and Codex depend on retained local sessions. Remote activity without local logs is not captured. Once Claude transcripts are available, older proxy-only records remain in Overview’s proxy section and are excluded from unified totals. Successful sync does not guarantee complete account history.</p>
            <p>Cursor charges and LiteLLM price changes can arrive later than the original activity or provider update. Refresh status reads the saved snapshot; collectors follow their own polling schedules.</p>
            {data.pricing.error && <p className="text-amber-900">Price refresh error: {data.pricing.error}</p>}
            <a className="font-medium text-sky-700 underline underline-offset-4" href="https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json" target="_blank" rel="noreferrer">Open LiteLLM price catalog ↗</a>
          </div>
        </details>
      </>}
    </div>
  );
}
