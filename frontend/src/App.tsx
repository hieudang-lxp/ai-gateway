import { CurrencyProvider } from "./components/CurrencyContext";
import { Header } from "./components/Header";
import { BudgetBars } from "./components/BudgetBars";
import { CacheStats } from "./components/CacheStats";
import { SpendChart } from "./components/SpendChart";
import { ModelTable } from "./components/ModelTable";
import { RecentCalls } from "./components/RecentCalls";
import { UnifiedUsage } from "./components/UnifiedUsage";

export default function App() {
  return (
    <CurrencyProvider>
      <div className="min-h-screen bg-gradient-to-b from-sky-50 via-white to-white text-slate-800">
        <div className="mx-auto max-w-6xl px-6 py-8">
          <Header />
          <div className="grid gap-5">
            <UnifiedUsage />
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
        </div>
      </div>
    </CurrencyProvider>
  );
}
