import { useCurrency } from "./CurrencyContext";

export function Header() {
  const { currency, toggle, rate, fetchedAt } = useCurrency();
  return (
    <header className="mb-6 flex items-center justify-between">
      <h1 className="text-2xl font-bold">ai-gateway</h1>
      <div className="flex items-center gap-3 text-sm">
        {rate !== null && (
          <span className="text-gray-500">
            1 USD = {new Intl.NumberFormat("vi-VN").format(rate)} ₫
            {fetchedAt && ` · ${fetchedAt.toLocaleTimeString("vi-VN")}`}
          </span>
        )}
        <button
          onClick={toggle}
          className="rounded border border-gray-300 px-3 py-1 font-mono"
        >
          {currency}
        </button>
      </div>
    </header>
  );
}
