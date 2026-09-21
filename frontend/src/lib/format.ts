
const fmt = new Intl.NumberFormat("en-US", {
  notation: "compact",
  maximumFractionDigits: 2,
});

export function compactTokens(v: bigint | number): string {
  return fmt.format(Number(v));
}
