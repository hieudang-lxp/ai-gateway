import { describe, it, expect } from "vitest";
import { compactTokens } from "./format";

describe("compactTokens", () => {
  it("passes small numbers through", () => {
    expect(compactTokens(604n)).toBe("604");
  });
  it("compacts thousands and millions", () => {
    expect(compactTokens(86231n)).toBe("86.23K");
    expect(compactTokens(1439668n)).toBe("1.44M");
  });
});
