import i18n from "./index";

export function formatNumber(value: number, options?: Intl.NumberFormatOptions, locale = i18n.resolvedLanguage ?? "en") {
  return new Intl.NumberFormat(locale, options).format(value);
}

export function formatDate(value: Date | string | number, options?: Intl.DateTimeFormatOptions, locale = i18n.resolvedLanguage ?? "en") {
  const date = value instanceof Date ? value : new Date(value);
  if (!Number.isFinite(date.getTime())) return "—";
  return new Intl.DateTimeFormat(locale, options ?? { dateStyle: "medium", timeStyle: "short" }).format(date);
}
