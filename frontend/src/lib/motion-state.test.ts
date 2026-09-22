import { expect, it } from "vitest";
import { createMotionState } from "./motion-state";

it("staggers new sections once and caps the delay for long pages", () => {
  const motion = createMotionState();
  expect(motion.enter("totals", 0, false)).toBe(0);
  expect(motion.enter("sources", 1, false)).toBe(55);
  expect(motion.enter("diagnostics", 20, false)).toBe(165);
  expect(motion.enter("totals", 0, false)).toBeUndefined();
});

it("does not replay existing sections after locale remounts or reduced-motion changes", () => {
  const motion = createMotionState();
  expect(motion.enter("total-tokens", 0, true)).toBeUndefined();
  expect(motion.enter("total-tokens", 0, false)).toBeUndefined();
});

it("highlights changed data, not initial loads, unchanged polling, or translated labels", () => {
  const motion = createMotionState();
  expect(motion.update("totals", "12:1000", false)).toBe(false);
  expect(motion.update("totals", "12:1000", false)).toBe(false);
  expect(motion.update("totals", "13:1100", false)).toBe(true);
  expect(motion.enter("totals", 0, false)).toBe(0);
  expect(motion.update("totals", "13:1100", false)).toBe(false);
});

it("remembers updates silently when reduced motion is enabled", () => {
  const motion = createMotionState();
  motion.update("totals", "1", false);
  expect(motion.update("totals", "2", true)).toBe(false);
  expect(motion.update("totals", "2", false)).toBe(false);
  expect(motion.update("totals", "3", false)).toBe(true);
});
