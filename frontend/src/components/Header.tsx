import { useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { Layers, LayoutDashboard, Database, MessagesSquare, ChartNoAxesCombined, Brain } from "lucide-react";
import { useCurrency } from "../features/currency/useCurrency";
import { useUsageSummary } from "../features/usage/useUsageSummary";
import { syncState, useSyncClock } from "../features/usage/syncStatus";
import { pages, type Page } from "../lib/navigation";
import { formatDate, formatNumber } from "@/i18n/format";
import { LanguageSelect } from "./LanguageSelect";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

export function Header({ page, period }: { page: Page; period: string }) {
  const { t, i18n } = useTranslation("common");
  const { t: usageT } = useTranslation("usage");
  const { currency, toggle, rate, fetchedAt } = useCurrency();
  const { data, isError } = useUsageSummary(period);
  const sync = syncState(data?.sources, isError, useSyncClock());
  const navRef = useRef<HTMLElement>(null);
  const [indicator, setIndicator] = useState<CSSProperties>({ opacity: 0 });
  useLayoutEffect(() => {
    const nav = navRef.current;
    if (!nav) return;
    const measure = () => {
      const active = nav.querySelector<HTMLElement>('[aria-current="page"]');
      if (!active) return;
      setIndicator({ opacity: 1, width: active.offsetWidth, height: active.offsetHeight, transform: `translate(${active.offsetLeft}px, ${active.offsetTop}px)` });
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(nav);
    return () => observer.disconnect();
  }, [page, i18n.resolvedLanguage]);
  const status = sync.healthy && sync.lastSync
    ? t("usageSynced", { time: formatDate(sync.lastSync, { hour: "2-digit", minute: "2-digit" }) })
    : t("usageStatus", { status: usageT(sync.key, sync.values) });

  return <header className="border-b border-sky-100 bg-white/95 shadow-sm">
    <a href="#main-content" onClick={event => { event.preventDefault(); document.getElementById("main-content")?.focus(); }} className="sr-only z-50 rounded-lg bg-sky-950 p-3 text-white focus:not-sr-only focus:fixed focus:left-4 focus:top-4">{t("skipContent")}</a>
    <div className="mx-auto max-w-6xl px-4 sm:px-6">
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-4 py-5">
        <a href="#overview" aria-label={t("home")} className="flex items-center gap-3 rounded-lg outline-offset-4 focus-visible:outline-2 focus-visible:outline-sky-600">
          <span aria-hidden="true" className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-sky-950 text-white shadow-sm"><Layers className="size-6" strokeWidth={1.6} /></span>
          <span><span className="block text-lg font-semibold tracking-tight text-sky-950">AI Gateway</span><span className="block text-xs text-slate-500">{t("tagline")}</span></span>
        </a>
        <div className="flex flex-wrap items-center gap-4 sm:gap-6">
          <a href="#data-pricing" className="hidden min-h-9 items-center gap-2 whitespace-nowrap rounded-md text-xs font-medium tabular-nums text-slate-500 outline-offset-4 hover:text-sky-800 focus-visible:outline-2 focus-visible:outline-sky-600 lg:inline-flex">
            <span aria-hidden="true" className={`h-1.5 w-1.5 shrink-0 rounded-full ${sync.healthy ? "bg-emerald-500" : "bg-amber-500"}`} />{status}
          </a>
          {page !== "local-memory" && <ToggleGroup type="single" value={currency} onValueChange={value => { if (value && value !== currency) toggle(); }} aria-label={t("currency")} spacing={1} className="rounded-xl border bg-slate-50 p-1">
            {(["USD", "VND"] as const).map(code => <ToggleGroupItem key={code} value={code} aria-label={code} className="rounded-lg text-xs font-semibold text-slate-500 data-[state=on]:bg-sky-950 data-[state=on]:text-white data-[state=on]:shadow-sm">{code}</ToggleGroupItem>)}
          </ToggleGroup>}
          <LanguageSelect />
        </div>
      </div>
      <div className="flex items-end justify-between gap-3">
        <div className="min-w-0 flex-1 overflow-x-auto overflow-y-hidden"><nav ref={navRef} aria-label={t("navigation")} className="relative flex w-max min-w-full flex-nowrap gap-0">
          <span aria-hidden="true" className="nav-indicator pointer-events-none absolute left-0 top-0 border-b-2 border-sky-600" style={indicator} />
          {pages.map(item => {
            const Icon = { overview: LayoutDashboard, sessions: MessagesSquare, insights: ChartNoAxesCombined, "local-memory": Brain, "data-pricing": Database }[item.id];
            return <a key={item.id} href={`#${item.id}`} aria-current={page === item.id ? "page" : undefined} className={`relative z-10 flex h-12 items-center gap-2 whitespace-nowrap border-b-2 border-transparent px-3 text-sm font-medium outline-offset-[-2px] transition-colors focus-visible:outline-2 focus-visible:outline-sky-600 sm:px-4 ${page === item.id ? "text-sky-950" : "text-slate-500 hover:text-sky-900"}`}><Icon aria-hidden="true" className="size-4 shrink-0" strokeWidth={1.75} />{t(item.id)}</a>;
          })}
        </nav></div>
        {page !== "local-memory" && <span className="hidden shrink-0 pb-3 text-[13px] text-slate-400 xl:block" title={fetchedAt ? t("rateUpdated", { time: formatDate(fetchedAt) }) : undefined}>{rate !== null ? t("exchangeRate", { value: formatNumber(rate) }) : t("rateUnavailable")}</span>}
      </div>
    </div>
  </header>;
}
