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
import { LocalMemory } from "./features/memory/LocalMemory";
import { usePage, useHash } from "./lib/navigation";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { usePageMotion } from "./lib/motion";

export default function App() {
  const { t, i18n } = useTranslation("common");
  const page = usePage();
  const mainRef = usePageMotion(page);
  const hash = useHash();
  const expanded = page === "insights" && new URLSearchParams(hash.split("?")[1]).get("view") === "network";
  useEffect(() => { window.scrollTo(0, 0); }, [expanded]);
  const [period, setPeriod] = useState("month");
  useEffect(() => {
    document.title = `${t(expanded ? "networkTitle" : page)} · AI Gateway`;
    document.documentElement.lang = i18n.resolvedLanguage ?? "en";
  }, [page, expanded, t, i18n.resolvedLanguage]);
  return (
    <CurrencyProvider>
      <div className="min-h-screen bg-gradient-to-b from-sky-50 via-white to-white text-base text-slate-800">
        {!expanded && <Header page={page} period={period} />}
        <main ref={mainRef} id="main-content" tabIndex={-1} className={expanded ? "min-h-screen bg-[#080c16] px-3 py-4 focus:outline-none sm:px-6" : "mx-auto max-w-6xl px-4 py-7 focus:outline-none sm:px-6 sm:py-9"}>
          {page === "local-memory" ? <LocalMemory /> : page === "sessions" ? <Sessions period={period} setPeriod={setPeriod} /> : page === "insights" ? <Insights period={period} setPeriod={setPeriod} expanded={expanded} /> : page === "data-pricing" ? <DataPricing period={period} /> : <>
          <div className="mb-6"><h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">{t("overview")}</h1><p className="mt-2 text-sm text-slate-500">{t("overviewDescription")}</p></div>
          <div className="grid grid-cols-1 gap-5">
            <UnifiedUsage period={period} setPeriod={setPeriod} />
            <h2 className="mt-3 text-sm font-semibold uppercase tracking-wide text-slate-500">{t("proxyTitle")} — {t("proxyDescription")}</h2>
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
