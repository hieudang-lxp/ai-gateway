import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";
import { getToken } from "./token";

const auth: Interceptor = (next) => (req) => {
  const token = getToken();
  if (token) req.header.set("Authorization", `Bearer ${token}`);
  return next(req);
};

export const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_API_URL ?? "http://localhost:8788/rpc",
  interceptors: [auth],
});
