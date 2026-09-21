import { useSyncExternalStore } from "react";

export const pages = [
  { id: "overview", label: "Overview" },
  { id: "sessions", label: "Sessions" },
  { id: "insights", label: "Insights" },
  { id: "data-pricing", label: "Data & Pricing" },
] as const;
export type Page = typeof pages[number]["id"];
export function pageFromHash(hash: string): Page {
  const id = hash.split("?")[0].replace(/^#/, "");
  return pages.find(page => page.id === id)?.id ?? "overview";
}
const subscribe = (notify: () => void) => {
  window.addEventListener("hashchange", notify);
  return () => window.removeEventListener("hashchange", notify);
};
const snapshot = () => pageFromHash(window.location.hash);
export function usePage() {
  return useSyncExternalStore(subscribe, snapshot, () => "overview" as Page);
}

export function useHash() {
  return useSyncExternalStore(subscribe, () => window.location.hash, () => "");
}
