import { expect, it } from "vitest";
import { pageFromHash } from "./navigation";

it.each([["", "overview"], ["#overview", "overview"], ["#data-pricing", "data-pricing"], ["#unknown", "overview"]] as const)("resolves %s to %s", (hash, page) => {
  expect(pageFromHash(hash)).toBe(page);
});

it.each([["#sessions", "sessions"], ["#sessions?source=codex&session_id=a", "sessions"], ["#insights", "insights"], ["#data-pricing?ignored=yes", "data-pricing"], ["#unknown?sessions=yes", "overview"]] as const)("routes page independently of query: %s", (hash, page) => {
  expect(pageFromHash(hash)).toBe(page);
});
