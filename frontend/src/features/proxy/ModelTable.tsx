import { formatNumber } from "@/i18n/format";
import { useTranslation } from "react-i18next";
import { ProxyQueryStatus } from "./ProxyQueryStatus";
import { useQuery } from "@connectrpc/connect-query";
import { Card } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import { useCurrency } from "../currency/useCurrency";

const n = (v: bigint) => formatNumber(Number(v));

export function ModelTable() {
  const { t } = useTranslation("usage");
  const { data, error, isPending } = useQuery(StatsService.method.modelBreakdown, { days: 30 });
  const { fmt } = useCurrency();
  const rows = data?.rows ?? [];
  if (!data) return <ProxyQueryStatus error={error} pending={isPending} />;
  return (
    <Card data-motion="proxy-models" className="gap-0 p-6">
      <ProxyQueryStatus error={error} />
      <h2 className="mb-4 text-sm font-semibold uppercase tracking-wide text-sky-900">
        {t("models30")}
      </h2>
      <div className="overflow-x-auto">
        <Table className="w-full text-sm">
          <TableHeader>
            <TableRow className="border-b border-sky-100 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
              <TableHead className="py-2">{t("model")}</TableHead>
              <TableHead className="text-right">{t("calls")}</TableHead>
              <TableHead className="text-right">{t("input")}</TableHead>
              <TableHead className="text-right">{t("output")}</TableHead>
              <TableHead className="text-right">{t("cacheRead")}</TableHead>
              <TableHead className="text-right">{t("cacheWrite")}</TableHead>
              <TableHead className="text-right">{t("cost")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="tabular-nums">
            {rows.map((r) => (
              <TableRow
                key={r.model}
                className="border-b border-sky-50 transition-colors last:border-0 hover:bg-sky-50/50"
              >
                <TableCell className="py-2.5 font-mono text-xs text-sky-900">
                  {r.model}
                </TableCell>
                <TableCell className="text-right">{n(r.calls)}</TableCell>
                <TableCell className="text-right">{n(r.inputTokens)}</TableCell>
                <TableCell className="text-right">{n(r.outputTokens)}</TableCell>
                <TableCell className="text-right">{n(r.cacheReadTokens)}</TableCell>
                <TableCell className="text-right">{n(r.cacheWriteTokens)}</TableCell>
                <TableCell className="text-right font-semibold text-sky-950">
                  {fmt(r.costUsd)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {rows.length === 0 && (
        <div className="py-6 text-center text-sm text-slate-400">
          {t("noCalls")}
        </div>
      )}
    </Card>
  );
}
