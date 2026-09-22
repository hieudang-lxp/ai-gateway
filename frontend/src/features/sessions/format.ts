import type { Session } from "./types";
import i18n from '@/i18n';
import { formatDate, formatNumber } from '@/i18n/format';
export const sourceName = (source: string) => ({ codex: "Codex", claude_code: "Claude Code", cursor: "Cursor" }[source] ?? source);
export const count = (n: number) => formatNumber(n);
export const timestamp = (ts: number) => formatDate(ts * 1000);
export const tokens = (s: Pick<Session, "input_tokens" | "output_tokens" | "cache_read_tokens" | "cache_write_tokens">) => s.input_tokens + s.output_tokens + s.cache_read_tokens + s.cache_write_tokens;

export function sessionTitle(session: Pick<Session, "title" | "source" | "models">) {
  if (session.title.trim()) return session.title;
  if (session.source === "codex" && session.models.length === 1 && session.models[0] === "codex-auto-review") {
    return i18n.t('autoReview', { ns: 'sessions' });
  }
  return i18n.t('untitled', { ns: 'sessions', source: sourceName(session.source) });
}

export const shortSessionID = (id: string) => id.length > 20 ? `${id.slice(0, 13)}…${id.slice(-4)}` : id;
