import { describe, it, expect, beforeEach } from "vitest";
import { getToken, setToken, clearToken } from "./token";

beforeEach(() => {
  const store = new Map<string, string>();
  (globalThis as Record<string, unknown>).localStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
  };
  clearToken();
});

describe("token storage", () => {
  it("keeps authentication usable in memory if storage is blocked", () => {
    (globalThis as Record<string, unknown>).localStorage = {
      getItem: () => { throw new Error("blocked"); },
      setItem: () => { throw new Error("blocked"); },
      removeItem: () => { throw new Error("blocked"); },
    };
    expect(getToken()).toBe("");
    expect(() => setToken("session-only-token")).not.toThrow();
    expect(getToken()).toBe("session-only-token");
    expect(() => clearToken()).not.toThrow();
    expect(getToken()).toBe("");
  });
  it("defaults to empty string", () => {
    expect(getToken()).toBe("");
  });
  it("round-trips", () => {
    setToken("abc");
    expect(getToken()).toBe("abc");
    clearToken();
    expect(getToken()).toBe("");
  });
});
