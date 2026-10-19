package query

import (
	"strings"
	"testing"
)

func TestSimpleIntBindsInteger(t *testing.T) {
	p := SimpleInt("import_id", Equal, 7)
	if p == nil {
		t.Fatal("SimpleInt(import_id, =) returned nil")
	}
	sql, args := p.WhereClause()
	if sql != "(import_id = ?)" {
		t.Errorf("sql = %q, want (import_id = ?)", sql)
	}
	if len(args) != 1 || args[0] != int64(7) {
		t.Errorf("args = %#v, want [int64(7)]", args)
	}
}

func TestSimpleIntRejectsOtherOperatorsAndFields(t *testing.T) {
	if SimpleInt("import_id", Like, 7) != nil {
		t.Error("LIKE accepted for integer predicate")
	}
	if SimpleInt("import_id", GreaterOrEqual, 7) != nil {
		t.Error(">= accepted for integer predicate")
	}
	if SimpleInt("no_such_field", Equal, 7) != nil {
		t.Error("unknown field accepted")
	}
}

func TestSimpleIntCombinesWithStringPredicates(t *testing.T) {
	combined := Combine([]*Predicate{
		Simple("source", Equal, "FILE"),
		SimpleInt("import_id", Equal, 3),
	}, OR)
	sql, args := combined.WhereClause()
	if !strings.Contains(sql, "source = ?") || !strings.Contains(sql, "import_id = ?") || !strings.Contains(sql, "OR") {
		t.Errorf("sql = %q", sql)
	}
	if len(args) != 2 || args[0] != "FILE" || args[1] != int64(3) {
		t.Errorf("args = %#v, want [FILE int64(3)]", args)
	}
}
