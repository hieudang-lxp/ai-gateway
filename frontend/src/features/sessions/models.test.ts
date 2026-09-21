import { expect, it } from "vitest";
import { modelForSource, modelsForSource } from "./models";

const options = [
  { source: "codex", model: "gpt-model" },
  { source: "cursor", model: "gpt-model" },
  { source: "claude_code", model: "claude-model" },
];

it("shows unique observed models for all sources and only matching models for a source", () => {
  expect(modelsForSource(options, "all")).toEqual(["claude-model", "gpt-model"]);
  expect(modelsForSource(options, "codex")).toEqual(["gpt-model"]);
  expect(modelsForSource([], "codex")).toEqual([]);
});

it("retains shared models but resets incompatible selections when source changes", () => {
  expect(modelForSource(options, "cursor", "gpt-model")).toBe("gpt-model");
  expect(modelForSource(options, "claude_code", "gpt-model")).toBe("");
  expect(modelForSource(options, "all", "claude-model")).toBe("claude-model");
});
