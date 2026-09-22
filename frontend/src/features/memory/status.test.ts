import { describe, expect, it } from "vitest";
import { createInstance } from "i18next";
import { memoryLocales } from "@/i18n/memory";
import { memoryHealth as healthState, parseMemoryStatus, type MemoryStatus } from "./status";

const i18n = createInstance();
void i18n.init({ lng: "en", fallbackLng: false, initAsync: false, resources: { en: { memory: memoryLocales.en } } });
// Keep the existing semantic assertions at the rendering boundary while health stays language-independent.
const memoryHealth = (...args: Parameters<typeof healthState>) => {
  const health = healthState(...args);
  return { ...health, title: i18n.t(health.titleKey, { ns: "memory" }), description: i18n.t(health.descriptionKey, { ns: "memory", ...health.params }) };
};

const now = Date.parse("2026-09-22T12:00:00Z");
const source = () => ({ state: "ok", pending: 0, delivered: 10, blocked: 0, last_success: "2026-09-22T11:59:30Z" });
const status = (): MemoryStatus => ({
  enabled: true, state: "ok", ready: true, pending: 0, delivered: 30, blocked: 0,
  last_check: "2026-09-22T11:59:30Z",
  sources: { claude_code: source(), codex: source(), cursor: source() },
  receiver: {
    version: 1, ready: true, capabilities: { ingest: true },
    dependencies: { llm: true, embedder: true, neo4j: true },
    queue: { pending: 0, processing: 0, succeeded: 15, failed: 0, legacy_uncertain: 0 },
  },
});

describe("memory status parsing", () => {
  it("accepts source counts and independently reported extraction counts", () => {
    expect(parseMemoryStatus(status())).toEqual(status());
  });

  it("accepts omitted receiver metadata and nullable status fields", () => {
    const data = status();
    delete data.receiver;
    expect(parseMemoryStatus(data).receiver).toBeUndefined();
    data.receiver = null;
    data.last_check = null;
    data.error = null;
    data.sources.codex.last_success = null;
    expect(parseMemoryStatus(data)).toEqual(data);
  });

  it("accepts null or absent receiver queue and dependencies", () => {
    const data = status();
    data.receiver = { version: 1, ready: true, capabilities: { ingest: true }, queue: null, dependencies: null };
    expect(parseMemoryStatus(data).receiver?.queue).toBeNull();
    delete data.receiver.queue;
    delete data.receiver.dependencies;
    expect(parseMemoryStatus(data).receiver?.queue).toBeUndefined();
  });

  it.each([
    null, [], "private-message", {},
    { ...status(), enabled: "true" },
    { ...status(), ready: 1 },
    { ...status(), state: null },
    { ...status(), pending: -1 },
    { ...status(), delivered: Number.NaN },
    { ...status(), blocked: Number.POSITIVE_INFINITY },
    { ...status(), sources: null },
    { ...status(), sources: { codex: { ...source(), pending: "1" } } },
    { ...status(), sources: { codex: { ...source(), state: 1 } } },
    { ...status(), last_check: 123 },
    { ...status(), receiver: { ...status().receiver, version: 2 } },
    { ...status(), receiver: { ...status().receiver, capabilities: { ingest: "true" } } },
    { ...status(), receiver: { ...status().receiver, dependencies: { llm: "ready" } } },
    { ...status(), receiver: { ...status().receiver, queue: { pending: -1 } } },
  ])("rejects malformed payload without including its content", payload => {
    expect(() => parseMemoryStatus(payload)).toThrow(/Invalid memory status/);
    expect(() => parseMemoryStatus(payload)).not.toThrow(/private-message/);
  });
});

describe("memory health", () => {
  it("returns language-independent keys and count parameters for cached health", () => {
    expect(memoryHealth({ ...status(), blocked: 2 }, false, now)).toMatchObject({
      titleKey: "healthDelivery", descriptionKey: "healthBlockedDescription", params: { count: 2 },
    });
  });
  it("renders singular source and blocked-message counts without changing cached state", () => {
    expect(memoryHealth({ ...status(), blocked: 1 }, false, now).description).toContain("1 blocked message is retained");
    const missing = status(); delete missing.sources.codex;
    expect(memoryHealth(missing, false, now).description).toContain("1 source report is missing");
    expect(memoryHealth({ ...status(), pending: 1 }, false, now).description).toContain("1 message awaiting delivery");
  });
  it("does not hide a fetch error behind healthy or disabled cached data", () => {
    expect(memoryHealth(status(), true, now)).toMatchObject({ tone: "warning", title: "Connection issue" });
    expect(memoryHealth({ ...status(), enabled: false }, true, now).tone).toBe("warning");
    expect(memoryHealth(undefined, false, now)).toMatchObject({ tone: "neutral", stale: false });
  });

  it("treats disabled configuration as neutral regardless of old errors", () => {
    const data = { ...status(), enabled: false, ready: false, last_check: "invalid", error: "old error" };
    expect(memoryHealth(data, false, now)).toMatchObject({ tone: "neutral", title: "Local memory is off", stale: false });
  });

  it.each(["2026-09-22T11:57:59Z", "invalid", null, undefined])("does not report stale or invalid status as healthy (%s)", last_check => {
    expect(memoryHealth({ ...status(), last_check }, false, now)).toMatchObject({ tone: "warning", title: "Status is stale", stale: true });
  });

  it("allows the first readiness check without declaring stale data", () => {
    expect(memoryHealth({ ...status(), state: "waiting", ready: false, last_check: null }, false, now))
      .toMatchObject({ tone: "neutral", title: "Waiting for first check", stale: false });
    expect(memoryHealth({ ...status(), last_check: "2026-09-22T11:58:00Z" }, false, now).stale).toBe(false);
  });

  it("reports unavailable Graphiti dependencies before delivery health", () => {
    expect(memoryHealth({ ...status(), ready: false }, false, now)).toMatchObject({ tone: "warning", title: "Waiting for Graphiti" });
    const data = status();
    data.receiver!.dependencies = { llm: false, embedder: true, neo4j: true };
    expect(memoryHealth(data, false, now).tone).toBe("warning");
  });

  it.each(["error", "unsupported", "waiting", "disabled"])("does not hide a %s source behind otherwise healthy delivery", state => {
    const data = status(); data.sources.codex.state = state;
    expect(memoryHealth(data, false, now)).toMatchObject({ tone: "warning", title: "Source needs attention" });
  });

  it("reports missing coverage, explicit errors, and blocked delivery", () => {
    const missing = status(); delete missing.sources.cursor;
    expect(memoryHealth(missing, false, now).tone).toBe("warning");
    expect(memoryHealth({ ...status(), blocked: 1, pending: 1 }, false, now)).toMatchObject({ tone: "warning", title: "Delivery needs attention" });
    expect(memoryHealth({ ...status(), error: "HTTP 401" }, false, now).tone).toBe("warning");
    const sourceError = status(); sourceError.sources.codex.error = "source unreadable";
    expect(memoryHealth(sourceError, false, now).tone).toBe("warning");
  });

  it.each(["2026-09-22T11:57:00Z", "invalid", null, undefined])("does not call an unread or stale source healthy (%s)", last_success => {
    const data = status(); data.sources.codex.last_success = last_success;
    expect(memoryHealth(data, false, now)).toMatchObject({ tone: "warning", title: "Source needs attention", stale: true });
  });

  it.each(["failed", "legacy_uncertain"])("reports receiver %s outcomes instead of declaring completion", state => {
    const data = status(); data.receiver!.queue![state] = 2;
    const health = memoryHealth(data, false, now);
    expect(health).toMatchObject({ tone: "warning", title: "Graphiti needs attention" });
    expect(health.description).not.toMatch(/0 failed|0 legacy/);
  });

  it.each(["pending", "processing"])("shows receiver %s backlog without claiming the worker is alive", state => {
    const data = status(); data.receiver!.queue![state] = 4;
    const health = memoryHealth(data, false, now);
    expect(health).toMatchObject({ tone: "neutral", title: "Work queued", stale: false });
    expect(health.description).toContain("4");
    expect(health.description).toContain("Worker activity is not reported");
    expect(health.description).not.toMatch(/actively|running|worker is healthy|%/i);
  });

  it("shows sender backlog separately from extraction", () => {
    const health = memoryHealth({ ...status(), pending: 7 }, false, now);
    expect(health).toMatchObject({ tone: "neutral", title: "Work queued" });
    expect(health.description).toContain("7 messages awaiting delivery");
  });

  it("does not infer extraction from delivery when receiver queue is absent", () => {
    const data = status(); data.receiver = null;
    const health = memoryHealth(data, false, now);
    expect(health).toMatchObject({ tone: "neutral", title: "Delivery up to date" });
    expect(health.description).toContain("Extraction status is unavailable");
    expect(health.description).not.toMatch(/30.*extract|100%/i);
  });

  const unavailableQueues: Array<Record<string, number> | null | undefined> = [null, undefined, {}, { pending: 0, failed: 0 }, { pending: 0, processing: 0, failed: 0 }];
  it.each(unavailableQueues)("keeps an unavailable receiver queue unknown (%s)", queue => {
    const data = status(); data.receiver!.queue = queue;
    expect(memoryHealth(data, false, now)).toMatchObject({ tone: "neutral", title: "Delivery up to date" });
    data.pending = 4;
    const health = memoryHealth(data, false, now);
    expect(health.description).toContain("4 messages awaiting delivery");
    expect(health.description).toContain("Graphiti queue status is unavailable");
    expect(health.description).not.toContain("0 pending");
  });

  it("reports fresh healthy sources and empty queues without equating delivery to extraction", () => {
    const health = memoryHealth(status(), false, now);
    expect(health).toMatchObject({ tone: "good", title: "Delivery up to date", stale: false });
    expect(health.description).toContain("No messages awaiting delivery");
    expect(health.description).not.toContain("30");
  });
});
