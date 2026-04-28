---
task: Tab Pass 3 scoped filters save tab query
slug: 20260428-010000_tab-pass3-scoped-filters-save-query
effort: advanced
phase: complete
progress: 37/37
mode: interactive
started: 2026-04-28T01:00:00Z
updated: 2026-04-28T01:00:00Z
---

## Context

Tab System Pass 3 of 4 for 4n6time-go. Four features: scoped filter dropdowns per tab, save tab query, stale/refresh polish, and date range scoping. Filter dropdowns currently show all values in the DB regardless of which tab is active — a tab scoped to `host = WORKSTATION1` should show only sources/types/users that appear on that host. Tab queries can't be saved/reloaded. Refresh doesn't bump filterVersion. Date ranges aren't scoped either.

### Root Causes
- `GetDistinctValues` has no WHERE clause support; no `GetDistinctValuesFiltered` in Store
- `GetMinMaxDate` has no WHERE clause support; no `GetMinMaxDateFiltered` in Store
- FilterPanel has no `baseQuery` prop; always calls unscoped methods
- TabBar has no save button on non-main tabs; no `onSaveTab` prop
- SavedQueries doesn't detect `tabQuery: true` flag; no `onOpenInNewTab` prop
- `handleTabRefresh` doesn't bump `filterVersion` so scoped dropdowns don't reload on refresh

## Criteria

### Store interface and backend
- [x] ISC-1: `store.go` has `GetDistinctValuesFiltered(field, whereClause string, whereArgs []interface{}) (map[string]int64, error)`
- [x] ISC-2: `store.go` has `GetMinMaxDateFiltered(whereClause string, whereArgs []interface{}) (string, string, error)`
- [x] ISC-3: `SQLiteStore.GetDistinctValuesFiltered` validates field, queries with WHERE clause
- [x] ISC-4: `SQLiteStore.GetDistinctValuesFiltered` returns grouped distinct values scoped to whereClause
- [x] ISC-5: `SQLiteStore.GetMinMaxDateFiltered` queries min/max datetime scoped to whereClause
- [x] ISC-6: `PostgresStore.GetDistinctValuesFiltered` uses `pgQuoteCol`, queries with WHERE
- [x] ISC-7: `PostgresStore.GetDistinctValuesFiltered` returns grouped distinct values scoped to whereClause
- [x] ISC-8: `PostgresStore.GetMinMaxDateFiltered` queries min/max with `to_char` format, scoped to whereClause

### Wails app.go methods
- [x] ISC-9: `app.go` has `GetFilteredDistinctValues(field, baseField, baseOp, baseValue string)` method
- [x] ISC-10: `GetFilteredDistinctValues` validates field and baseField against `model.Fields`
- [x] ISC-11: `GetFilteredDistinctValues` validates baseOp is one of `=`, `!=`, `LIKE`, `NOT LIKE`
- [x] ISC-12: `GetFilteredDistinctValues` builds dialect-aware WHERE and calls `GetDistinctValuesFiltered`
- [x] ISC-13: `app.go` has `GetFilteredMinMaxDate(baseField, baseOp, baseValue string)` method
- [x] ISC-14: `GetFilteredMinMaxDate` validates baseField and baseOp, builds WHERE, calls `GetMinMaxDateFiltered`

### JS bindings
- [x] ISC-15: `App.js` has `GetFilteredDistinctValues(arg1, arg2, arg3, arg4)` export
- [x] ISC-16: `App.js` has `GetFilteredMinMaxDate(arg1, arg2, arg3)` export

### FilterPanel scoped behavior
- [x] ISC-17: `FilterPanel.jsx` accepts `baseQuery` prop `{ field, op, value } | null`
- [x] ISC-18: `FilterPanel.jsx` imports `GetFilteredDistinctValues` and `GetFilteredMinMaxDate`
- [x] ISC-19: FilterPanel calls `GetFilteredDistinctValues` when `baseQuery` is non-null
- [x] ISC-20: FilterPanel calls `GetDistinctValues` when `baseQuery` is null (unchanged path)
- [x] ISC-21: FilterPanel calls `GetFilteredMinMaxDate` for date range when `baseQuery` is non-null
- [x] ISC-22: FilterPanel calls `GetMinMaxDate` for date range when `baseQuery` is null (unchanged path)
- [x] ISC-23: `App.jsx` passes `activeTab?.baseQuery || null` as `baseQuery` prop to FilterPanel
- [x] ISC-24: `baseQuery` is included in FilterPanel loadValues effect dependency array

### Save tab query (TabBar)
- [x] ISC-25: `TabBar.jsx` accepts `onSaveTab(tabId, name)` prop
- [x] ISC-26: TabBar shows save icon button on non-main tabs when not in save mode
- [x] ISC-27: TabBar has inline save form (input + confirm + cancel) when save button clicked
- [x] ISC-28: TabBar calls `onSaveTab(tabId, name.trim())` on confirm; clears save mode

### Save tab query (App + SavedQueries)
- [x] ISC-29: `App.jsx` has `handleSaveTabQuery(tabId, name)` calling `SaveQuery` with tabQuery JSON
- [x] ISC-30: Saved JSON has `{"tabQuery": true, "field": "...", "op": "...", "value": "...", "label": "..."}`
- [x] ISC-31: `App.jsx` passes `onSaveTab={handleSaveTabQuery}` to `TabBar`
- [x] ISC-32: `SavedQueries.jsx` accepts `onOpenInNewTab` prop
- [x] ISC-33: `SavedQueries.jsx` detects `tabQuery === true` and calls `onOpenInNewTab({field, op, value, label})`
- [x] ISC-34: SavedQueries shows visual indicator (badge/prefix) for tab queries in list
- [x] ISC-35: `App.jsx` passes `onOpenInNewTab` callback to `SavedQueries`

### Stale/refresh polish
- [x] ISC-36: `handleTabRefresh` increments `filterVersion` to force dropdown reload
- [x] ISC-37: style.css has tab save button and inline form CSS classes

### Anti-criteria
- [x] ISC-A1: Main tab FilterPanel still loads unscoped values when `baseQuery` is null
- [x] ISC-A2: Existing saved query load behavior unchanged for non-tab queries
- [x] ISC-A3: No new Go module dependencies added

## Decisions

### Examiner notes in scoped results
`GetDistinctValuesFiltered` does NOT merge examiner_notes data (unlike the unscoped version). Scoped queries target `log2timeline` with a WHERE clause; examiner notes live in a separate table and lack fields like host/user/sourcetype. Including them in scoped results would pollute the dropdown.

### baseQuery dependency in FilterPanel
Pass the whole `baseQuery` object as a prop. It's created once per tab in `makeTab`/`handleOpenInNewTab` and never mutated; the reference is stable within a tab. Include it in the effect dependency array directly.

### JS bindings without a build
Since `Do not build` is in effect, manually add bindings to `App.js`. Follow existing pattern: `window['go']['main']['App']['MethodName'](args...)`.

### Tab save button position
Save button appears between the stale dot and close button. When in save mode, the save button is hidden and replaced by inline input + confirm/cancel buttons. Close button remains visible.

## Verification

- ISC-1/2: store.go lines 30 and 33 confirm both interface methods present.
- ISC-3/4/5: database.go lines 369-410 confirm SQLiteStore implementations with field validation and parameterized WHERE.
- ISC-6/7/8: postgres.go lines 590-636 confirm PostgresStore implementations with pgQuoteCol and to_char format.
- ISC-9-14: app.go lines 940-1000 confirm GetFilteredDistinctValues and GetFilteredMinMaxDate with model.Fields validation and operator whitelist.
- ISC-15/16: App.js lines 61-67 confirm both JS bindings present.
- ISC-17-24: FilterPanel.jsx has baseQuery prop, imports both scoped methods, conditionally dispatches based on baseQuery != null, includes baseQuery in effect deps.
- ISC-25-28: TabBar.jsx has onSaveTab prop, inline save form with input/confirm/cancel, calls onSaveTab(tabId, name.trim()) with empty-name guard.
- ISC-29-31: App.jsx has handleSaveTabQuery at line 857 calling SaveQuery; JSON includes tabQuery:true; onSaveTab passed to TabBar at line 1187.
- ISC-32-35: SavedQueries.jsx has onOpenInNewTab prop; handleLoad dispatches on tabQuery===true; Tab badge rendered with sq-tab-badge; App.jsx passes onOpenInNewTab at line 1287.
- ISC-36: handleTabRefresh at line 807-810 now calls setFilterVersion(v => v + 1).
- ISC-37: style.css has .tab-save, .tab-save-form, .tab-save-input, .tab-save-confirm, .tab-save-cancel, .sq-tab-badge, .sq-item-tab classes.
- ISC-A1: FilterPanel null-guards baseQuery: `if (baseQuery) { ... } else { GetDistinctValues(...) }`.
- ISC-A2: SavedQueries handleLoad only diverts on tabQuery===true; all other query formats still call onLoad.
- ISC-A3: go.mod unchanged — no new dependencies.
