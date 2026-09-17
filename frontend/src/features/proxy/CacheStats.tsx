import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import { compactTokens } from "../../lib/format";
import { useCurrency } from "../currency/useCurrency";

function Tile({
  label,
  value,
  sub,
  title,
}: {
  label: string;
  value: string;
  sub?: string;
  title?: string;
}) {
  return (
    <div className="rounded-xl bg-sky-50/60 p-4" title={title}>
      <div className="text-xs font-medium text-slate-500">{label}</div>
      <div className="mt-1 text-2xl font-bold tabular-nums tracking-tight text-sky-950">
        {value}
      </div>
      {sub && <div className="mt-0.5 text-[11px] text-slate-400">{sub}</div>}
    </div>
  );
}

export function CacheStats() {
  const { data } = useQuery(StatsService.method.overview, {});
  const { data: models } = useQuery(StatsService.method.modelBreakdown, {
    days: 30,
  });
  const { fmt } = useCurrency();

  // token totals over the same 30d window as the Models table
  let input = 0n,
    output = 0n,
    cacheRd = 0n,
    cacheWr = 0n;
  for (const r of models?.rows ?? []) {
    input += r.inputTokens;
    output += r.outputTokens;
    cacheRd += r.cacheReadTokens;
    cacheWr += r.cacheWriteTokens;
  }

  const total = Number(data?.totalCalls ?? 0n);
  const hits = Number(data?.cacheHits ?? 0n);
  const rate = total > 0 ? ((hits / total) * 100).toFixed(1) + "%" : "—";

  return (
    <section className="grid h-full grid-cols-2 gap-4 rounded-2xl border border-sky-100 bg-white p-6 shadow-sm sm:grid-cols-3">
      <Tile label="Total calls" value={total.toLocaleString()} sub="all time" />
      <Tile
        label="Input tokens"
        value={compactTokens(input)}
        sub="last 30 days"
        title="Tokens gửi lên (không tính cache)"
      />
      <Tile
        label="Output tokens"
        value={compactTokens(output)}
        sub="last 30 days"
        title="Tokens model trả về"
      />
      <Tile
        label="Cache read"
        value={compactTokens(cacheRd)}
        sub="last 30 days"
        title="Prompt-cache của Anthropic đọc lại (rẻ hơn 10x input)"
      />
      <Tile
        label="Cache write"
        value={compactTokens(cacheWr)}
        sub="last 30 days"
        title="Ghi vào prompt-cache của Anthropic (1.25x input)"
      />
      <Tile
        label="Gateway cache"
        value={fmt(data?.cacheSavedUsd ?? 0)}
        sub={`${hits.toLocaleString()} hits · ${rate}`}
        title="Cache exact-match của gateway (bật trong gateway.yaml) — khác prompt-cache của Anthropic"
      />
    </section>
  );
}
