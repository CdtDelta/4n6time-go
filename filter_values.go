package main

import (
	"errors"
	"fmt"
	"sort"
)

// filterPanelFields are the filter panel's dropdown fields, in panel order.
var filterPanelFields = []string{"source", "sourcetype", "type", "user", "host", importIDField}

// GetFilterValues returns every filter panel dropdown's values in one call:
// field name to its values. baseField, baseOp, and baseValue are the active
// tab's base query; an empty baseField means the unscoped main view.
//
// Unscoped values come from the metadata tables (Store.GetFilterValues), so no
// call scans log2timeline unless the metadata was never built. Scoped values
// run the existing per-field query (GetFilteredDistinctValues) unchanged, one
// field after another, inside this single call.
//
// A field that fails is logged and left out, as the panel did when it made one
// call per field. The call fails only when no field could be loaded.
func (a *App) GetFilterValues(baseField, baseOp, baseValue string) (map[string][]string, error) {
	if a.store == nil {
		return nil, fmt.Errorf("no database open")
	}

	var result map[string][]string
	var err error
	if baseField == "" {
		result, err = a.store.GetFilterValues(filterPanelFields)
	} else {
		result, err = a.scopedFilterValues(baseField, baseOp, baseValue)
	}
	if err != nil {
		a.logError("Loading filter values: " + err.Error())
		if len(result) == 0 {
			return nil, fmt.Errorf("loading filter values: %w", err)
		}
	}
	return result, nil
}

// scopedFilterValues runs GetFilteredDistinctValues for each panel field and
// keeps the values, sorted. Per-field errors are joined.
func (a *App) scopedFilterValues(baseField, baseOp, baseValue string) (map[string][]string, error) {
	result := make(map[string][]string, len(filterPanelFields))
	var errs []error
	for _, f := range filterPanelFields {
		counts, err := a.GetFilteredDistinctValues(f, baseField, baseOp, baseValue)
		if err != nil {
			errs = append(errs, fmt.Errorf("loading %s values: %w", f, err))
			continue
		}
		values := make([]string, 0, len(counts))
		for v := range counts {
			values = append(values, v)
		}
		sort.Strings(values)
		result[f] = values
	}
	return result, errors.Join(errs...)
}
