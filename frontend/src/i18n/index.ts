import { createInstance } from "i18next";
import { initReactI18next } from "react-i18next";
import { browserStorage, detectLanguage, languages, persistLanguage, type Language } from "./locale";
import { commonLocales } from "./common";
import { usageLocales } from "./usage";
import { sessionsLocales } from "./sessions";
import { memoryLocales } from "./memory";

export const resources = Object.fromEntries(languages.map(({ code }) => [code, {
  common: commonLocales[code], usage: usageLocales[code], sessions: sessionsLocales[code], memory: memoryLocales[code],
}]));

export const i18n = createInstance();
void i18n.use(initReactI18next).init({
  resources,
  lng: detectLanguage(browserStorage(), typeof navigator === "undefined" ? [] : navigator.languages),
  fallbackLng: "en",
  supportedLngs: languages.map(language => language.code),
  load: "currentOnly",
  defaultNS: "common",
  initAsync: false,
  interpolation: { escapeValue: false },
  react: { useSuspense: false },
});

const updateDocumentLanguage = () => {
  if (typeof document !== "undefined") document.documentElement.lang = i18n.resolvedLanguage ?? "en";
};
updateDocumentLanguage();
i18n.on("languageChanged", updateDocumentLanguage);

export async function changeLanguage(language: Language) {
  await i18n.changeLanguage(language);
  persistLanguage(browserStorage(), language);
}

export default i18n;
