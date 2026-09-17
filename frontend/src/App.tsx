import { CurrencyProvider } from "./features/currency/CurrencyProvider";
import { Header } from "./components/Header";
import { BudgetBars } from "./features/proxy/BudgetBars";
import { CacheStats } from "./features/proxy/CacheStats";
import { SpendChart } from "./features/proxy/SpendChart";
import { ModelTable } from "./features/proxy/ModelTable";
import { RecentCalls } from "./features/proxy/RecentCalls";
import { UnifiedUsage } from "./features/usage/UnifiedUsage";

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
