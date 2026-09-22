import { afterEach, describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import i18n from "@/i18n";
import { CurrencyCtx } from "../currency/useCurrency";
import { UnifiedUsage } from "./UnifiedUsage";
import { DataPricing } from "./DataPricing";
import { UsageFetchError, usageErrorMessage, type UsageSummary } from "./useUsageSummary";

// Transport derives its URL from window; these SSR tests exercise a populated
// query cache without invoking the network transport.
vi.mock("../../lib/transport", () => ({ apiBaseURL: "http://localhost:8788/rpc" }));

afterEach(async () => { await i18n.changeLanguage("en"); });

describe("localized cached usage", () => {
  const snapshot: UsageSummary = {
    pricing: { source: "LiteLLM", updated_at: "2026-09-22T00:00:00Z", stale: false, basis: "api", fallback_model: "sol-id", refresh_seconds: 60 },
    sources: { codex: { state: "error", error: "permission denied /data/codex", last_success: null, poll_seconds: 60 } },
    rows: [{ source: "codex", model: "model-unchanged", calls: 2, input_tokens: 1234, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0, known_cost_usd: 0, unknown_cost_calls: 2, estimated_cost_calls: 0, fallback_cost_calls: 0, first_ts: 1, last_ts: 2 }],
    cursor_history_days: 30, since: "2026-09-01T00:00:00Z", generated_at: "2026-09-22T00:00:00Z",
  };

  function render(client: QueryClient, pricing = false) {
    return renderToStaticMarkup(<I18nextProvider i18n={i18n}><QueryClientProvider client={client}><CurrencyCtx.Provider value={{ currency: "USD", toggle: () => {}, rate: null, fetchedAt: null, fmt: value => `$${value}` }}>{pricing ? <DataPricing period="month" /> : <UnifiedUsage period="month" setPeriod={() => {}} />}</CurrencyCtx.Provider></QueryClientProvider></I18nextProvider>);
  }

  it("changes cached data labels and numbers without modifying usage or source diagnostics", async () => {
    const client = new QueryClient();
    client.setQueryData(["unified-usage", "month"], snapshot);
    await i18n.changeLanguage("en");
    expect(render(client)).toContain("Cost unavailable");
    await i18n.changeLanguage("de");
    const german = render(client);
    expect(german).toContain("Kosten nicht verfügbar");
    expect(german).toContain("1.234 insgesamt");
    expect(german).toContain("permission denied /data/codex");
    expect(client.getQueryData(["unified-usage", "month"])).toBe(snapshot);
    expect(render(client, true)).toContain("Kosten nicht verfügbar");
    client.clear();
  });

  it("translates the same HTTP error when the language changes", async () => {
    const error = new UsageFetchError(503);
    await i18n.changeLanguage("en");
    expect(usageErrorMessage(error, i18n.getFixedT(null, "usage"))).toContain("Collector unavailable (HTTP 503)");
    await i18n.changeLanguage("de");
    expect(usageErrorMessage(error, i18n.getFixedT(null, "usage"))).toContain("Datensammler nicht erreichbar (HTTP 503)");
    expect(usageErrorMessage(new Error("private raw diagnostic"), i18n.getFixedT(null, "usage"))).toContain("Nutzung konnte nicht geladen werden");
  });

  it("keeps motion identity stable across language and polling timestamps, while marking numeric changes", async () => {
    const client = new QueryClient();
    client.setQueryData(["unified-usage", "month"], snapshot);
    const signature = () => {
      const card = render(client).match(/<[^>]*data-motion="usage-summary-tokens"[^>]*>/)?.[0];
      expect(card).toBeDefined();
      return card?.match(/data-motion-update="([^"]*)"/)?.[1];
    };
    await i18n.changeLanguage("en");
    const initial = signature();
    expect(initial).toBe("1234");
    await i18n.changeLanguage("de");
    expect(signature()).toBe(initial);
    client.setQueryData(["unified-usage", "month"], { ...snapshot, generated_at: "2026-09-22T00:00:30Z" });
    expect(signature()).toBe(initial);
    client.setQueryData(["unified-usage", "month"], { ...snapshot, rows: [{ ...snapshot.rows[0], input_tokens: 1235 }] });
    expect(signature()).toBe("1235");
    client.clear();
  });
});
