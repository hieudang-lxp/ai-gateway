import { useEffect, useRef } from "react";
import { observeSectionMotion } from "./section-motion";

export function animatePage(element: Pick<HTMLElement, "animate">, reduced: boolean) {
  if (reduced || !element.animate) return;
  return element.animate([
    { opacity: 0, transform: "translateY(5px)" },
    { opacity: 1, transform: "translateY(0)" },
  ], { duration: 220, easing: "cubic-bezier(.2,.7,.2,1)" });
}

export function usePageMotion(page: string) {
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const preference = window.matchMedia("(prefers-reduced-motion: reduce)");
    const animation = animatePage(element, preference.matches);
    const stopSections = observeSectionMotion(element, preference);
    const stop = () => { if (preference.matches) animation?.cancel(); };
    preference.addEventListener("change", stop);
    return () => { animation?.cancel(); stopSections(); preference.removeEventListener("change", stop); };
  }, [page]);
  return ref;
}
