import { formatNumber } from "@/i18n/format";

export function compactTokens(v: bigint | number): string {
  return formatNumber(Number(v), { notation: "compact", maximumFractionDigits: 2 });
}
