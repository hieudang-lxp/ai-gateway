import { describe, expect, it } from "vitest";
import { detectLanguage, persistLanguage, resolveLocale } from "./locale";

describe("locale selection", () => {
  it.each([["ko-KR", "ko"], ["vi-VN", "vi"], ["de-AT", "de"], ["zh", "zh-Hans"], ["zh-CN", "zh-Hans"], ["zh-Hans-SG", "zh-Hans"], ["zh-TW", "en"], ["zh-Hant", "en"], ["zh-HK", "en"], ["fr", "en"]])("resolves %s safely", (input, expected) => {
    expect(resolveLocale(input)).toBe(expected);
  });
  it("prefers the saved language over browser language", () => {
    expect(detectLanguage({ getItem: () => "de" }, ["vi-VN"])).toBe("de");
  });
  it("uses the first supported browser language without accepting invalid stored data", () => {
    expect(detectLanguage({ getItem: () => "invalid" }, ["fr-FR", "ko-KR"])).toBe("ko");
  });
  it("does not interpret a generic Chinese fallback as consent to Simplified", () => {
    expect(detectLanguage(undefined, ["zh-TW", "zh", "en-US"])).toBe("en");
    expect(detectLanguage(undefined, ["zh-Hant", "zh"])).toBe("en");
    expect(detectLanguage(undefined, ["zh-Hant", "zh", "ko-KR"])).toBe("ko");
  });
  it("survives blocked browser storage", () => {
    expect(detectLanguage({ getItem: () => { throw new Error("blocked"); } }, ["vi-VN"])).toBe("vi");
    expect(() => persistLanguage({ setItem: () => { throw new Error("blocked"); } }, "de")).not.toThrow();
  });
  it("saves the explicit choice", () => {
    const saved: string[] = [];
    persistLanguage({ setItem: (_key, value) => { saved.push(value); } }, "zh-Hans");
    expect(saved).toEqual(["zh-Hans"]);
  });
});
