// Compact token counts: 1439668 -> "1.44M", 86231 -> "86.2K", 604 -> "604"
const fmt = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 2,
});

export function compactTokens(v: bigint | number): string {
  return fmt.format(Number(v));
}
