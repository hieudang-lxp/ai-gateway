import { sourceNames, useUsageSummary } from "./useUsageSummary";
import { collectorIds, sourceIsFresh, useSyncClock } from "./syncStatus";

const dateTime = (value: string | null | undefined) => value && !value.startsWith("0001") ? new Date(value).toLocaleString() : "Not synced yet";
const card = "rounded-2xl border border-sky-100 bg-white p-5 shadow-sm sm:p-6";

export function DataPricing({ period }: { period: string }) {
  const now = useSyncClock();
  const { data, error, isPending, isFetching, refetch } = useUsageSummary(period);
  const fallback = data?.rows.filter(row => row.fallback_cost_calls > 0) ?? [];
  const unpriced = data?.rows.reduce((sum, row) => sum + row.unknown_cost_calls, 0) ?? 0;
  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div><p className="mb-2 text-xs font-semibold uppercase tracking-widest text-sky-700">Data & Pricing</p><h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">Know what’s behind the numbers.</h1><p className="mt-2 max-w-2xl text-sm leading-relaxed text-slate-500">Your sources, collection coverage and the assumptions behind usage estimates.</p></div>
        <button type="button" disabled={isFetching} onClick={() => void refetch()} className="rounded-xl border border-sky-200 bg-white px-4 py-2.5 text-sm font-medium text-sky-800 hover:bg-sky-50 focus-visible:outline-2 focus-visible:outline-sky-600 disabled:opacity-50">{isFetching ? "Refreshing…" : "Refresh status"}</button>
      </div>
      {isPending && <p role="status" className={card}>Loading source details…</p>}
      {error && <p role="alert" className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">{error.message}{data && " Showing the last available data."}</p>}
      {data && <>
        <section aria-label="Collection sources" className="grid gap-4 md:grid-cols-3">
          {collectorIds.map(id => {
            const source = data.sources[id];
            const healthy = sourceIsFresh(source, now);
            const rows = data.rows.filter(row => row.source === id);
            const details = [
              ["Last successful sync", dateTime(source?.last_success)],
              ["Collection interval", source ? `Every ${source.poll_seconds} seconds` : "Unknown"],
              [id === "cursor" ? "Account history checked" : "Session files found", id === "cursor" ? `Up to ${data.cursor_history_days} days` : (source?.files ?? 0).toLocaleString()],
              ["Records in selected period", `${rows.reduce((sum, row) => sum + row.calls, 0).toLocaleString()} events`],
            ];
            return <article key={id} className={card}>
              <div className="flex items-center justify-between gap-2"><h2 className="font-semibold text-sky-950">{sourceNames[id]}</h2><span className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${healthy ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-800"}`}>{healthy ? "Synced" : source?.state === "ok" ? "Sync overdue" : source?.state ?? "Unavailable"}</span></div>
              <p className="mb-5 mt-2 text-xs text-slate-500">{id === "cursor" ? "Signed-in account · Cursor API" : "Local session logs · read-only"}</p>
              <dl className="space-y-3 text-xs">{details.map(([label, value]) => <div key={label}><dt className="text-slate-400">{label}</dt><dd className="mt-1 text-slate-700">{value}</dd></div>)}</dl>
              {source?.error && <p className="mt-4 break-words rounded-lg bg-amber-50 p-3 text-xs text-amber-900">{source.error}</p>}
            </article>;
          })}
        </section>
        <div className="grid gap-4 lg:grid-cols-2">
          <section className={card}>
            <div className="flex flex-wrap items-center justify-between gap-2"><h2 className="font-semibold text-sky-950">Codex price catalog</h2><span className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${data.pricing.stale || data.pricing.error ? "bg-amber-50 text-amber-800" : "bg-sky-50 text-sky-700"}`}>{data.pricing.stale || data.pricing.error ? "Needs attention" : "Recently fetched"}</span></div>
            <p className="mt-3 text-sm leading-relaxed text-slate-500">Current standard API rates apply to each request, including cache tokens and context tiers. Fast/priority premiums and subscription fees are excluded.</p>
            <dl className="mt-5 grid gap-4 text-xs sm:grid-cols-2"><div><dt className="text-slate-400">Last successful price fetch</dt><dd className="mt-1 text-slate-700">{dateTime(data.pricing.updated_at)}</dd></div><div><dt className="text-slate-400">Refresh schedule</dt><dd className="mt-1 text-slate-700">Every {data.pricing.refresh_seconds / 60} minutes</dd></div></dl>
            <a className="mt-5 inline-block text-sm font-medium text-sky-700 underline underline-offset-4" href="https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json" target="_blank" rel="noreferrer">Open LiteLLM price catalog ↗</a>
            <p className="mt-3 text-xs leading-relaxed text-slate-400">Last good rates remain available if refresh fails. Historical usage uses current prices; previous rate cards are not stored yet. Catalog changes can lag provider updates.</p>
            {data.pricing.error && <p className="mt-3 text-xs text-amber-800">{data.pricing.error}</p>}
          </section>
          <section className={card}>
            <h2 className="font-semibold text-sky-950">How usage is valued</h2>
            <dl className="mt-4 space-y-4 text-sm"><div><dt className="font-medium text-slate-700">Claude Code</dt><dd className="mt-1 text-slate-500">Estimated using the gateway’s configured API rates.</dd></div><div><dt className="font-medium text-slate-700">Codex</dt><dd className="mt-1 text-slate-500">Estimated using the current catalog. Unlisted models use {data.pricing.fallback_model} as a labelled fallback.</dd></div><div><dt className="font-medium text-slate-700">Cursor</dt><dd className="mt-1 text-slate-500">Reported charges when supplied by Cursor. Missing charges remain unpriced.</dd></div></dl>
          </section>
        </div>
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-2"><h2 className="font-semibold text-sky-950">Data coverage & assumptions</h2><a href="#overview" className="text-xs font-medium text-sky-700 underline underline-offset-4">Change period in Overview →</a></div>
          <p className="mt-2 text-xs text-slate-500">{period === "0" ? "All collected history" : `${dateTime(data.since)} — ${dateTime(data.generated_at)}`}</p>
          <div className="mt-5 grid gap-4 sm:grid-cols-2"><div className="rounded-xl bg-slate-50 p-4"><p className="text-2xl font-semibold text-sky-950">{unpriced.toLocaleString()}</p><p className="mt-1 text-xs text-slate-500">Events without a supplied cost</p></div><div className="rounded-xl bg-slate-50 p-4"><p className="text-2xl font-semibold text-sky-950">{fallback.reduce((sum, row) => sum + row.fallback_cost_calls, 0).toLocaleString()}</p><p className="mt-1 text-xs text-slate-500">Events using a substitute model price</p></div></div>
          {fallback.length > 0 && <ul className="mt-4 space-y-2 text-xs text-slate-600">{fallback.map(row => <li key={`${row.source}:${row.model}`}><span className="font-mono">{row.model}</span> · {row.fallback_cost_calls.toLocaleString()} events · priced using {data.pricing.fallback_model}</li>)}</ul>}
          <p className="mt-5 text-xs leading-relaxed text-slate-500">Claude and Codex coverage depends on retained local sessions. Remote-only activity without local logs is not captured. Once Claude transcripts are available, older proxy-only records are excluded from unified totals and remain in Overview’s proxy section. A successful sync does not guarantee a complete account history.</p>
        </section>
      </>}
    </div>
  );
}
