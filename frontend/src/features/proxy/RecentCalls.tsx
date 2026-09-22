import { useTranslation } from "react-i18next";
import { formatDate, formatNumber } from "@/i18n/format";
import { ProxyQueryStatus } from "./ProxyQueryStatus";
import { Fragment, useState } from "react";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
const compactTokens = (value: bigint) => formatNumber(Number(value), { notation: "compact", maximumFractionDigits: 1 });
import { useCurrency } from "../currency/useCurrency";

export function RecentCalls() {
  const { t } = useTranslation("usage");
  const [beforeId, setBeforeId] = useState(0n);
  const { data, error, isPending } = useQuery(StatsService.method.recentCalls, {
    limit: 50,
    beforeId,
  });
  const { fmt } = useCurrency();
  const calls = data?.calls ?? [];
  if (!data) return <ProxyQueryStatus error={error} pending={isPending} />;
  return (
    <Card data-motion="proxy-recent-calls" className="gap-0 p-6">
      <ProxyQueryStatus error={error} />
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-sky-900">
          {t("recentCalls")}
        </h2>
        <div className="flex gap-2">
          {beforeId !== 0n && (
            <Button variant="outline" size="sm" onClick={() => setBeforeId(0n)}>
              ↩ {t("latest")}
            </Button>
          )}
          {calls.length === 50 && (
            <Button variant="outline" size="sm"
              onClick={() => setBeforeId(calls[calls.length - 1].id)}
            >
              {t("older")} →
            </Button>
          )}
        </div>
      </div>
      <p className="mb-3 text-xs text-slate-500">{t("hidden429")}</p>
      <div className="overflow-x-auto">
        <Table className="w-full text-sm">
          <TableHeader>
            <TableRow className="border-b border-sky-100 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
              <TableHead className="py-2">{t("time")}</TableHead>
              <TableHead>{t("model")}</TableHead>
              <TableHead className="text-right" title={t("inputTooltip")}>
                {t("input")}
              </TableHead>
              <TableHead className="text-right" title={t("outputTooltip")}>
                {t("output")}
              </TableHead>
              <TableHead className="text-right" title={t("readTooltip")}>
                {t("cacheRead")}
              </TableHead>
              <TableHead className="text-right" title={t("writeTooltip")}>
                {t("cacheWrite")}
              </TableHead>
              <TableHead className="text-right">{t("cost")}</TableHead>
              <TableHead className="text-right">{t("latency")}</TableHead>
              <TableHead className="text-right">{t("status")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="tabular-nums">
            {calls.map((c) => (
              <Fragment key={String(c.id)}>
              <TableRow
                className="border-b border-sky-50 transition-colors last:border-0 hover:bg-sky-50/50"
              >
                <TableCell className="py-2.5 whitespace-nowrap text-xs text-slate-500">
                  {formatDate(Number(c.tsUnix) * 1000, { dateStyle: "medium", timeStyle: "short" })}
                </TableCell>
                <TableCell className="font-mono text-xs text-sky-900">
                  {c.model === "unknown" ? t("unidentifiedRequest") : c.model}
                  {c.modelSource === "request" && <span className="mt-1 block font-sans text-xs text-slate-500">{t("requestedUnconfirmed")}</span>}
                  {c.routedFrom && (
                    <Badge variant="secondary" className="ml-1.5 bg-sky-100 text-sky-800">
                      ← {c.routedFrom}
                    </Badge>
                  )}
                  {c.cacheHit && (
                    <Badge variant="secondary" className="ml-1.5 bg-emerald-100 text-emerald-800">
                      {t("cache")}{c.savedUsd > 0 && ` +${fmt(c.savedUsd)}`}
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-right">
                  {formatNumber(Number(c.inputTokens))}
                </TableCell>
                <TableCell className="text-right">
                  {formatNumber(Number(c.outputTokens))}
                </TableCell>
                <TableCell className="text-right text-slate-500">
                  {compactTokens(c.cacheReadTokens)}
                </TableCell>
                <TableCell className="text-right text-slate-500">
                  {compactTokens(c.cacheWriteTokens)}
                </TableCell>
                <TableCell className="text-right font-semibold text-sky-950">
                  {fmt(c.costUsd)}
                </TableCell>
                <TableCell className="text-right text-slate-500">
                  {formatNumber(Number(c.latencyMs))}
                </TableCell>
                <TableCell className="text-right">
                  <Badge variant="secondary"
                    className={`rounded-full px-2 py-0.5 text-[12px] font-semibold ${
                      c.status >= 400
                        ? "bg-red-100 text-red-700"
                        : "bg-sky-50 text-sky-700"
                    }`}
                  >
                    {c.status}
                  </Badge>
                </TableCell>
              </TableRow>
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={9} className="px-2 py-0">
                  <Accordion type="single" collapsible>
                    <AccordionItem value="trace" className="border-0">
                      <AccordionTrigger className="justify-start gap-2 py-2 text-xs text-slate-500">{t("requestTrace", { id: String(c.id) })}</AccordionTrigger>
                      <AccordionContent>
                        {c.requestId ? <dl className="grid gap-x-8 gap-y-3 py-2 sm:grid-cols-2">
                          {[
                            [t("requestedModel"), c.requestModel],
                            [t("modelSource"), t(({ response: "providerResponse", request: "outgoingRequest", cache: "cachedResponse" } as Record<string, string>)[c.modelSource] ?? "notRecorded")],
                            [t("requestPath"), c.requestPath],
                            [t("gatewayRequestId"), c.requestId],
                            [t("providerRequestId"), c.upstreamRequestId || t("notReturned")],
                          ].map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-xs text-slate-500">{label}</dt><dd className="mt-1 whitespace-normal break-all font-mono text-xs text-sky-950">{value}</dd></div>)}
                        </dl> : <p className="whitespace-normal text-sm text-slate-500">{t("noTrace")}</p>}
                      </AccordionContent>
                    </AccordionItem>
                  </Accordion>
                </TableCell>
              </TableRow>
              </Fragment>
            ))}
          </TableBody>
        </Table>
      </div>
      {calls.some(c => c.model === "unknown") && <p className="mt-3 text-xs text-slate-500">{t("unknownHistory")}</p>}
      {calls.length === 0 && (
        <div className="py-6 text-center text-sm text-slate-400">
          {t("noCalls")}
        </div>
      )}
    </Card>
  );
}
