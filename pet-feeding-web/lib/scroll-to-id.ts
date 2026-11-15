import type { MouseEvent } from "react";

export const LEAD_ROLE_EVENT = "pet-feeding:lead-role";

/** Same-page hash navigation that still works when the hash is already set. */
export function scrollToId(id: string) {
  const el = document.getElementById(id);
  if (!el) return;
  // Prefer scroll-margin on the target; fall back to sticky-header clearance.
  const top = el.getBoundingClientRect().top + window.scrollY;
  const style = window.getComputedStyle(el);
  const marginTop = Number.parseFloat(style.scrollMarginTop) || 80;
  window.scrollTo({ top: Math.max(0, top - marginTop), behavior: "smooth" });
}

export function onHashLinkClick(
  event: MouseEvent<HTMLAnchorElement>,
  id: string,
  options?: { leadRole?: "owner" | "student" },
) {
  // Let modified clicks / new-tab open normally.
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  ) {
    return;
  }
  event.preventDefault();
  const hash = `#${id}`;
  if (window.location.hash !== hash) {
    window.history.pushState(null, "", hash);
  }
  if (options?.leadRole) {
    window.dispatchEvent(
      new CustomEvent(LEAD_ROLE_EVENT, { detail: options.leadRole }),
    );
  }
  scrollToId(id);
}
