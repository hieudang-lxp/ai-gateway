import { describe, expect, it } from "vitest";
import { summarizeUsage } from "./usage";

describe("unified usage", () => {
  it("adds exclusive token categories and keeps unknown costs visible", () => {
    expect(summarizeUsage([
      { calls: 1, input_tokens: 40, output_tokens: 20, cache_read_tokens: 60, cache_write_tokens: 0, known_cost_usd: 0, unknown_cost_calls: 1 },
      { calls: 2, input_tokens: 10, output_tokens: 5, cache_read_tokens: 0, cache_write_tokens: 5, known_cost_usd: 2.5, unknown_cost_calls: 0 },
    ])).toEqual({ calls: 3, tokens: 140, cost: 2.5, unknown: 1 });
  });
});
