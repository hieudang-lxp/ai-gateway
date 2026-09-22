import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import { memoryLocales } from "@/i18n/memory";
import { LocalMemoryView } from "./LocalMemoryView";
import type { MemoryStatus } from "./status";

const now = Date.parse("2026-09-22T06:00:00Z");
const data: MemoryStatus = {
  enabled: true, state: "ok", ready: true, pending: 0, delivered: 2306, blocked: 0,
  last_check: new Date(now).toISOString(),
  sources: Object.fromEntries(["claude_code", "codex", "cursor"].map(id => [id, {
    state: "ok", pending: 0, delivered: 12, blocked: 0, last_success: new Date(now).toISOString(),
  }])),
  receiver: { version: 1, ready: true, capabilities: { ingest: true },
    dependencies: { llm: true, embedder: true, neo4j: true },
    queue: { pending: 2362, processing: 1, succeeded: 1, failed: 0, excluded: 415 } },
};
const i18n = createInstance();
void i18n.init({ lng: "en", fallbackLng: false, initAsync: false, interpolation: { escapeValue: false }, resources: Object.fromEntries(Object.entries(memoryLocales).map(([language, memory]) => [language, { memory }])) });
const render = (overrides: Partial<Parameters<typeof LocalMemoryView>[0]> = {}, language = "en") => {
  void i18n.changeLanguage(language);
  return renderToStaticMarkup(<I18nextProvider i18n={i18n}><LocalMemoryView data={data} isError={false} isFetching={false} now={now} onRefresh={() => {}} {...overrides} /></I18nextProvider>);
};

describe("Local Memory dashboard", () => {
  it.each([
    ["vi", "Lỗi kết nối", "Dữ liệu gần nhất đã biết"],
    ["ko", "연결 문제", "마지막으로 확인된 상태"],
    ["zh-Hans", "连接问题", "最后已知快照"],
    ["de", "Verbindungsproblem", "Zuletzt bekannter Stand"],
  ])("translates the same cached failure and metadata in %s", (language, warning, saved) => {
    const cached = { ...data, error: "Upstream raw diagnostic", sources: {} };
    const english = render({ data: cached, isError: true });
    const translated = render({ data: cached, isError: true }, language);
    expect(english).toContain("Connection issue");
    expect(translated).toContain(warning);
    expect(translated).toContain(saved);
    expect(translated).toContain("Upstream raw diagnostic");
    expect(translated).not.toContain("Refresh memory status");
    expect(translated).not.toContain("Accepted by Graphiti");
  });
  it("uses the active language for cached numbers and missing data labels", () => {
    const html = render({ data: { ...data, receiver: { ...data.receiver!, dependencies: {}, queue: { failed: 0 } } } }, "de");
    expect(html).toContain("2.306");
    expect(html).toContain('aria-label="Unbekannt"');
  });
  it("formats warning counts in the active language and uses singular job wording", () => {
    const snapshot = { ...data, receiver: { ...data.receiver!, queue: { failed: 2306, legacy_uncertain: 1 } } };
    expect(render({ data: snapshot }, "de")).toContain("2.306 fehlgeschlagene Aufträge");
    const english = render({ data: { ...snapshot, receiver: { ...snapshot.receiver, queue: { failed: 1, legacy_uncertain: 1 } } } });
    expect(english).toContain("1 failed job.");
    expect(english).toContain("1 legacy job needs review.");
    expect(english).not.toContain("1 failed jobs");
  });
  it("keeps failed and legacy review counts outside disclosures when a source also fails", () => {
    const html = render({ data: { ...data, sources: {}, receiver: { ...data.receiver!, queue: { failed: 3, legacy_uncertain: 2 } } } });
    const visible = html.replace(/<details[\s\S]*?<\/details>/g, "");
    expect(visible).toContain("3 failed jobs");
    expect(visible).toContain("2 legacy jobs need review");
  });
  it("separates accepted messages from extracted jobs, without a false progress percentage", () => {
    const html = render();
    expect(html).toContain("Accepted by Graphiti");
    expect(html).toContain("2,306");
    expect(html).toContain("Completed jobs");
    expect(html).toContain("2,362");
    expect(html).not.toContain("progressbar");
    expect(html).not.toContain("All synced");
  });
  it("does not show fabricated counts before the first response", () => {
    const html = render({ data: undefined, isFetching: true });
    expect(html).toContain("Checking local memory");
    expect(html).not.toContain("Accepted by Graphiti");
  });
  it("explains optional configuration when disabled", () => {
    const html = render({ data: { ...data, enabled: false, state: "disabled" } });
    expect(html).toContain("GRAPHITI_URL");
    expect(html).not.toContain("Accepted by Graphiti");
  });
  it("keeps last known counts with an explicit warning after refresh failure", () => {
    const html = render({ isError: true });
    expect(html).toContain('role="alert"');
    expect(html).toContain("Last known snapshot");
    expect(html).toContain("2,306");
  });
  it("marks unavailable extraction status as unknown instead of zero", () => {
    const html = render({ data: { ...data, receiver: undefined } });
    expect(html).toContain("Extraction status unavailable");
    expect(html).not.toContain("Completed jobs");
  });
  it("shows delivery errors even when readiness and individual sources succeeded", () => {
    const html = render({ data: { ...data, error: "Graphiti ingestion HTTP 503" } });
    expect(html).toContain("Graphiti ingestion HTTP 503");
  });
  it("does not mark an old source scan as checked", () => {
    const html = render({ data: { ...data, sources: { codex: { state: "ok", pending: 0, delivered: 1, blocked: 0, last_success: "2020-01-01T00:00:00Z" } } } });
    expect(html).toContain("Scan is stale");
    expect(html).not.toContain(">Checked<");
  });
});
