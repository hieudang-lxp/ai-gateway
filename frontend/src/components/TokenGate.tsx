import { useState, type ReactNode } from "react";
import { QueryErrorResetBoundary } from "@tanstack/react-query";
import { ErrorBoundary } from "react-error-boundary";
import { ConnectError, Code } from "@connectrpc/connect";
import { clearToken, setToken } from "../lib/token";

function isUnauthenticated(err: unknown): boolean {
  return err instanceof ConnectError && err.code === Code.Unauthenticated;
}

function TokenForm({ onSubmit }: { onSubmit: (t: string) => void }) {
  const [value, setValue] = useState("");
  return (
    <form
      className="mx-auto mt-24 flex max-w-sm flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(value.trim());
      }}
    >
      <h1 className="text-lg font-semibold">Dashboard token</h1>
      <input
        className="rounded border border-gray-300 px-3 py-2"
        type="password"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Bearer token"
        autoFocus
      />
      <button className="rounded bg-gray-900 py-2 text-white" type="submit">
        Save
      </button>
    </form>
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
              <div className="m-6 rounded bg-red-50 p-4 text-red-700">
                {String(error)}
              </div>
            )
          }
        >
          {children}
        </ErrorBoundary>
      )}
    </QueryErrorResetBoundary>
  );
}
