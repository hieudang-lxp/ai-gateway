import { useCurrency } from "./CurrencyContext";

export function Header() {
  const { currency, toggle, rate, fetchedAt } = useCurrency();
  return (
    <header className="mb-8 flex flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-sky-800 text-lg font-bold text-white shadow-sm">
          ⛵
        </div>
        <div>
          <h1 className="text-xl font-bold tracking-tight text-sky-950">
            ai-gateway
          </h1>
          <p className="text-xs text-slate-500">
            usage · cost · budget dashboard
          </p>
        </div>
      </div>
      <div className="flex items-center gap-3 text-sm">
        {rate !== null && (
          <span className="hidden text-xs text-slate-500 sm:inline">
            1 USD = {new Intl.NumberFormat("vi-VN").format(rate)} ₫
            {fetchedAt && ` · ${fetchedAt.toLocaleTimeString("vi-VN")}`}
          </span>
        )}
        <div className="flex overflow-hidden rounded-lg border border-sky-200 bg-white shadow-sm">
          {(["USD", "VND"] as const).map((c) => (
            <button
              key={c}
              onClick={() => currency !== c && toggle()}
              className={`px-3 py-1.5 font-mono text-xs font-semibold transition-colors ${
                currency === c
                  ? "bg-sky-800 text-white"
                  : "text-sky-800 hover:bg-sky-50"
              }`}
            >
              {c}
            </button>
          ))}
        </div>
      </div>
    </header>
  );
}
