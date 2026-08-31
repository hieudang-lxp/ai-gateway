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
      <div className="mx-auto max-w-5xl p-6">
        <Header />
        <div className="grid gap-4">
          <div className="grid gap-4 md:grid-cols-2">
            <BudgetBars />
            <CacheStats />
          </div>
          <SpendChart />
          <ModelTable />
          <RecentCalls />
        </div>
      </div>
    </CurrencyProvider>
  );
}
