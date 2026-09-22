import type { memoryLocales } from "@/i18n/memory";

export interface MemorySourceStatus {
  state: string;
  pending: number;
  delivered: number;
  blocked: number;
  error?: string | null;
  last_success?: string | null;
}

export interface MemoryReceiverStatus {
  version: 1;
  ready: boolean;
  capabilities: { ingest: boolean };
  dependencies?: Record<string, boolean> | null;
  queue?: Record<string, number> | null;
}

export interface MemoryStatus {
  enabled: boolean;
  state: string;
  ready: boolean;
  pending: number;
  delivered: number;
  blocked: number;
  error?: string | null;
  last_check?: string | null;
  sources: Record<string, MemorySourceStatus>;
  receiver?: MemoryReceiverStatus | null;
}

export interface MemoryHealth {
  tone: "neutral" | "good" | "warning";
  titleKey: keyof typeof memoryLocales.en;
  descriptionKey: keyof typeof memoryLocales.en;
  params?: Record<string, number>;
  stale: boolean;
}

function invalid(field: string): never {
  throw new Error(`Invalid memory status: ${field}`);
}

function object(value: unknown, field: string): Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) invalid(field);
  return value as Record<string, unknown>;
}

function boolean(value: unknown, field: string): boolean {
  if (typeof value !== "boolean") invalid(field);
  return value;
}

function string(value: unknown, field: string): string {
  if (typeof value !== "string") invalid(field);
  return value;
}

function count(value: unknown, field: string): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0) invalid(field);
  return value;
}

function optionalString(value: unknown, field: string): string | null | undefined {
  if (value === undefined || value === null) return value;
  return string(value, field);
}

function parseSource(value: unknown): MemorySourceStatus {
  const data = object(value, "source");
  return {
    state: string(data.state, "source.state"),
    pending: count(data.pending, "source.pending"),
    delivered: count(data.delivered, "source.delivered"),
    blocked: count(data.blocked, "source.blocked"),
    ...(data.error !== undefined && { error: optionalString(data.error, "source.error") }),
    ...(data.last_success !== undefined && { last_success: optionalString(data.last_success, "source.last_success") }),
  };
}

function parseReceiver(value: unknown): MemoryReceiverStatus | null | undefined {
  if (value === null || value === undefined) return value;
  const data = object(value, "receiver");
  if (data.version !== 1) invalid("receiver.version");
  const capabilities = object(data.capabilities, "receiver.capabilities");
  const receiver: MemoryReceiverStatus = {
    version: 1,
    ready: boolean(data.ready, "receiver.ready"),
    capabilities: { ingest: boolean(capabilities.ingest, "receiver.capabilities.ingest") },
  };
  if (data.dependencies === null) receiver.dependencies = null;
  else if (data.dependencies !== undefined) {
    receiver.dependencies = Object.fromEntries(Object.entries(object(data.dependencies, "receiver.dependencies"))
      .map(([key, value]) => [key, boolean(value, "receiver.dependencies value")]));
  }
  if (data.queue === null) receiver.queue = null;
  else if (data.queue !== undefined) {
    receiver.queue = Object.fromEntries(Object.entries(object(data.queue, "receiver.queue"))
      .map(([key, value]) => [key, count(value, "receiver.queue value")]));
  }
  return receiver;
}

export function parseMemoryStatus(value: unknown): MemoryStatus {
  const data = object(value, "response");
  const sources = object(data.sources, "sources");
  return {
    enabled: boolean(data.enabled, "enabled"),
    state: string(data.state, "state"),
    ready: boolean(data.ready, "ready"),
    pending: count(data.pending, "pending"),
    delivered: count(data.delivered, "delivered"),
    blocked: count(data.blocked, "blocked"),
    sources: Object.fromEntries(Object.entries(sources).map(([key, source]) => [key, parseSource(source)])),
    ...(data.error !== undefined && { error: optionalString(data.error, "error") }),
    ...(data.last_check !== undefined && { last_check: optionalString(data.last_check, "last_check") }),
    ...(data.receiver !== undefined && { receiver: parseReceiver(data.receiver) }),
  };
}

export function memoryHealth(data: MemoryStatus | undefined, isError: boolean, now: number): MemoryHealth {
  if (isError) return {
    tone: "warning", titleKey: "healthConnection", stale: false,
    descriptionKey: "healthConnectionDescription",
  };
  if (!data) return {
    tone: "neutral", titleKey: "healthChecking", stale: false,
    descriptionKey: "healthCheckingDescription",
  };
  if (!data.enabled) return {
    tone: "neutral", titleKey: "healthOff", stale: false,
    descriptionKey: "healthOffDescription",
  };
  if (data.last_check == null && data.state === "waiting") return {
    tone: "neutral", titleKey: "healthWaiting", stale: false,
    descriptionKey: "healthWaitingDescription",
  };
  const checked = Date.parse(data.last_check ?? "");
  if (!Number.isFinite(checked) || now - checked > 120_000) return {
    tone: "warning", titleKey: "healthStale", stale: true,
    descriptionKey: "healthStaleDescription",
  };
  const receiver = data.receiver;
  if (!data.ready || receiver?.ready === false || receiver?.capabilities.ingest === false ||
    Object.values(receiver?.dependencies ?? {}).some(ready => !ready)) return {
    tone: "warning", titleKey: "healthGraphiti", stale: false,
    descriptionKey: "healthGraphitiDescription",
  };
  if (data.blocked > 0 || data.error) return {
    tone: "warning", titleKey: "healthDelivery", stale: false,
    descriptionKey: data.blocked > 0 ? "healthBlockedDescription" : "healthDeliveryErrorDescription",
    params: { count: data.blocked },
  };
  const expected = ["claude_code", "codex", "cursor"];
  const missingSources = expected.filter(name => !data.sources[name]).length;
  const sourceIsStale = (source: MemorySourceStatus) => {
    const succeeded = Date.parse(source.last_success ?? "");
    return !Number.isFinite(succeeded) || now - succeeded > 120_000;
  };
  const sourceStatuses = Object.values(data.sources);
  const staleSources = sourceStatuses.some(sourceIsStale);
  const failedSources = sourceStatuses.filter(source => source.state !== "ok" || source.error || source.blocked > 0 || sourceIsStale(source)).length;
  if (missingSources + failedSources > 0) return {
    tone: "warning", titleKey: "healthSource", stale: staleSources,
    descriptionKey: "healthSourceDescription", params: { count: missingSources + failedSources },
  };
  if (data.state !== "ok") return {
    tone: "warning", titleKey: "healthDelivery", stale: false,
    descriptionKey: "healthDeliveryUnknownDescription",
  };
  const queue = receiver?.queue;
  const failed = queue?.failed ?? 0;
  const uncertain = queue?.legacy_uncertain ?? 0;
  if (failed + uncertain > 0) {
    return {
      tone: "warning", titleKey: "healthExtraction", stale: false,
      descriptionKey: "healthExtractionDescription",
    };
  }
  const receiverPending = queue?.pending ?? 0;
  const receiverProcessing = queue?.processing ?? 0;
  const queueKnown = queue?.pending !== undefined && queue.processing !== undefined && queue.failed !== undefined && queue.succeeded !== undefined;
  if (data.pending + receiverPending + receiverProcessing > 0) return {
    tone: "neutral", titleKey: "healthQueued", stale: false,
    descriptionKey: queueKnown ? "healthQueuedDescription" : "healthQueuedUnknownDescription",
    params: queueKnown ? { count: data.pending, pending: receiverPending, processing: receiverProcessing } : { count: data.pending },
  };
  return {
    tone: queueKnown ? "good" : "neutral", titleKey: "healthCurrent", stale: false,
    descriptionKey: queueKnown ? "healthCurrentDescription" : "healthCurrentUnknownDescription",
  };
}
