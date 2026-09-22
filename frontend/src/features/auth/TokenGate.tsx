import { useState, type ReactNode } from "react";
import { QueryErrorResetBoundary } from "@tanstack/react-query";
import { ErrorBoundary } from "react-error-boundary";
import { ConnectError, Code } from "@connectrpc/connect";
import { clearToken, setToken } from "./token";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { useTranslation } from "react-i18next";
import { LanguageSelect } from "@/components/LanguageSelect";

function isUnauthenticated(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.Unauthenticated;
}

function TokenForm({ onSubmit }: { onSubmit: (t: string) => void }) {
  const { t } = useTranslation("common");
  const [value, setValue] = useState("");
  return (
    <Card className="mx-auto mt-24 max-w-sm p-6"><form
      className="flex flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(value.trim());
      }}
    >
      <LanguageSelect />
    <h1 className="text-lg font-semibold text-sky-950">{t("dashboardToken")}</h1>
      <Label htmlFor="dashboard-token">{t("bearerToken")}</Label>
      <Input id="dashboard-token"
        type="password"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder={t("bearerToken")}
        autoFocus
      />
      <Button
        type="submit"
      >
        {t("save")}
      </Button>
    </form></Card>
  );
}

export function TokenGate({ children }: { children: ReactNode }) {
  const { t } = useTranslation("common");
  return (
    <QueryErrorResetBoundary>
      {({ reset }) => (
        <ErrorBoundary
          onError={(err) => {
            if (isUnauthenticated(err)) clearToken();
          }}
          fallbackRender={({ error, resetErrorBoundary }) =>
            isUnauthenticated(error) ? (
              <TokenForm
                onSubmit={(t) => {
                  setToken(t);
                  reset();
                  resetErrorBoundary();
                }}
              />
            ) : (
              <Alert variant="destructive" className="mx-auto my-6 max-w-3xl"><AlertDescription className="space-y-3"><LanguageSelect /><p>{t("unexpectedError")}</p><details><summary>{t("errorDetails")}</summary><p className="break-words">{String(error)}</p></details><Button variant="outline" onClick={() => { reset(); resetErrorBoundary(); }}>{t("retry")}</Button></AlertDescription></Alert>
            )
          }
        >
          {children}
        </ErrorBoundary>
      )}
    </QueryErrorResetBoundary>
  );
}
