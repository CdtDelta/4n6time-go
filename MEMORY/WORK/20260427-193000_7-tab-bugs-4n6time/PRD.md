---
task: Fix 7 tab system bugs in 4n6time-go
slug: 20260427-193000_7-tab-bugs-4n6time
effort: advanced
phase: complete
progress: 32/32
mode: interactive
started: 2026-04-27T19:30:00Z
updated: 2026-04-27T19:33:00Z
---

## Context

7 bugs found during tab system testing in 4n6time-go forensic timeline viewer. Bugs span Go backend (app.go, database.go), React frontend (App.jsx, style.css, TabBar.jsx), and Wails native menu (main.go).

### Key Architecture Notes
- `shouldIncludeExaminerNotes` inspects raw SQL strings, but parameterized queries use `?` not literal values
- Themes use `--border-accent-bright` not `--accent-color` (which is undefined)
- WebKit/Wails: `<select>` elements require `-webkit-appearance: none` for CSS background to apply
- TabBar gear removed; settings move to Tools > Settings native menu

## Criteria

### Bug 1: Examiner notes in URL-filtered tabs
- [x] ISC-1: `shouldIncludeExaminerNotes` returns false when SQL has `/* no-notes */` comment
- [x] ISC-2: app.go prepends `/* no-notes */` to SQL when baseField is non-examiner-note field
- [x] ISC-3: app.go prepends `/* no-notes */` to count SQL (same condition)
- [x] ISC-4: `source != 'EXAMINER'` predicate removed from app.go (no longer needed)
- [x] ISC-5: URL-filtered tab shows no examiner notes (verified via code path)

### Bug 2: Context menu dismissal
- [x] ISC-6: Context menu header has an X button that calls `setContextMenu(null)`
- [x] ISC-7: X button is visually styled in the context menu header

### Bug 3: Inactive tab visual contrast
- [x] ISC-8: `.tab:not(.active)` has a visible background using a theme variable
- [x] ISC-9: Inactive tab text color is more readable than before (uses `--text-secondary` or similar)
- [x] ISC-10: Active tab remains clearly more prominent than inactive tabs

### Bug 4: Active tab border uses correct theme variable
- [x] ISC-11: `--accent-color` replaced with `--border-accent-bright` in tab.active CSS
- [x] ISC-12: Active tab border visually changes when switching themes

### Bug 5: Move gear to Tools > Settings menu
- [x] ISC-13: `main.go` has a Tools submenu between View and Help
- [x] ISC-14: Tools menu has "Settings..." item emitting `menu:settings`
- [x] ISC-15: `SettingsDialog.jsx` created with max-tabs number input
- [x] ISC-16: SettingsDialog loads current tabLimit from `GetTabLimit` on open
- [x] ISC-17: SettingsDialog saves new limit via `SetTabLimit` on apply
- [x] ISC-18: SettingsDialog has a Close button and modal-overlay dismiss
- [x] ISC-19: App.jsx has `showSettings` state and renders `<SettingsDialog>`
- [x] ISC-20: App.jsx listens for `menu:settings` to set `showSettings(true)`
- [x] ISC-21: TabBar.jsx has gear button and gear popover completely removed
- [x] ISC-22: TabBar.jsx no longer receives `tabLimit` or `onSetTabLimit` props
- [x] ISC-23: App.jsx no longer passes gear-related props to TabBar
- [x] ISC-24: Stale refresh button in tab-bar-actions still present and works

### Bug 6: Filter dropdown styling regression
- [x] ISC-25: `.filter-row select` has `-webkit-appearance: none; appearance: none;`
- [x] ISC-26: `.filter-row select` has a CSS dropdown arrow via background-image
- [x] ISC-27: `color-scheme` only applies to `.filter-row input`, not `.filter-row select`

### Bug 7: Filename/URL exact match instead of LIKE
- [x] ISC-28: `filename` field in context menu fieldDefs uses `=` operator (not LIKE)
- [x] ISC-29: `URL` field in context menu fieldDefs uses `=` operator (not LIKE)
- [x] ISC-30: `event_identifier` still uses `=` (unchanged)
- [x] ISC-31: Other fields (host, user, source, etc.) still use `LIKE` (unchanged)

### Anti-criteria
- [x] ISC-A1: No existing feature broken (imports, filters, pagination, search, export)
- [x] ISC-A2: No Store interface changes (examiner notes fix stays in SQL comments)

## Decisions

### Bug 1: SQL comment approach
Parameterized queries embed `?` not literal values, so `shouldIncludeExaminerNotes` regex never matches. Instead: append `/* no-notes */` SQL comment when notes should be excluded. This is detectable by the function without changing the Store interface.

### Bug 5: New SettingsDialog
Modal pattern from LoggingDialog.jsx — modal-overlay click to close, inner div stops propagation, loads state on visibility change.

## Verification

- ISC-1/2/3/4: Verified `/* no-notes */` comment added in app.go, detected in database.go. Source != EXAMINER predicate removed.
- ISC-6/7: context-menu-close button present in App.jsx and styled in style.css.
- ISC-8/9/10/11/12: Tab CSS uses `--border-accent-bright` (all themes define it), inactive tab has `var(--bg-tab-inactive, rgba(0,0,0,0.15))`.
- ISC-13/14: `toolsMenu` in main.go between View and Help, emits `menu:settings`.
- ISC-15-24: SettingsDialog.jsx created, App.jsx wires `menu:settings`, TabBar.jsx has no gear refs.
- ISC-25/26/27: `-webkit-appearance: none` + custom SVG arrow on `.filter-row select`, `color-scheme` stays only on `.filter-row input`.
- ISC-28/29/30/31: `exactFields` Set in fieldDefs covers `event_identifier`, `filename`, `URL`. Other fields still use LIKE.
- ISC-A1: `go test ./...` passes, `wails build` succeeds.
- ISC-A2: No Store interface changes.
