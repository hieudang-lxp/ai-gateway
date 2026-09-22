import { expect, it } from "vitest";
import { createInstance } from "i18next";
import { resources } from "./index";
import { languages } from "./locale";

function parameters(text: string) {
  return [...text.matchAll(/{{\s*([^},\s]+)(?:[^}]*)}}/g)].map(match => match[1]).sort();
}

it("ships every translation and preserves interpolation contracts", () => {
  const english = resources.en;
  for (const { code } of languages) {
    const localized = resources[code];
    expect(Object.keys(localized).sort()).toEqual(Object.keys(english).sort());
    for (const namespace of Object.keys(english)) {
      const original = english[namespace as keyof typeof english];
      const translated = localized[namespace as keyof typeof localized];
      expect(Object.keys(translated).sort(), `${code}/${namespace}`).toEqual(Object.keys(original).sort());
      for (const key of Object.keys(original)) {
        const base = (original as Record<string, string>)[key];
        const value = (translated as Record<string, string>)[key];
        expect(value.trim().length, `${code}/${namespace}/${key}`).toBeGreaterThan(0);
        expect(parameters(value), `${code}/${namespace}/${key}`).toEqual(parameters(base));
      }
    }
  }
});

it("renders stored status keys in the newly selected language", async () => {
  const instance = createInstance();
  await instance.init({ resources, lng: "en", fallbackLng: false, initAsync: false });
  expect(instance.t("common:sessions")).toBe("Sessions");
  await instance.changeLanguage("ko");
  expect(instance.t("common:sessions")).toBe("세션");
});
