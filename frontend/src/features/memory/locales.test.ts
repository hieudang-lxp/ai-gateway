import { describe, expect, it } from "vitest";
import { memoryLocales } from "@/i18n/memory";

describe("memory translations", () => {
  it.each(Object.entries(memoryLocales))("%s has every key and the same interpolation contract", (_language, dictionary) => {
    expect(Object.keys(dictionary).sort()).toEqual(Object.keys(memoryLocales.en).sort());
    for (const key of Object.keys(memoryLocales.en) as Array<keyof typeof memoryLocales.en>) {
      expect(dictionary[key].trim(), key).not.toBe("");
      const params = (value: string) => [...value.matchAll(/\{\{([^}]+)\}\}/g)].map(match => match[1]).sort();
      expect(params(dictionary[key]), key).toEqual(params(memoryLocales.en[key]));
    }
  });
});
