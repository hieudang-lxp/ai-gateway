import { expect, it, vi } from "vitest";
import { animatePage } from "./motion";

it("does not start motion when reduced motion is requested", () => {
  const animate = vi.fn(() => { throw new Error("motion must not start"); });
  expect(animatePage({ animate }, true)).toBeUndefined();
  expect(animate).not.toHaveBeenCalled();
});
it("returns a cancellable finite animation without replacing the page", () => {
  const animation = { cancel: vi.fn() } as unknown as Animation;
  const animate = vi.fn(() => animation);
  const result = animatePage({ animate }, false);
  expect(result).toBe(animation);
  expect(animate.mock.calls).toHaveLength(1);
});
