import { afterEach, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18n from "@/i18n";
import { LanguageSelect } from "./LanguageSelect";

afterEach(async () => { await i18n.changeLanguage("en"); });

it.each([
  ["en", "Language", "English"],
  ["vi", "Ngôn ngữ", "Tiếng Việt"],
  ["ko", "언어", "한국어"],
  ["zh-Hans", "语言", "简体中文"],
  ["de", "Sprache", "Deutsch"],
])("exposes the %s selection in a labelled popup trigger", async (locale, label, selection) => {
  await i18n.changeLanguage(locale);
  const html = renderToStaticMarkup(<LanguageSelect />);
  const trigger = html.match(/<button\b[^>]*role="combobox"[^>]*>[\s\S]*?<\/button>/)?.[0];
  expect(trigger).toBeDefined();
  expect(trigger).toContain(`aria-label="${label}"`);
  expect(trigger).toContain('aria-expanded="false"');
  expect(trigger).toContain(selection);
});
