package query

import (
	"strings"
	"testing"
)

func TestSimplePredicate(t *testing.T) {
	p := Simple("source", Equal, "FILE")
	if p == nil {
		t.Fatal("expected non-nil predicate")
	}

	sql, args := p.WhereClause()
	if sql != "(source = ?)" {
		t.Errorf("expected '(source = ?)', got '%s'", sql)
	}
	if len(args) != 1 || args[0] != "FILE" {
		t.Errorf("expected args ['FILE'], got %v", args)
	}
}

func TestSimplePredicateInvalidField(t *testing.T) {
	p := Simple("DROP TABLE", Equal, "oops")
	if p != nil {
		t.Error("expected nil for invalid field name")
	}
}

func TestSimplePredicateInvalidOperator(t *testing.T) {
	p := Simple("source", "HACK", "value")
	if p != nil {
		t.Error("expected nil for invalid operator")
	}
}

func TestLikePredicate(t *testing.T) {
	p := Simple("desc", Like, "malware")
	sql, args := p.WhereClause()

	if sql != "(desc LIKE ?)" {
		t.Errorf("expected '(desc LIKE ?)', got '%s'", sql)
	}
	if len(args) != 1 || args[0] != "%malware%" {
		t.Errorf("expected args ['%%malware%%'], got %v", args)
	}
}

func TestNotLikePredicate(t *testing.T) {
	p := Simple("filename", NotLike, "tmp")
	sql, args := p.WhereClause()

	if sql != "(filename NOT LIKE ?)" {
		t.Errorf("expected '(filename NOT LIKE ?)', got '%s'", sql)
	}
	if len(args) != 1 || args[0] != "%tmp%" {
		t.Errorf("expected args ['%%tmp%%'], got %v", args)
	}
}

func TestNotEqualPredicate(t *testing.T) {
	p := Simple("color", NotEqual, "RED")
	sql, args := p.WhereClause()

	if sql != "(color != ?)" {
		t.Errorf("expected '(color != ?)', got '%s'", sql)
	}
	if len(args) != 1 || args[0] != "RED" {
		t.Errorf("expected args ['RED'], got %v", args)
	}
}

func TestDateRangePredicate(t *testing.T) {
	p := DateRange("2025-01-01 00:00:00", "2025-06-30 23:59:59")
	sql, args := p.WhereClause()

	if sql != "(datetime BETWEEN datetime(?) AND datetime(?))" {
		t.Errorf("unexpected sql: %s", sql)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "2025-01-01 00:00:00" || args[1] != "2025-06-30 23:59:59" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestCombineAND(t *testing.T) {
	p1 := Simple("source", Equal, "FILE")
	p2 := Simple("host", Equal, "WORKSTATION1")

	combined := Combine([]*Predicate{p1, p2}, AND)
	sql, args := combined.WhereClause()

	if sql != "((source = ?) AND (host = ?))" {
		t.Errorf("unexpected sql: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestCombineOR(t *testing.T) {
	p1 := Simple("source", Equal, "FILE")
	p2 := Simple("source", Equal, "REG")

	combined := Combine([]*Predicate{p1, p2}, OR)
	sql, args := combined.WhereClause()

	if sql != "((source = ?) OR (source = ?))" {
		t.Errorf("unexpected sql: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestCombineThree(t *testing.T) {
	p1 := Simple("source", Equal, "FILE")
	p2 := Simple("host", Equal, "WS1")
	p3 := Simple("user", Equal, "admin")

	combined := Combine([]*Predicate{p1, p2, p3}, AND)
	sql, args := combined.WhereClause()

	// Should produce a left-leaning tree: ((p1 AND p2) AND p3)
	if !strings.Contains(sql, "AND") {
		t.Errorf("expected AND in sql: %s", sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestCombineSingle(t *testing.T) {
	p := Simple("source", Equal, "FILE")
	combined := Combine([]*Predicate{p}, AND)

	// Single predicate should just return itself
	sql, args := combined.WhereClause()
	if sql != "(source = ?)" {
		t.Errorf("expected simple predicate sql, got: %s", sql)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestCombineEmpty(t *testing.T) {
	combined := Combine([]*Predicate{}, AND)
	if combined != nil {
		t.Error("expected nil for empty combine")
	}
}

func TestCombineSkipsNils(t *testing.T) {
	p := Simple("source", Equal, "FILE")
	combined := Combine([]*Predicate{nil, p, nil}, AND)

	sql, args := combined.WhereClause()
	if sql != "(source = ?)" {
		t.Errorf("expected simple predicate sql, got: %s", sql)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestCombineAllNils(t *testing.T) {
	combined := Combine([]*Predicate{nil, nil}, AND)
	if combined != nil {
		t.Error("expected nil when all predicates are nil")
	}
}

func TestNilPredicateWhereClause(t *testing.T) {
	var p *Predicate
	sql, args := p.WhereClause()
	if sql != "" {
		t.Errorf("expected empty sql, got: %s", sql)
	}
	if args != nil {
		t.Errorf("expected nil args, got: %v", args)
	}
}

func TestPredicateFields(t *testing.T) {
	p1 := Simple("source", Equal, "FILE")
	p2 := DateRange("2025-01-01", "2025-12-31")
	p3 := Simple("source", Like, "REG") // duplicate field

	combined := Combine([]*Predicate{p1, p2, p3}, AND)
	fields := combined.Fields()

	// Should contain "source" and "datetime" (deduplicated)
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d: %v", len(fields), fields)
	}

	hasSource := false
	hasDatetime := false
	for _, f := range fields {
		if f == "source" {
			hasSource = true
		}
		if f == "datetime" {
			hasDatetime = true
		}
	}
	if !hasSource || !hasDatetime {
		t.Errorf("expected source and datetime, got: %v", fields)
	}
}

// --- Query builder tests ---

func TestQueryBuildNoPredicates(t *testing.T) {
	q := New(0)
	sql, args := q.Build()

	if !strings.HasPrefix(sql, "SELECT rowid,") {
		t.Errorf("expected SELECT rowid prefix, got: %s", sql)
	}
	if !strings.Contains(sql, "FROM log2timeline") {
		t.Errorf("expected FROM log2timeline, got: %s", sql)
	}
	if strings.Contains(sql, "WHERE") {
		t.Errorf("expected no WHERE clause, got: %s", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestQueryBuildWithPredicate(t *testing.T) {
	q := New(0)
	q.AddPredicate(Simple("source", Equal, "FILE"))

	sql, args := q.Build()

	if !strings.Contains(sql, "WHERE (source = ?)") {
		t.Errorf("expected WHERE clause, got: %s", sql)
	}
	if len(args) != 1 || args[0] != "FILE" {
		t.Errorf("expected args ['FILE'], got %v", args)
	}
}

func TestQueryBuildWithOrderBy(t *testing.T) {
	q := New(0)
	q.OrderBy("datetime")

	sql, _ := q.Build()

	if !strings.Contains(sql, "ORDER BY datetime") {
		t.Errorf("expected ORDER BY datetime, got: %s", sql)
	}
}

func TestQueryOrderByInvalidField(t *testing.T) {
	q := New(0)
	err := q.OrderBy("DROP TABLE")
	if err == nil {
		t.Error("expected error for invalid order by field")
	}
}

func TestQueryOrderByRowid(t *testing.T) {
	q := New(0)
	err := q.OrderBy("rowid")
	if err != nil {
		t.Errorf("expected rowid to be valid, got error: %v", err)
	}
}

func TestQueryBuildWithPagination(t *testing.T) {
	q := New(1000)
	q.SetPage(1)

	sql, _ := q.Build()
	if !strings.Contains(sql, "LIMIT 1000 OFFSET 0") {
		t.Errorf("expected LIMIT 1000 OFFSET 0, got: %s", sql)
	}

	q.SetPage(3)
	sql, _ = q.Build()
	if !strings.Contains(sql, "LIMIT 1000 OFFSET 2000") {
		t.Errorf("expected LIMIT 1000 OFFSET 2000, got: %s", sql)
	}
}

func TestQuerySetPageIgnoresInvalid(t *testing.T) {
	q := New(100)
	q.SetPage(5)
	q.SetPage(0)  // should be ignored
	q.SetPage(-1) // should be ignored

	if q.PageNumber() != 5 {
		t.Errorf("expected page 5, got %d", q.PageNumber())
	}
}

func TestQueryBuildFull(t *testing.T) {
	q := New(1000)
	q.AddPredicate(Simple("source", Equal, "FILE"))
	q.AddPredicate(DateRange("2025-01-01", "2025-06-30"))
	q.SetLogic(AND)
	q.OrderBy("datetime")
	q.SetPage(2)

	sql, args := q.Build()

	if !strings.Contains(sql, "WHERE") {
		t.Errorf("expected WHERE, got: %s", sql)
	}
	if !strings.Contains(sql, "AND") {
		t.Errorf("expected AND, got: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY datetime") {
		t.Errorf("expected ORDER BY, got: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT 1000 OFFSET 1000") {
		t.Errorf("expected LIMIT/OFFSET for page 2, got: %s", sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(args), args)
	}
}

func TestQueryBuildCount(t *testing.T) {
	q := New(1000)
	q.AddPredicate(Simple("host", Equal, "WS1"))

	sql, args := q.BuildCount()

	if !strings.HasPrefix(sql, "SELECT COUNT(rowid) FROM log2timeline") {
		t.Errorf("expected COUNT query, got: %s", sql)
	}
	if !strings.Contains(sql, "WHERE (host = ?)") {
		t.Errorf("expected WHERE clause, got: %s", sql)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestQueryPredicateFields(t *testing.T) {
	q := New(0)
	q.AddPredicate(Simple("source", Equal, "FILE"))
	q.AddPredicate(Simple("host", Like, "WS"))
	q.AddPredicate(Simple("source", NotEqual, "EVT")) // duplicate field

	fields := q.PredicateFields()

	if len(fields) != 2 {
		t.Errorf("expected 2 unique fields, got %d: %v", len(fields), fields)
	}
}

func TestQueryRemovePredicate(t *testing.T) {
	q := New(0)
	p1 := Simple("source", Equal, "FILE")
	p2 := Simple("host", Equal, "WS1")

	q.AddPredicate(p1)
	q.AddPredicate(p2)
	q.RemovePredicate(p1)

	sql, args := q.Build()
	if !strings.Contains(sql, "host") {
		t.Errorf("expected host predicate to remain, got: %s", sql)
	}
	if strings.Contains(sql, "AND") {
		t.Errorf("expected no AND after removing predicate, got: %s", sql)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestQueryClearPredicates(t *testing.T) {
	q := New(0)
	q.AddPredicate(Simple("source", Equal, "FILE"))
	q.AddPredicate(Simple("host", Equal, "WS1"))
	q.ClearPredicates()

	sql, args := q.Build()
	if strings.Contains(sql, "WHERE") {
		t.Errorf("expected no WHERE after clear, got: %s", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestQueryORLogic(t *testing.T) {
	q := New(0)
	q.SetLogic(OR)
	q.AddPredicate(Simple("source", Equal, "FILE"))
	q.AddPredicate(Simple("source", Equal, "REG"))

	sql, _ := q.Build()
	if !strings.Contains(sql, "OR") {
		t.Errorf("expected OR logic, got: %s", sql)
	}
}

// --- RawQuery tests ---

func TestRawQueryBuild(t *testing.T) {
	rq := NewRaw(1000, "source = 'FILE' AND host = 'WS1'")
	rq.OrderBy("datetime")
	rq.SetPage(1)

	sql, args := rq.Build()

	if !strings.Contains(sql, "WHERE source = 'FILE' AND host = 'WS1'") {
		t.Errorf("expected raw WHERE clause, got: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY datetime") {
		t.Errorf("expected ORDER BY, got: %s", sql)
	}
	if !strings.Contains(sql, "LIMIT 1000 OFFSET 0") {
		t.Errorf("expected LIMIT/OFFSET, got: %s", sql)
	}
	if args != nil {
		t.Errorf("expected nil args for raw query, got: %v", args)
	}
}

func TestRawQueryEmpty(t *testing.T) {
	rq := NewRaw(0, "")
	sql, _ := rq.Build()

	if strings.Contains(sql, "WHERE") {
		t.Errorf("expected no WHERE for empty raw query, got: %s", sql)
	}
}

func TestRawQuerySetRawWhere(t *testing.T) {
	rq := NewRaw(0, "source = 'FILE'")
	rq.SetRawWhere("host = 'WS2'")

	sql, _ := rq.Build()
	if !strings.Contains(sql, "host = 'WS2'") {
		t.Errorf("expected updated WHERE, got: %s", sql)
	}
}
