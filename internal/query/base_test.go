package query

import (
	"strings"
	"testing"
)

// The tab base is ANDed with the whole filter set, so it must not take part in
// the filters' OR logic.
func TestBaseIsANDedOutsideOrLogic(t *testing.T) {
	q := New(0)
	q.SetLogic(OR)
	q.AddPredicate(Simple("host", Equal, "WS2"))
	q.AddPredicate(Simple("source", Equal, "REG"))
	q.SetBase(Simple("source", Equal, "FILE"))

	sql, args := q.Build()
	want := "WHERE ((source = ?) AND ((host = ?) OR (source = ?)))"
	if !strings.Contains(sql, want) {
		t.Errorf("Build SQL = %q, want it to contain %q", sql, want)
	}
	if len(args) != 3 || args[0] != "FILE" || args[1] != "WS2" || args[2] != "REG" {
		t.Errorf("Build args = %#v, want [FILE WS2 REG]", args)
	}

	countSQL, countArgs := q.BuildCount()
	if !strings.Contains(countSQL, want) || len(countArgs) != 3 {
		t.Errorf("BuildCount = %q %#v, want the same base-ANDed WHERE", countSQL, countArgs)
	}
}

// With no base the WHERE clause is the filters alone, as before SetBase existed.
func TestNoBaseLeavesWhereUnchanged(t *testing.T) {
	plain := New(0)
	plain.SetLogic(OR)
	plain.AddPredicate(Simple("host", Equal, "WS2"))
	plain.AddPredicate(Simple("source", Equal, "REG"))
	wantSQL, wantArgs := plain.Build()

	cleared := New(0)
	cleared.SetLogic(OR)
	cleared.AddPredicate(Simple("host", Equal, "WS2"))
	cleared.AddPredicate(Simple("source", Equal, "REG"))
	cleared.SetBase(Simple("source", Equal, "FILE"))
	cleared.SetBase(nil)
	gotSQL, gotArgs := cleared.Build()

	if gotSQL != wantSQL || len(gotArgs) != len(wantArgs) {
		t.Errorf("cleared base changed SQL: got %q %#v, want %q %#v", gotSQL, gotArgs, wantSQL, wantArgs)
	}
}

// A base alone, with no filters, is still applied.
func TestBaseAloneIsApplied(t *testing.T) {
	q := New(0)
	q.SetBase(SimpleInt("import_id", Equal, 4))
	sql, args := q.Build()
	if !strings.Contains(sql, "WHERE (import_id = ?)") || len(args) != 1 || args[0] != int64(4) {
		t.Errorf("Build = %q %#v, want WHERE (import_id = ?) with int64(4)", sql, args)
	}
}
