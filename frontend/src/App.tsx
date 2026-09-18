import { CurrencyProvider } from "./features/currency/CurrencyProvider";
import { Header } from "./components/Header";
import { BudgetBars } from "./features/proxy/BudgetBars";
import { CacheStats } from "./features/proxy/CacheStats";
import { SpendChart } from "./features/proxy/SpendChart";
import { ModelTable } from "./features/proxy/ModelTable";
import { RecentCalls } from "./features/proxy/RecentCalls";
import { UnifiedUsage } from "./features/usage/UnifiedUsage";
import { DataPricing } from "./features/usage/DataPricing";
import { usePage } from "./lib/navigation";
import { useEffect, useState } from "react";

export default function App() {
  const page = usePage();
  const [period, setPeriod] = useState("month");
  useEffect(() => {
    document.title = `${page === "overview" ? "Overview" : "Data & Pricing"} · AI Gateway`;
  }, [page]);
  return (
    <CurrencyProvider>
      <div className="min-h-screen bg-gradient-to-b from-sky-50 via-white to-white text-slate-800">
        <Header page={page} period={period} />
        <main id="main-content" tabIndex={-1} className="mx-auto max-w-6xl px-4 py-7 focus:outline-none sm:px-6 sm:py-9">
          {page === "data-pricing" ? <DataPricing period={period} /> : <>
          <div className="mb-6"><p className="mb-2 text-xs font-semibold uppercase tracking-widest text-sky-700">Overview</p><h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">Your AI usage, at a glance.</h1><p className="mt-2 text-sm text-slate-500">One place for Claude Code, Codex and Cursor.</p></div>
          <div className="grid gap-5">
            <UnifiedUsage period={period} setPeriod={setPeriod} />
            <h2 className="mt-3 text-sm font-semibold uppercase tracking-wide text-slate-500">Gateway proxy only — budgets, cache and request diagnostics</h2>
            <div className="grid gap-5 lg:grid-cols-5">
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
