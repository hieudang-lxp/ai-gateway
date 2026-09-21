import { describe, it, expect, beforeEach } from "vitest";
import { getToken, setToken, clearToken } from "./token";

beforeEach(() => {
  const store = new Map<string, string>();
  (globalThis as Record<string, unknown>).localStorage = {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
  };
});

describe("token storage", () => {
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
