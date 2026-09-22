import { describe, expect, it } from "vitest";
import { syncStatus, syncState, type collectorIds } from "./syncStatus";
import type { SourceStatus } from "./useUsageSummary";
import { createInstance } from "i18next";
import { usageLocales } from "@/i18n/usage";

describe("collector navigation status", () => {
  const now = Date.parse("2026-09-18T01:00:00Z");
  const source = (seconds: number): SourceStatus => ({ state: "ok", poll_seconds: 60, last_success: new Date(now - seconds * 1000).toISOString() });
  const sources = (): Record<typeof collectorIds[number], SourceStatus> => ({ claude_code: source(10), codex: source(20), cursor: source(50) });
  it("requires all three sources and shows the oldest successful sync", () => {
    expect(syncStatus(sources(), false, now)).toEqual({ label: "All collectors synced", healthy: true, lastSync: source(50).last_success });
    expect(syncStatus({}, false, now).healthy).toBe(false);
  });
  it.each(["stale", "error", "missing timestamp"])("does not mark %s data healthy", kind => {
    const s = sources();
    if (kind === "stale") s.codex = source(181);
    if (kind === "error") s.codex.state = "error";
    if (kind === "missing timestamp") s.codex.last_success = null;
    expect(syncStatus(s, false, now).label).toBe("2/3 collectors ready");
  });
  it("does not hide connection errors behind cached successful data", () => {
    expect(syncStatus(sources(), true, now)).toMatchObject({ healthy: false, label: "Connection issue" });
    expect(syncStatus(undefined, false, now).label).toBe("Connecting…");
  });
  it("keeps cached status independent of the display language", async () => {
    const state = syncState(sources(), true, now);
    const translations = createInstance();
    await translations.init({ lng: "en", resources: { en: { usage: usageLocales.en }, de: { usage: usageLocales.de } }, defaultNS: "usage" });
    expect(translations.t(state.key, state.values)).toBe("Connection issue");
    await translations.changeLanguage("de");
    expect(translations.t(state.key, state.values)).toBe("Verbindungsproblem");
    expect(syncState({}, false, now)).toMatchObject({ key: "syncReady", values: { ready: 0 }, healthy: false });
  });
});
