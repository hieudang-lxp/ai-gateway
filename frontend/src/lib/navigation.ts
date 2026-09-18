import { useSyncExternalStore } from "react";

export const pages = [
  { id: "overview", label: "Overview" },
  { id: "data-pricing", label: "Data & Pricing" },
] as const;
export type Page = typeof pages[number]["id"];
export function pageFromHash(hash: string): Page {
  return hash === "#data-pricing" ? "data-pricing" : "overview";
}
const subscribe = (notify: () => void) => {
  window.addEventListener("hashchange", notify);
  return () => window.removeEventListener("hashchange", notify);
};
const snapshot = () => pageFromHash(window.location.hash);
export function usePage() {
  return useSyncExternalStore(subscribe, snapshot, () => "overview" as Page);
}
