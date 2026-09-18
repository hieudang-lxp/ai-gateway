import { expect, it } from "vitest";
import { pageFromHash } from "./navigation";

it.each([["", "overview"], ["#overview", "overview"], ["#data-pricing", "data-pricing"], ["#unknown", "overview"]] as const)("resolves %s to %s", (hash, page) => {
  expect(pageFromHash(hash)).toBe(page);
});
