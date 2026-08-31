import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TransportProvider } from "@connectrpc/connect-query";
import { transport } from "./lib/transport";
import { TokenGate } from "./components/TokenGate";
import App from "./App";
import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { refetchInterval: 30_000, throwOnError: true, retry: false },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <TokenGate>
          <App />
        </TokenGate>
      </QueryClientProvider>
    </TransportProvider>
  </StrictMode>,
);
