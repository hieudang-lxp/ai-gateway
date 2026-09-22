import { useTranslation } from "react-i18next";

export function ProxyQueryStatus({ error, pending }: { error?: Error | null; pending?: boolean }) {
  const { t } = useTranslation("usage");
  if (error) return <div role="alert" className="p-4 text-sm text-amber-900"><p>{t("proxyError")}</p><details className="mt-2"><summary>{t("details")}</summary><p className="break-words">{error.message}</p></details></div>;
  if (pending) return <p role="status" className="motion-loading p-4 text-sm text-slate-500">{t("proxyLoading")}</p>;
  return null;
}
