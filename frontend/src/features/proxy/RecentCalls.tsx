import { Fragment, useState } from "react";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { useQuery } from "@connectrpc/connect-query";
import { StatsService } from "../../gen/gateway/v1/stats_pb";
import { compactTokens } from "../../lib/format";
import { useCurrency } from "../currency/useCurrency";

export function RecentCalls() {
  const [beforeId, setBeforeId] = useState(0n);
  const { data } = useQuery(StatsService.method.recentCalls, {
    limit: 50,
    beforeId,
  });
  const { fmt } = useCurrency();
  const calls = data?.calls ?? [];
  return (
    <Card className="gap-0 p-6">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-sky-900">
          Recent calls
        </h2>
        <div className="flex gap-2">
          {beforeId !== 0n && (
            <Button variant="outline" size="sm" onClick={() => setBeforeId(0n)}>
              ↩ Latest
            </Button>
          )}
          {calls.length === 50 && (
            <Button variant="outline" size="sm"
              onClick={() => setBeforeId(calls[calls.length - 1].id)}
            >
              Older →
            </Button>
          )}
        </div>
      </div>
      <p className="mb-3 text-xs text-slate-500">429 responses with no recorded usage are hidden.</p>
      <div className="overflow-x-auto">
        <Table className="w-full text-sm">
          <TableHeader>
            <TableRow className="border-b border-sky-100 text-left text-xs font-medium uppercase tracking-wide text-slate-500">
              <TableHead className="py-2">Time</TableHead>
              <TableHead>Model</TableHead>
              <TableHead className="text-right" title="Input tokens (không tính cache)">
                Input
              </TableHead>
              <TableHead className="text-right" title="Output tokens">
                Output
              </TableHead>
              <TableHead className="text-right" title="Prompt-cache read tokens">
                Cache rd
              </TableHead>
              <TableHead className="text-right" title="Prompt-cache write tokens">
                Cache wr
              </TableHead>
              <TableHead className="text-right">Cost</TableHead>
              <TableHead className="text-right">ms</TableHead>
              <TableHead className="text-right">Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="tabular-nums">
            {calls.map((c) => (
              <Fragment key={String(c.id)}>
              <TableRow
                className="border-b border-sky-50 transition-colors last:border-0 hover:bg-sky-50/50"
              >
                <TableCell className="py-2.5 whitespace-nowrap text-xs text-slate-500">
                  {new Date(Number(c.tsUnix) * 1000).toLocaleString("vi-VN")}
                </TableCell>
                <TableCell className="font-mono text-xs text-sky-900">
                  {c.model === "unknown" ? "Unidentified request" : c.model}
                  {c.modelSource === "request" && <span className="mt-1 block font-sans text-xs text-slate-500">Requested · not confirmed by provider</span>}
                  {c.routedFrom && (
                    <Badge variant="secondary" className="ml-1.5 bg-sky-100 text-sky-800">
                      ← {c.routedFrom}
                    </Badge>
                  )}
                  {c.cacheHit && (
                    <Badge variant="secondary" className="ml-1.5 bg-emerald-100 text-emerald-800">
                      cache{c.savedUsd > 0 && ` +${fmt(c.savedUsd)}`}
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-right">
                  {Number(c.inputTokens).toLocaleString()}
                </TableCell>
                <TableCell className="text-right">
                  {Number(c.outputTokens).toLocaleString()}
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
                  {Number(c.latencyMs)}
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
                      <AccordionTrigger className="justify-start gap-2 py-2 text-xs text-slate-500">Request trace #{String(c.id)}</AccordionTrigger>
                      <AccordionContent>
                        {c.requestId ? <dl className="grid gap-x-8 gap-y-3 py-2 sm:grid-cols-2">
                          {[
                            ["Requested model", c.requestModel],
                            ["Model source", ({ response: "Provider response", request: "Outgoing request (provider did not confirm)", cache: "Cached response" } as Record<string, string>)[c.modelSource] ?? "Not recorded"],
                            ["Request path", c.requestPath],
                            ["Gateway request ID", c.requestId],
                            ["Provider request ID", c.upstreamRequestId || "Not returned"],
                          ].map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-xs text-slate-500">{label}</dt><dd className="mt-1 whitespace-normal break-all font-mono text-xs text-sky-950">{value}</dd></div>)}
                        </dl> : <p className="whitespace-normal text-sm text-slate-500">This older record has no request trace. The original request model, path and request IDs were not stored.</p>}
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
      {calls.some(c => c.model === "unknown") && <p className="mt-3 text-xs text-slate-500">Some older requests have no recorded model or URL. Their original HTTP status is preserved; a 200 status with zero tokens does not establish that a model was called. Missing historical model names cannot be reconstructed from these records.</p>}
      {calls.length === 0 && (
        <div className="py-6 text-center text-sm text-slate-400">
          No calls yet
        </div>
      )}
    </Card>
  );
}
