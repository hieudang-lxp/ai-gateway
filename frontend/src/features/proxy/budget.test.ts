import { describe, it, expect } from "vitest";
import { barState } from "./budget";

describe("barState", () => {
  it("no thresholds -> no cap, ok", () => {
    expect(barState(5, 0, 0)).toEqual({ pct: 0, level: "ok", cap: null });
  });
  it("under warn", () => {
    expect(barState(5, 10, 20)).toEqual({ pct: 25, level: "ok", cap: 20 });
  });
  it("at warn", () => {
    expect(barState(10, 10, 20)).toEqual({ pct: 50, level: "warn", cap: 20 });
  });
  it("over hard clamps to 100", () => {
    expect(barState(25, 10, 20)).toEqual({ pct: 100, level: "over", cap: 20 });
  });
  it("warn-only budget uses warn as cap", () => {
    expect(barState(5, 10, 0)).toEqual({ pct: 50, level: "ok", cap: 10 });
  });
});
