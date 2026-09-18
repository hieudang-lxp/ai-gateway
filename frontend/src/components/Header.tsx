import { useCurrency } from "../features/currency/useCurrency";
import { useUsageSummary } from "../features/usage/useUsageSummary";
import { syncStatus, useSyncClock } from "../features/usage/syncStatus";
import { pages, type Page } from "../lib/navigation";

export function Header({ page, period }: { page: Page; period: string }) {
  const { currency, toggle, rate, fetchedAt } = useCurrency();
  const { data, isError } = useUsageSummary(period);
  const now = useSyncClock();
  const sync = syncStatus(data?.sources, isError, now);
  return (
    <header className="border-b border-sky-100 bg-white/95 shadow-sm">
      <a href="#main-content" onClick={event => { event.preventDefault(); document.getElementById("main-content")?.focus(); }} className="sr-only z-50 rounded-lg bg-sky-950 p-3 text-white focus:not-sr-only focus:fixed focus:left-4 focus:top-4">Skip to content</a>
      <div className="mx-auto max-w-6xl px-4 sm:px-6">
        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-4 py-5">
          <a href="#overview" aria-label="AI Gateway overview" className="flex items-center gap-3 rounded-lg outline-offset-4 focus-visible:outline-2 focus-visible:outline-sky-600">
            <span aria-hidden="true" className="flex h-11 w-11 items-center justify-center rounded-2xl bg-sky-950 text-white shadow-sm">
              <svg width="25" height="25" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6"><path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="m3 12 9 5 9-5M3 16l9 5 9-5"/></svg>
            </span>
            <span><span className="block text-lg font-semibold tracking-tight text-sky-950">AI Gateway</span><span className="block text-xs text-slate-500">Your AI workspace, in view.</span></span>
          </a>
          <div className="flex flex-wrap items-center gap-4 sm:gap-6">
            <a href="#data-pricing" className="hidden rounded-md text-right outline-offset-4 focus-visible:outline-2 focus-visible:outline-sky-600 sm:block" aria-label={`Collector details: ${sync.label}`}>
              <span className="flex items-center justify-end gap-2 text-xs font-medium text-slate-700"><span aria-hidden="true" className={`h-1.5 w-1.5 rounded-full ${sync.healthy ? "bg-emerald-500" : "bg-amber-500"}`} />{sync.label}</span>
              <span className="mt-1 block text-[13px] text-slate-400">{sync.lastSync ? `All synced by ${new Date(sync.lastSync).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}` : "View connection details"}</span>
            </a>
            <div role="group" aria-label="Display currency" className="flex gap-1 rounded-xl border border-slate-200 bg-slate-50 p-1">
              {(["USD", "VND"] as const).map(c => <button key={c} type="button" aria-pressed={currency === c} onClick={() => currency !== c && toggle()} className={`min-h-9 rounded-lg px-3 text-xs font-semibold transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sky-600 ${currency === c ? "bg-sky-950 text-white shadow-sm" : "text-slate-500 hover:bg-white hover:text-sky-900"}`}>{c}</button>)}
            </div>
          </div>
        </div>
        <div className="flex items-center justify-between gap-3">
          <nav aria-label="Main navigation" className="flex gap-5 sm:gap-7">
            {pages.map(item => <a key={item.id} href={`#${item.id}`} aria-current={page === item.id ? "page" : undefined} className={`border-b-2 px-1 pb-3 pt-1 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sky-600 ${page === item.id ? "border-sky-700 text-sky-900" : "border-transparent text-slate-500 hover:border-sky-200 hover:text-sky-800"}`}>{item.label}</a>)}
          </nav>
          <span className="hidden pb-3 text-[13px] text-slate-400 lg:block" title={fetchedAt ? `Exchange rate updated ${fetchedAt.toLocaleString()}` : undefined}>{rate !== null ? `1 USD = ${new Intl.NumberFormat("vi-VN").format(rate)} ₫` : "Exchange rate unavailable"}</span>
          <a href="#data-pricing" aria-label={sync.label} className={`mb-3 flex items-center gap-1.5 text-[12px] sm:hidden ${sync.healthy ? "text-emerald-700" : "text-amber-700"}`}><span aria-hidden="true" className={`h-1.5 w-1.5 rounded-full ${sync.healthy ? "bg-emerald-500" : "bg-amber-500"}`} />{sync.healthy ? "Synced" : "Check sync"}</a>
        </div>
      </div>
    </header>
  );
}
