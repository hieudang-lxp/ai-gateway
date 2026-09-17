import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";
import { getToken } from "../features/auth/token";

const auth: Interceptor = (next) => (req) => {
  const token = getToken();
  if (token) req.header.set("Authorization", `Bearer ${token}`);
  return next(req);
};

export const apiBaseURL = import.meta.env.VITE_API_URL ?? (
  window.location.pathname.startsWith("/dashboard") ? `${window.location.origin}/rpc` : "http://localhost:8788/rpc"
);

export const transport = createConnectTransport({
  baseUrl: apiBaseURL,
  interceptors: [auth],
});
