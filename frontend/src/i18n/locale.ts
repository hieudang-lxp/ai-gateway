import { useTranslation } from "react-i18next";

export const languages = [
  { code: "en", name: "English" },
  { code: "vi", name: "Tiếng Việt" },
  { code: "ko", name: "한국어" },
  { code: "zh-Hans", name: "简体中文" },
  { code: "de", name: "Deutsch" },
] as const;
export type Language = typeof languages[number]["code"];
const storageKey = "ai-gateway.language";

function supportedLocale(input: string): Language | undefined {
  const locale = input.toLowerCase().replaceAll("_", "-");
  if (/^zh(?:-|$)/.test(locale)) {
    if (/hant|-(tw|hk|mo)(-|$)/.test(locale)) return undefined;
    if (locale === "zh" || /^zh-(hans|cn|sg)(-|$)/.test(locale)) return "zh-Hans";
    return undefined;
  }
  return languages.find(({ code }) => code === locale.split("-")[0])?.code;
}

export function resolveLocale(input: string): Language {
  return supportedLocale(input) ?? "en";
}

export function detectLanguage(storage: Pick<Storage, "getItem"> | undefined, browserLanguages: readonly string[] = []): Language {
  try {
    const saved = storage?.getItem(storageKey);
    if (saved && supportedLocale(saved)) return supportedLocale(saved)!;
  } catch { /* Private browsing can deny preference storage. */ }
  let traditionalChinese = false;
  for (const input of browserLanguages) {
    const normalized = input.toLowerCase().replaceAll("_", "-");
    if (/^zh(?:-|$)/.test(normalized) && /hant|-(tw|hk|mo)(-|$)/.test(normalized)) traditionalChinese = true;
    // Browsers append generic "zh" to zh-TW/zh-Hant; that is not a script preference.
    if (traditionalChinese && normalized === "zh") continue;
    const language = supportedLocale(input);
    if (language) return language;
  }
  return "en";
}

export function persistLanguage(storage: Pick<Storage, "setItem"> | undefined, language: Language) {
  try { storage?.setItem(storageKey, language); } catch { /* Switching still works in memory. */ }
}

export function browserStorage(): Storage | undefined {
  try { return typeof window === "undefined" ? undefined : window.localStorage; } catch { return undefined; }
}

export function useLocale(): Language {
  const { i18n } = useTranslation();
  return resolveLocale(i18n.resolvedLanguage ?? i18n.language ?? "en");
}
