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

function isUnauthenticated(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.Unauthenticated;
}

function TokenForm({ onSubmit }: { onSubmit: (t: string) => void }) {
  const [value, setValue] = useState("");
  return (
    <Card className="mx-auto mt-24 max-w-sm p-6"><form
      className="flex flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(value.trim());
      }}
    >
      <h1 className="text-lg font-semibold text-sky-950">Dashboard token</h1>
      <Label htmlFor="dashboard-token">Bearer token</Label>
      <Input id="dashboard-token"
        type="password"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Bearer token"
        autoFocus
      />
      <Button
        type="submit"
      >
        Save
      </Button>
    </form></Card>
  );
}

export function TokenGate({ children }: { children: ReactNode }) {
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
              <Alert variant="destructive" className="mx-auto my-6 max-w-3xl"><AlertDescription>{String(error)}</AlertDescription></Alert>
            )
          }
        >
          {children}
        </ErrorBoundary>
      )}
    </QueryErrorResetBoundary>
  );
}
