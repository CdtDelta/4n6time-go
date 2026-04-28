---
task: Fix 4 bugs: search bleed, scroll, examiner notes
slug: 20260428-000000_4-bug-fixes-search-scroll-notes
effort: standard
phase: complete
progress: 16/16
mode: interactive
started: 2026-04-28T00:00:00Z
updated: 2026-04-28T00:00:00Z
---

## Context

4 bugs in 4n6time-go tab system. Bugs 1/2 share the same root cause (activeFilters not per-tab), Bug 3 is a missing scroll-to-top on page nav, Bug 4 is the examiner notes exclusion not covering filter panel predicates.

### Root Causes
- Bug 1/2: `activeFilters` is React global state; `handleTabClick` saves/restores search state but not filter state. Saved query sets `activeFilters` globally, which `buildQueryRequest` uses on every `loadPage` call regardless of active tab.
- Bug 3: `pendingScrollRowRef` is only set > 0 for tab switches with saved scroll row. Page navigation never sets it; the `[events]` effect only scrolls when `row > 0`.
- Bug 4: `excludeNotes` in app.go only checks `req.BaseField`. Filter panel predicates with non-examiner fields (sourcetype, host, filename, etc.) don't trigger the `/* no-notes */` marker.

## Criteria

### Bug 1/2: Saved query filter bleed across tabs
- [ ] ISC-1: `makeTab` includes `savedFilters: null` in its state shape
- [ ] ISC-2: `handleTabClick` saves `activeFilters` into leaving tab's `savedFilters`
- [ ] ISC-3: `handleTabClick` restores `activeFilters` from arriving tab's `savedFilters` via `setActiveFilters`
- [ ] ISC-4: `handleTabClose` restores `activeFilters` from the tab being switched to
- [ ] ISC-5: `handleOpenInNewTab` saves `activeFilters` into leaving tab's `savedFilters`; new tab gets `savedFilters: null`
- [ ] ISC-6: `handleTabClose` saves current `activeFilters` into leaving tab before restoring prev tab

### Bug 3: Page navigation scroll to top
- [ ] ISC-7: `pendingScrollTopRef = useRef(false)` added to App component
- [ ] ISC-8: `handlePrevPage`, `handleNextPage` set `pendingScrollTopRef.current = true`
- [ ] ISC-9: `handleFirstPage`, `handleLastPage` set `pendingScrollTopRef.current = true`
- [ ] ISC-10: `handlePageInputSubmit` sets `pendingScrollTopRef.current = true` before calling `loadPage`
- [ ] ISC-11: `[events]` effect checks `pendingScrollTopRef.current` first; if true, scrolls to row 0 and resets flag
- [ ] ISC-12: Tab switch scroll restore still works (pendingScrollRowRef path unchanged)

### Bug 4: Examiner notes in filter panel views
- [ ] ISC-13: `examinerNoteFields` map in app.go has `sourcetype` removed (per user spec: sourcetype filters should exclude notes)
- [ ] ISC-14: `excludeNotes` also checks `req.Filters` for any field not in the compatible fields map
- [ ] ISC-15: Fields checked: at minimum host, filename, user, format, type, MACB, URL and other empty fields trigger exclusion
- [ ] ISC-16: `excludeNotes` for BaseField still works (existing behavior preserved)

## Decisions

### Bug 1: Per-tab filter state
Add `savedFilters` to tab state. Save current `activeFilters` on tab exit, restore on tab enter. New tabs start with `null`. This mirrors the existing `savedSearch` / `savedPage` pattern.

### Bug 3: Separate ref for scroll-to-top
Add `pendingScrollTopRef` boolean ref. Set it on page nav actions. Check it first in `[events]` effect before the existing `pendingScrollRowRef` check.

### Bug 4: Expand examinerNoteFields check
The existing "compatible fields" map is too broad. Remove `sourcetype` (per user spec). Add a loop over `req.Filters` to check each field against the compatible set.

## Verification
