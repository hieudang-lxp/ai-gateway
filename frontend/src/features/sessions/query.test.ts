import { describe, expect, it } from "vitest";
import { changeLabel, periodParams, sessionLink, sessionParams, sessionSelection } from "./query";
describe("indexed usage query parameters", () => {
  it("keeps month and day parameters mutually exclusive", () => {
    expect(periodParams("month").toString()).toBe("period=month");
    expect(periodParams("0").toString()).toBe("days=0");
    expect(periodParams("7").toString()).toBe("days=7");
  });
  it("encodes submitted filters without empty values or previous pagination", () => {
    const params = sessionParams({ period: "30", source: "codex", q: "  fix + cache & parser ", model: "" });
    expect(params.get("q")).toBe("fix + cache & parser");
    expect(params.has("model")).toBe(false);
    expect(params.has("cursor")).toBe(false);
    expect(params.get("limit")).toBe("25");
    expect(params.get("source")).toBe("codex");
  });
});
describe("session deep links", () => {
  it("round-trips identifiers with reserved characters", () => {
    expect(sessionSelection(sessionLink("claude_code", "id/with+spaces & ?"))).toEqual({ source: "claude_code", sessionID: "id/with+spaces & ?" });
  });
  it.each(["#sessions", "#sessions?source=codex", "#sessions?session_id=a", "#insights?source=codex&session_id=a"])("does not open an incomplete or unrelated link: %s", hash => {
    expect(sessionSelection(hash)).toBeNull();
  });
});
it("avoids infinite or fabricated percentage change with no baseline", () => {
  expect(changeLabel(5, 0)).toBe("No prior baseline");
  expect(changeLabel(0, 0)).toBe("No change");
  expect(changeLabel(15, 10)).toContain("+50%");
  expect(changeLabel(0, 10)).toContain("-100%");
});
