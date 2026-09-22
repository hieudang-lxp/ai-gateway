import { useQuery } from "@tanstack/react-query";
import { apiBaseURL } from "@/lib/transport";
import { useSyncClock } from "../usage/syncStatus";
import { parseMemoryStatus } from "./status";
import { LocalMemoryView } from "./LocalMemoryView";

export function LocalMemory() {
  const now = useSyncClock();
  const query = useQuery({
    queryKey: ["memory-status"],
    queryFn: async ({ signal }) => {
      const response = await fetch(new URL("/_memory", apiBaseURL), { cache: "no-store", signal });
      if (!response.ok) throw new Error("memory_status_unavailable", { cause: { status: response.status } });
      return parseMemoryStatus(await response.json());
    },
    refetchInterval: 30_000,
    retry: 1,
    throwOnError: false,
  });
  return <LocalMemoryView data={query.data} isError={query.isError} isFetching={query.isFetching} now={now} onRefresh={() => { void query.refetch(); }} />;
}
