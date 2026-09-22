# Dashboard i18n and motion implementation plan

> **For agentic workers:** Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task; independent page migrations may run in parallel under explicit ownership.

**Goal:** Ship the approved compact dashboard, five complete UI languages and restrained motion on the existing local deployment.

**User correction (2026-09-22, takes precedence):** Restore the original sky-blue
visual design, typography, spacing, radius, card structure and shadows. Keep i18n,
motion and functional fixes. Do not reapply the neutral redesign described below.

**Architecture:** A synchronous bundled i18next instance provides namespaces for common shell, usage/proxy, sessions/insights and memory. UI helpers format with the selected locale; status logic retains language-independent codes. Existing React state, query keys, API semantics and microservices remain unchanged.

**Tech Stack:** React 19, TypeScript, i18next/react-i18next, Intl, Tailwind/CSS, existing Radix primitives, Vitest.

**Spec:** `docs/superpowers/specs/2026-09-22-dashboard-i18n-motion-design.md`

## Global constraints

- Resources: en, vi, ko, zh-Hans, de; no external translation service.
- Preserve unknown/stale/error states, currency selection, timezone semantics, filters, deep links and user-generated text.
- Animation must respect reduced motion, keep controls usable immediately, and not replay on polling.
- Preserve the current dirty backend and memory implementation. Deploy gateway only.

## Review focus

1. Cached errors/status update language without a fetch; test by translating the same state under two languages.
2. Partial resources cannot silently pass by falling back to English; compare keys/placeholders across all dictionaries.
3. Traditional Chinese must not silently select Simplified; test zh-TW/zh-Hant and unsupported languages.
4. Blocked storage must not crash language selection; test throwing storage.
5. Long German labels and CJK text must not overflow; inspect 320/390/640/768px and desktop in browser.

## Task 1: Shared i18n and locale contracts (root)

Files: `frontend/src/i18n/{index,locale,format,common}.ts`, tests, `main.tsx`, `package*.json`, currency and auth modules.

Interfaces: `useTranslation(namespace)` from react-i18next; namespace resources `common`, `usage`, `sessions`, `memory`. Each domain exports `<domain>Locales` from `frontend/src/i18n/<domain>.ts`, keyed by supported language, each value a flat string dictionary. All language dictionaries must satisfy the English key set. `formatNumber(value:number, options?:Intl.NumberFormatOptions, locale?:string):string` and `formatDate(value:Date|string|number, options?:Intl.DateTimeFormatOptions, locale?:string):string` use selected locale by default. Numeric dates are milliseconds. `useLocale():string` subscribes to language updates. `ApiError` carries status and kind, never a cached translated message.

- [ ] Add failing tests: `expect(resolveLocale('ko-KR')).toBe('ko')`, `expect(resolveLocale('zh-TW')).toBe('en')`, blocked storage fallback, German number formatting and resource key/interpolation parity.
- [ ] Run `npm test -- src/i18n` and inspect failures.
- [ ] Install i18next/react-i18next, implement locale resolution, safe persistence, bundled resources, formatters and provider initialization. Example usage: `t('accepted', { count: n })`, `formatNumber(n)`, `formatDate(new Date(unix * 1000))`.
- [ ] Wire language switching to html lang/document title; localize currency/auth without changing exchange rate behavior. Run targeted tests and typecheck.

## Task 2: Compact shell and motion (root)

Files: `App.tsx`, `components/{Header,LanguageSelect}.tsx`, `index.css`, `lib/navigation.ts`, shared UI primitives where necessary.

- [ ] Test translated page titles and locale selection; preserve route resolution tests.
- [ ] Replace slogans/gradient with compact titles and neutral tokens; expose native-name language picker.
- [ ] Animate main element through a ref on page change, not a keyed remount: `element.animate([{opacity:0,transform:'translateY(5px)'},{opacity:1,transform:'none'}],{duration:220})`, guarded by reduced motion. Animate nav indicator using measured active bounds and ResizeObserver. Never remount solely for animation.
- [ ] Add short CSS interaction/disclosure transitions with reduced-motion override. Test navigation/filter retention and resizing in browser.

## Task 3: Usage, pricing and proxy (usage owner)

Files: all `features/usage/*`, `features/proxy/*`, `i18n/usage.ts`; shared shell/currency owned by root.

- [ ] Add a behavior test for cached sync status rendered in two languages and preserve existing usage calculations.
- [ ] Translate every UI string, tooltip, chart label, error/empty/loading state and formatter. Use stable status codes or keys for data; no localized query keys.
- [ ] Provide all five resource dictionaries with identical interpolation contracts; slim titles/cards and move secondary explanations into accessible details.
- [ ] Run existing and new tests; verify unknown prices and source errors retain honest semantics.

## Task 4: Sessions and insights (sessions owner)

Files: `features/sessions/*`, `features/insights/*`, `i18n/sessions.ts`.

- [ ] Test localized generated session fallback title and comparison text while real titles/model IDs remain unchanged.
- [ ] Localize filters, charts, network/canvas labels, errors, aria labels, date/number helpers and metadata. Keep filters, deep links and query identities independent of locale.
- [ ] Simplify headings/layout while preserving charts; honor reduced motion in the network animation loop.
- [ ] Run sessions/insights tests; test German labels and CJK in browser during integration.

## Task 5: Memory (memory owner)

Files: `features/memory/*`, `i18n/memory.ts`.

- [ ] Add failing localized SSR/health tests for delivery versus extraction and errors/unknown/stale.
- [ ] Return health translation key/params rather than English text. Rebuild view into compact readiness + delivery table + extraction summary with accessible technical details.
- [ ] Translate all five dictionaries. Keep failures visible even when another state takes headline precedence. Localize known failures; unknown service diagnostics stay in details.
- [ ] Run full memory suite; verify zero/unknown distinctions and metadata-only display.

## Task 6: Integration, review and deployment (root)

- [ ] Scan UI strings and locale hardcodes across TSX/TS; inspect exceptions manually, including UI primitives and canvas text.
- [ ] Run `npm test`, `npm run build`, `npm run lint`, `git diff --check`; resolve cross-task type/resource mismatches.
- [ ] Fresh independent review of the frontend diff against the spec; fix concrete issues with regression tests.
- [ ] Browser-check each language, refresh/navigation/filter retention, mobile overflow, errors and reduced motion; keep final local dashboard visible.
- [ ] `docker compose build gateway` then `docker compose up -d --no-deps gateway`; verify deployed UI and health without restarting workers.

## Execution record

- User approved spec and explicitly requested immediate implementation; proceed without another approval round.
- Work in current checkout on a feature branch to preserve and exercise the existing uncommitted memory feature; no separate worktree copy or backend edits.
- Independent page owners may work concurrently after the shared interfaces above are communicated. Root owns shared infrastructure and final integration.
- Product changes stay uncommitted for review; do not mix existing backend changes into frontend commits.

### Implementation and verification — 2026-09-22

- Tasks 1–5 implemented in parallel by the shared-shell, usage/proxy,
  sessions/insights and memory owners. Resources are bundled and checked across
  all five languages; no translation service receives user data.
- Independent review found and corrected Traditional Chinese browser fallback,
  network camera reset on language/resize, and blocked token-storage handling.
  Regression cases were observed failing before the corresponding fixes.
- Fresh frontend verification: **156 tests in 22 files passed**, TypeScript/Vite
  build passed, lint and `git diff --check` passed. Vite still warns about chunks
  over 500 kB; the network renderer remains a separate lazy-loaded chunk.
- Browser checks covered all five languages, cached label changes, preserved
  session search draft/applied filter, German/CJK labels and 320/390/640/768px
  widths. Overview, sessions, memory, pricing and insights had no page-level
  horizontal overflow in the checked layouts. Reduced-motion behavior is covered
  by the motion tests, CSS override and network preference guard; no OS preference
  was changed for testing. Translations have not been reviewed by native speakers.
- Existing Graphiti failed jobs are backend status, not hidden or rewritten by
  this UI change. Gateway-only deployment verification follows below.
- Docker gateway build and `up -d --no-deps gateway` succeeded. The live dashboard
  at `http://localhost:8788/dashboard/#sessions` displays Vietnamese after reload,
  with no captured browser warnings/errors. Memory-sync retained its two-hour
  uptime. The temporary Vite server was stopped and viewport override reset.
- Browser keyboard check: Enter opens source coverage with visible focus.
  After orbiting the network and letting damping settle, the Codex label retained
  identical projected coordinates across EN → VI (362.967px, 322.156px), confirming
  that localization did not refit the camera.

### Visual correction after user feedback

- Restored original sky-blue theme tokens, typography scale, radius, shadow and
  page spacing from HEAD; Card primitive is identical to the pre-redesign version.
- Restored icon navigation with an animated blue underline, dark-blue currency
  selection, logo tile, usage/pricing/session/insight cards and network background.
  Mobile navigation scrolls horizontally as in the original design.
- Kept five-language resources, locale/currency behavior, motion/reduced-motion,
  cache/error handling and camera fixes. Local Memory uses matching sky cards.
- Fresh verification: 156 tests passed, build/lint/diff-check passed. Checked
  Sessions/Overview in-browser and German mobile at 320px without page overflow.
  Docker gateway rebuild passed; no backend source or worker changes were made.

### Approved motion enhancement and dropdown correction

- Replaced the native language select with the existing shadcn/Radix Select and
  Tailwind styling; five-language choice/persistence remains unchanged.
- Added opt-in staggered section entrances, leaf-card hover lift, button press
  spring transitions, elastic tab underline, menu/accordion timing, pending-state
  shimmer and subtle raw-data-change pulses using `tw-animate-css` and CSS.
- Original theme tokens, page padding, radius and layout are unchanged. Stable
  section IDs and raw signatures prevent locale/unchanged-poll replays; observer
  cleanup and reduced-motion handling were independently reviewed.
- Browser verified Home/Enter selection, Escape/focus restoration, mobile popup
  fit and search draft retention across language changes (with no entry/pulse
  replay on the locale switch).
- Fresh verification: 168 tests across 25 files passed, TypeScript/Vite build,
  lint and diff-check passed. Docker gateway build passed; the existing large
  bundle warning remains. No extra animation library was installed.
