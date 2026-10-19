package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cdtdelta/4n6time/internal/query"
)

// importIDField is the filter and base-query field that selects one import.
// It is an integer column, so its value must be a whole number and only "="
// applies. The field is not a grid column and has no examiner notes side.
const importIDField = "import_id"

// importIDNotNull excludes events from before import provenance, whose
// import_id is NULL, from the Import dropdown.
const importIDNotNull = "import_id IS NOT NULL"

// importIDValue checks an import filter or base-query value and returns it as
// an int64 for binding. Any operator other than "=" is rejected, and so is a
// value that is not a whole number.
func importIDValue(op, value string) (int64, error) {
	if op != "=" {
		return 0, fmt.Errorf("import_id supports only =, not %q", op)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("import_id must be a whole number, got %q: %w", value, err)
	}
	return n, nil
}

// importIDPredicate builds the parameterized predicate for an import filter row.
func importIDPredicate(op, value string) (*query.Predicate, error) {
	n, err := importIDValue(op, value)
	if err != nil {
		return nil, err
	}
	return query.SimpleInt(importIDField, query.Equal, n), nil
}

// baseOperator returns the base query operator, defaulting to "=" when unset.
func baseOperator(op string) string {
	if op == "" {
		return "="
	}
	return op
}
