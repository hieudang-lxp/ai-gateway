export type UsageCounts = {
  calls: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  known_cost_usd: number;
  unknown_cost_calls: number;
};

export function summarizeUsage(rows: UsageCounts[]) {
  return rows.reduce((total, row) => ({
    calls: total.calls + row.calls,
    tokens: total.tokens + row.input_tokens + row.output_tokens + row.cache_read_tokens + row.cache_write_tokens,
    cost: total.cost + row.known_cost_usd,
    unknown: total.unknown + row.unknown_cost_calls,
  }), { calls: 0, tokens: 0, cost: 0, unknown: 0 });
}
