import type { Session } from "./types";
export const sourceName = (source: string) => ({ codex: "Codex", claude_code: "Claude Code", cursor: "Cursor" }[source] ?? source);
export const count = (n: number) => n.toLocaleString();
export const timestamp = (ts: number) => new Date(ts * 1000).toLocaleString();
export const tokens = (s: Pick<Session, "input_tokens" | "output_tokens" | "cache_read_tokens" | "cache_write_tokens">) => s.input_tokens + s.output_tokens + s.cache_read_tokens + s.cache_write_tokens;
