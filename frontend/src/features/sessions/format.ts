import type { Session } from "./types";
export const sourceName = (source: string) => ({ codex: "Codex", claude_code: "Claude Code", cursor: "Cursor" }[source] ?? source);
export const count = (n: number) => n.toLocaleString();
export const timestamp = (ts: number) => new Date(ts * 1000).toLocaleString();
export const tokens = (s: Pick<Session, "input_tokens" | "output_tokens" | "cache_read_tokens" | "cache_write_tokens">) => s.input_tokens + s.output_tokens + s.cache_read_tokens + s.cache_write_tokens;

export function sessionTitle(session: Pick<Session, "title" | "source" | "models">) {
  if (session.title.trim()) return session.title;
  if (session.source === "codex" && session.models.length === 1 && session.models[0] === "codex-auto-review") {
    return "Codex auto-review";
  }
  return `Untitled ${sourceName(session.source)} session`;
}

export const shortSessionID = (id: string) => id.length > 20 ? `${id.slice(0, 13)}…${id.slice(-4)}` : id;
