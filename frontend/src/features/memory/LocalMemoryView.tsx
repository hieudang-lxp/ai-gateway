import { Check, CircleHelp, RefreshCw, TriangleAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { formatDate, formatNumber } from "@/i18n/format";
import { memoryHealth, type MemoryStatus } from "./status";

type Props = { data?: MemoryStatus; isError: boolean; isFetching: boolean; now: number; onRefresh: () => void };
const sources = [["claude_code", "Claude Code"], ["codex", "Codex"], ["cursor", "Cursor"]] as const;
const tones = { good: "text-emerald-700", warning: "text-amber-800", neutral: "text-muted-foreground" };
const summaryClass = "cursor-pointer py-3 text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring";

export function LocalMemoryView({ data, isError, isFetching, now, onRefresh }: Props) {
  const { t, i18n } = useTranslation("memory");
  const health = memoryHealth(data, isError, now);
  const lastKnown = isError || health.stale;
  const queue = data?.receiver?.queue;
  const number = (value: number | undefined) => value === undefined
    ? <span aria-label={t("unknown")}>—</span> : formatNumber(value, undefined, i18n.language);
  const timestamp = (value?: string | null) => value && Number.isFinite(Date.parse(value))
    ? formatDate(value, { dateStyle: "medium", timeStyle: "short" }, i18n.language) : t("notReported");
  const healthDescription = t(health.descriptionKey, health.params);
  const diagnostics = [
    ...(data?.error ? [{ name: t("serviceDetail"), error: data.error }] : []),
    ...sources.flatMap(([id, name]) => data?.sources[id]?.error
      ? [{ name: t("sourceDetail", { source: name }), error: data.sources[id].error! }] : []),
  ];
  const metrics = (items: Array<[string, number | undefined]>) => <dl className="grid grid-cols-2 gap-x-6 gap-y-4 sm:flex sm:flex-wrap">
    {items.map(([label, value]) => <div key={label} className="min-w-0 rounded-xl bg-sky-50/60 p-4 sm:min-w-28"><dt className="text-sm text-muted-foreground">{label}</dt><dd className="mt-1 text-xl font-semibold tabular-nums text-sky-950">{number(value)}</dd></div>)}
  </dl>;

  return <div className="min-w-0 space-y-6">
    <header className="flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0"><h1 className="text-2xl font-semibold tracking-tight text-sky-950 sm:text-3xl">{t("title")}</h1>
        <div className={`mt-2 flex items-start gap-2 text-sm ${tones[health.tone]}`} role={health.tone === "warning" ? "alert" : "status"}>
          {health.tone === "warning" && <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />}
          <span>{t(health.titleKey)}</span>
        </div>
        {data?.enabled && <p className="mt-1 text-sm text-muted-foreground">{t(lastKnown ? "lastKnown" : "checkedAt")}: {timestamp(data.last_check)}</p>}
      </div>
      <Button variant="outline" onClick={onRefresh} disabled={isFetching} aria-label={t(isFetching ? "refreshingAria" : "refreshAria")}>
        <RefreshCw aria-hidden="true" className={isFetching ? "motion-safe:animate-spin" : ""} />{t(isFetching ? "refreshing" : "refresh")}
      </Button>
    </header>

    {health.tone === "warning" && <p className="border-l-2 border-amber-500 pl-3 text-sm text-amber-800">{healthDescription}</p>}
    {!data && <p className="text-sm text-muted-foreground">{t(isError ? "fetchError" : "loading")}</p>}
    {data && !data.enabled && <section className="space-y-2 rounded-xl border border-sky-100 bg-white p-5 shadow-sm sm:p-6"><h2 className="font-semibold">{t("configure")}</h2><p className="text-sm text-muted-foreground">{t("configureDescription")}</p><p className="text-sm text-muted-foreground">{t("disabledPrivacy")}</p></section>}

    {data?.enabled && <>
      <section data-motion="memory-dependencies" aria-labelledby="dependencies-title" className="rounded-xl border border-sky-100 bg-white p-5 shadow-sm sm:p-6">
        <h2 id="dependencies-title" className="text-sm font-semibold text-sky-950">{t("dependencies")} <span className="font-normal text-muted-foreground">· {t(lastKnown ? "lastReported" : "readiness")}</span></h2>
        <dl className="mt-3 grid gap-x-8 gap-y-2 sm:grid-cols-3">{(["llm", "embedder", "neo4j"] as const).map(id => {
          const ready = data.receiver?.dependencies?.[id];
          const Icon = ready === undefined ? CircleHelp : ready ? Check : TriangleAlert;
          return <div key={id} className="flex min-w-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm"><dt>{id === "neo4j" ? "Neo4j" : t(id)}</dt><dd className={`flex items-center gap-1.5 ${ready === undefined || lastKnown ? tones.neutral : ready ? tones.good : tones.warning}`}><Icon aria-hidden="true" className="size-4 shrink-0" />{t(ready === undefined ? "unknown" : ready ? "ready" : "notReady")}</dd></div>;
        })}</dl>
      </section>

      <section data-motion="memory-delivery" data-motion-update={`${data.delivered}:${data.pending}:${data.blocked}`} aria-labelledby="delivery-title" className="space-y-4 rounded-xl border border-sky-100 bg-white p-5 shadow-sm sm:p-6">
        <h2 id="delivery-title" className="text-base font-semibold text-sky-950">{t("delivery")}</h2>
        {metrics([[t("accepted"), data.delivered], [t("waiting"), data.pending], [t("blocked"), data.blocked]])}
        <p className="text-sm text-muted-foreground">{t("deliveryMeaning")}</p>
        <table className="block w-full text-left text-sm lg:table">
          <thead className="hidden border-b text-muted-foreground lg:table-header-group"><tr>{["source", "sourceState", "sourceAccepted", "sourceWaiting", "sourceBlocked", "scanTime"].map(key => <th key={key} scope="col" className="px-2 py-2 font-medium first:pl-0 last:pr-0">{t(key)}</th>)}</tr></thead>
          <tbody className="block divide-y lg:table-row-group">{sources.map(([id, name]) => {
            const source = data.sources[id];
            const scanned = Date.parse(source?.last_success ?? "");
            const scanStale = source?.state === "ok" && (!Number.isFinite(scanned) || now - scanned > 120_000);
            const ok = source?.state === "ok" && !source.error && !source.blocked && !scanStale;
            const label = !source ? "sourceMissing" : scanStale ? "sourceStale" : ok ? (lastKnown ? "sourcePreviously" : "sourceChecked") : source.state === "waiting" ? "sourceWaiting" : "sourceAttention";
            return <tr key={id} className="grid grid-cols-3 gap-x-3 gap-y-2 py-3 lg:table-row">
              <th scope="row" className="col-span-3 font-medium lg:py-3 lg:pr-2">{name}</th>
              <td className={`col-span-3 lg:px-2 lg:py-3 ${!source || lastKnown ? tones.neutral : ok ? tones.good : tones.warning}`}><span className="sr-only lg:hidden">{t("sourceState")}: </span>{t(label)}</td>
              {([["sourceAccepted", source?.delivered], ["sourceWaiting", source?.pending], ["sourceBlocked", source?.blocked]] as const).map(([key, value]) => <td key={key} className="min-w-0 tabular-nums lg:px-2 lg:py-3"><span className="mb-1 block break-words text-muted-foreground lg:hidden">{t(key)}</span>{number(value)}</td>)}
              <td className="col-span-3 text-muted-foreground lg:py-3 lg:pl-2"><span className="lg:hidden">{t("scanTime")}: </span>{timestamp(source?.last_success)}</td>
            </tr>;
          })}</tbody>
        </table>
      </section>

      <section data-motion="memory-extraction" data-motion-update={queue ? `${queue.pending}:${queue.processing}:${queue.succeeded}:${queue.failed}` : undefined} aria-labelledby="extraction-title" className="space-y-4 rounded-xl border border-sky-100 bg-white p-5 shadow-sm sm:p-6">
        <h2 id="extraction-title" className="text-base font-semibold text-sky-950">{t("extraction")}</h2>
        {queue ? <>
          {metrics([[t("extractionWaiting"), queue.pending], [t("processing"), queue.processing], [t("completed"), queue.succeeded], [t("failed"), queue.failed]])}
          {!!queue.failed && <p role="alert" className="border-l-2 border-amber-500 pl-3 text-sm text-amber-800">{t("failedAlert", { count: queue.failed })}</p>}
          {!!queue.legacy_uncertain && <p role="alert" className="border-l-2 border-amber-500 pl-3 text-sm text-amber-800">{t("legacyAlert", { count: queue.legacy_uncertain })}</p>}
          <details className="border-y"><summary className={summaryClass}>{t("queueDetails")}</summary><div className="space-y-3 pb-4">{metrics([[t("excluded"), queue.excluded], [t("legacy"), queue.legacy_uncertain]])}<p className="text-sm text-muted-foreground">{t("excludedMeaning")}</p><p className="text-sm text-muted-foreground">{t("queueMeaning")}</p><p className="text-sm text-muted-foreground">{t("workerNote")}</p></div></details>
        </> : <div className="space-y-1"><p className="text-sm font-medium">{t("extractionUnknown")}</p><p className="text-sm text-muted-foreground">{t("extractionUnknownDescription")}</p></div>}
      </section>
    </>}

    {!!diagnostics.length && <details className="border-y"><summary className={summaryClass}>{t("technicalDetails")}</summary><dl className="space-y-3 pb-4 text-sm">{diagnostics.map(({ name, error }) => <div key={name}><dt className="font-medium">{name}</dt><dd className="mt-1 whitespace-pre-wrap break-words text-muted-foreground [overflow-wrap:anywhere]">{error}</dd></div>)}</dl></details>}
    <details className="border-t"><summary className={summaryClass}>{t("details")}</summary><div className="space-y-2 pb-2 text-sm text-muted-foreground">{health.tone !== "warning" && <p>{healthDescription}</p>}<p>{t("footer")}</p></div></details>
  </div>;
}
