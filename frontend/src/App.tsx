import { CurrencyProvider } from "./components/CurrencyContext";
import { Header } from "./components/Header";
import { BudgetBars } from "./components/BudgetBars";
import { CacheStats } from "./components/CacheStats";
import { SpendChart } from "./components/SpendChart";
import { ModelTable } from "./components/ModelTable";
import { RecentCalls } from "./components/RecentCalls";

export default function App() {
  return (
    <CurrencyProvider>
      <div className="min-h-screen bg-gradient-to-b from-sky-50 via-white to-white text-slate-800">
        <div className="mx-auto max-w-6xl px-6 py-8">
          <Header />
          <div className="grid gap-5">
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
