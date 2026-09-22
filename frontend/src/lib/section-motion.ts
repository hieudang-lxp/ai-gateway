import { createMotionState } from "./motion-state";

/** CSS owns the visuals. This observer only schedules named sections once per page. */
export function observeSectionMotion(container: HTMLElement, preference: MediaQueryList) {
  const state = createMotionState();
  const watched = new WeakSet<HTMLElement>();
  const pending = new Set<HTMLElement>();
  const animated = new Set<HTMLElement>();
  const timers = new Set<ReturnType<typeof setTimeout>>();
  let frame = 0;

  const play = (element: HTMLElement, className: string, duration: number) => {
    element.classList.add(className);
    animated.add(element);
    const timer = setTimeout(() => {
      element.classList.remove(className);
      animated.delete(element);
      timers.delete(timer);
    }, duration);
    timers.add(timer);
  };
  const reveal = (element: HTMLElement, order: number) => {
    const delay = state.enter(element.dataset.motion!, order, preference.matches);
    if (delay === undefined || !container.contains(element)) return;
    element.style.setProperty("--motion-delay", `${delay}ms`);
    play(element, "motion-enter", 520 + delay);
  };
  const intersection = typeof IntersectionObserver === "undefined" ? undefined : new IntersectionObserver(entries => {
    let order = 0;
    for (const entry of entries) {
      if (!entry.isIntersecting) continue;
      const element = entry.target as HTMLElement;
      intersection?.unobserve(element);
      pending.delete(element);
      reveal(element, order++);
    }
  }, { threshold: 0.08 });

  const scan = () => {
    for (const element of pending) {
      if (!container.contains(element)) { intersection?.unobserve(element); pending.delete(element); }
    }
    container.querySelectorAll<HTMLElement>("[data-motion]").forEach((element, index) => {
      const id = element.dataset.motion!;
      if (!watched.has(element)) {
        watched.add(element);
        if (element.dataset.motionReveal === "false") { /* Update-only region. */ }
        else if (preference.matches || !intersection) reveal(element, index);
        else { pending.add(element); intersection.observe(element); }
      }
      const version = element.dataset.motionUpdate;
      if (version !== undefined && state.update(id, version, preference.matches || document.hidden)) {
        // Do not flash a first entrance or interrupt a pulse already in progress.
        if (!element.classList.contains("motion-enter") && !element.classList.contains("motion-updated")) play(element, "motion-updated", 700);
      }
    });
  };
  const mutation = new MutationObserver(() => {
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(scan);
  });
  mutation.observe(container, { subtree: true, childList: true, attributes: true, attributeFilter: ["data-motion-update"] });
  const stop = () => {
    if (!preference.matches) return;
    for (const element of animated) element.classList.remove("motion-enter", "motion-updated");
    for (const element of pending) { intersection?.unobserve(element); reveal(element, 0); }
    pending.clear();
  };
  preference.addEventListener("change", stop);
  scan();
  return () => {
    mutation.disconnect(); intersection?.disconnect(); cancelAnimationFrame(frame);
    preference.removeEventListener("change", stop);
    timers.forEach(clearTimeout);
    for (const element of animated) {
      element.classList.remove("motion-enter", "motion-updated");
      element.style.removeProperty("--motion-delay");
    }
  };
}
