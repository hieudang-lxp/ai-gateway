import { useCurrency } from "../features/currency/useCurrency";
import { useUsageSummary } from "../features/usage/useUsageSummary";
import { syncStatus, useSyncClock } from "../features/usage/syncStatus";
import { pages, type Page } from "../lib/navigation";
import { Layers } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { NavigationMenu, NavigationMenuItem, NavigationMenuLink, NavigationMenuList } from "@/components/ui/navigation-menu";

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
              <Layers className="size-6" strokeWidth={1.6} />
            </span>
            <span><span className="block text-lg font-semibold tracking-tight text-sky-950">AI Gateway</span><span className="block text-xs text-slate-500">Your AI workspace, in view.</span></span>
          </a>
          <div className="flex flex-wrap items-center gap-4 sm:gap-6">
            <a href="#data-pricing" className="hidden min-h-9 items-center gap-2 whitespace-nowrap rounded-md text-xs font-medium tabular-nums text-slate-500 outline-offset-4 hover:text-sky-800 focus-visible:outline-2 focus-visible:outline-sky-600 sm:inline-flex" aria-label={`Collector details: ${sync.label}`}>
              <span aria-hidden="true" className={`h-1.5 w-1.5 shrink-0 rounded-full ${sync.healthy ? "bg-emerald-500" : "bg-amber-500"}`} />
              <span>{sync.healthy && sync.lastSync ? `All synced by ${new Date(sync.lastSync).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}` : sync.label}</span>
            </a>
            <ToggleGroup type="single" value={currency} onValueChange={value => { if (value && value !== currency) toggle(); }} aria-label="Display currency" spacing={1} className="rounded-xl border bg-slate-50 p-1">
              {(["USD", "VND"] as const).map(c => <ToggleGroupItem key={c} value={c} aria-label={c} className="rounded-lg text-xs font-semibold text-slate-500 data-[state=on]:bg-sky-950 data-[state=on]:text-white data-[state=on]:shadow-sm">{c}</ToggleGroupItem>)}
            </ToggleGroup>
          </div>
        </div>
        <div className="flex items-center justify-between gap-3">
          <NavigationMenu aria-label="Main navigation" viewport={false}>
            <NavigationMenuList className="gap-5 sm:gap-7">
              {pages.map(item => <NavigationMenuItem key={item.id}><NavigationMenuLink asChild active={page === item.id} className="rounded-none border-b-2 border-transparent bg-transparent px-1 pb-3 pt-1 text-sm font-medium text-slate-500 data-active:border-sky-700 data-active:bg-transparent data-active:text-sky-900"><a href={`#${item.id}`} aria-current={page === item.id ? "page" : undefined}>{item.label}</a></NavigationMenuLink></NavigationMenuItem>)}
            </NavigationMenuList>
          </NavigationMenu>
          <span className="hidden pb-3 text-[13px] text-slate-400 lg:block" title={fetchedAt ? `Exchange rate updated ${fetchedAt.toLocaleString()}` : undefined}>{rate !== null ? `1 USD = ${new Intl.NumberFormat("vi-VN").format(rate)} ₫` : "Exchange rate unavailable"}</span>
          <a href="#data-pricing" aria-label={sync.label} className={`mb-3 flex items-center gap-1.5 text-[12px] sm:hidden ${sync.healthy ? "text-emerald-700" : "text-amber-700"}`}><span aria-hidden="true" className={`h-1.5 w-1.5 rounded-full ${sync.healthy ? "bg-emerald-500" : "bg-amber-500"}`} />{sync.healthy ? "Synced" : "Check sync"}</a>
        </div>
      </div>
    </header>
  );
}
