import { expect, it } from "vitest";
import { formatDate, formatNumber } from "./format";
import i18n from "./index";

it("formats numbers in the explicit locale", () => {
  expect(formatNumber(1234.5, {}, "de")).toBe("1.234,5");
  expect(formatNumber(1234.5, {}, "en")).toBe("1,234.5");
});
it("formats dates without changing the supplied timezone", () => {
  expect(formatDate("2026-09-22T20:00:00Z", { year: "numeric", month: "2-digit", day: "2-digit", timeZone: "Asia/Ho_Chi_Minh" }, "de")).toBe("23.09.2026");
});
it("does not freeze number format when language changes", async () => {
  await i18n.changeLanguage("vi");
  expect(formatNumber(1234.5)).toBe("1.234,5");
  await i18n.changeLanguage("en");
  expect(formatNumber(1234.5)).toBe("1,234.5");
});
