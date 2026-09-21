import { CurrencyProvider } from "./features/currency/CurrencyProvider";
import { Header } from "./components/Header";
import { BudgetBars } from "./features/proxy/BudgetBars";
import { CacheStats } from "./features/proxy/CacheStats";
import { SpendChart } from "./features/proxy/SpendChart";
import { ModelTable } from "./features/proxy/ModelTable";
import { RecentCalls } from "./features/proxy/RecentCalls";
import { UnifiedUsage } from "./features/usage/UnifiedUsage";
import { DataPricing } from "./features/usage/DataPricing";
import { Sessions } from "./features/sessions/Sessions";
import { Insights } from "./features/insights/Insights";
import { pages, usePage, useHash } from "./lib/navigation";
import { useEffect, useState } from "react";

export default function App() {
  const page = usePage();
  const hash = useHash();
  const expanded = page === "insights" && new URLSearchParams(hash.split("?")[1]).get("view") === "network";
  useEffect(() => { window.scrollTo(0, 0); }, [expanded]);
  const [period, setPeriod] = useState("month");
  useEffect(() => {
    document.title = `${expanded ? "Network explorer" : pages.find(item => item.id === page)?.label} · AI Gateway`;
  }, [page, expanded]);
  return (
    <CurrencyProvider>
      <div className="min-h-screen bg-gradient-to-b from-sky-50 via-white to-white text-base text-slate-800">
        {!expanded && <Header page={page} period={period} />}
        <main id="main-content" tabIndex={-1} className={expanded ? "min-h-screen bg-[#080c16] px-3 py-4 focus:outline-none sm:px-6" : "mx-auto max-w-6xl px-4 py-7 focus:outline-none sm:px-6 sm:py-9"}>
          {page === "sessions" ? <Sessions period={period} setPeriod={setPeriod} /> : page === "insights" ? <Insights period={period} setPeriod={setPeriod} expanded={expanded} /> : page === "data-pricing" ? <DataPricing period={period} /> : <>
          <div className="mb-6"><p className="mb-2 text-xs font-semibold uppercase tracking-widest text-sky-700">Overview</p><h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">Your AI usage, at a glance.</h1><p className="mt-2 text-sm text-slate-500">One place for Claude Code, Codex and Cursor.</p></div>
          <div className="grid grid-cols-1 gap-5">
            <UnifiedUsage period={period} setPeriod={setPeriod} />
            <h2 className="mt-3 text-sm font-semibold uppercase tracking-wide text-slate-500">Gateway proxy only — budgets, cache and request diagnostics</h2>
            <div className="grid grid-cols-1 gap-5 lg:grid-cols-5">
              <div className="lg:col-span-3">
                <BudgetBars />
              </div>
              <div className="lg:col-span-2">
                <CacheStats />
              </div>
            </div>
            <SpendChart />
            <ModelTable />
            <RecentCalls />
          </div>
          </>}
        </main>
      </div>
    </CurrencyProvider>
  );
}
