package query

import (
	"reflect"
	"strings"
	"testing"
)

// pgTestDialect numbers placeholders and quotes the reserved columns the way
// database.PostgresDialect does, without importing the database package.
type pgTestDialect struct{}

func (pgTestDialect) Placeholder(i int) string { return "$" + string(rune('0'+i)) }
func (pgTestDialect) IDColumn() string         { return "id" }
func (pgTestDialect) DateBetweenSQL(a, b int) string {
	return "(datetime BETWEEN $" + string(rune('0'+a)) + " AND $" + string(rune('0'+b)) + ")"
}
func (pgTestDialect) NoteDatetimeSQL(column string) string { return column }
func (pgTestDialect) OffsetSQLType() string                { return "BIGINT" }
func (pgTestDialect) QuoteColumn(name string) string {
	switch name {
	case "user", "desc", "offset":
		return `"` + name + `"`
	}
	return name
}

// Constraints are ANDed outside the filter group, so OR logic cannot widen them.
func TestAddConstraintOutsideOrGroup(t *testing.T) {
	q := New(0)
	q.SetLogic(OR)
	q.SetBase(Simple("source", Equal, "FILE"))
	q.AddPredicate(Simple("host", Equal, "WS1"))
	q.AddPredicate(Simple("user", Equal, "bob"))
	q.AddConstraint(Simple("bookmark", Equal, "1"))
	q.AddConstraint(Simple("datetime", GreaterOrEqual, "2025-01-01 00:00:00"))
	q.AddConstraint(nil)

	sql, args := q.BuildCount()
	want := "SELECT COUNT(rowid) FROM log2timeline WHERE ((((source = ?) AND ((host = ?) OR (user = ?))) AND (bookmark = ?)) AND (datetime >= ?))"
	if sql != want {
		t.Errorf("sql =\n  %s\nwant\n  %s", sql, want)
	}
	wantArgs := []interface{}{"FILE", "WS1", "bob", "1", "2025-01-01 00:00:00"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("args = %v, want %v", args, wantArgs)
	}
}

// A constraint alone, with no base or filters, is the whole WHERE.
func TestAddConstraintAlone(t *testing.T) {
	q := New(0)
	q.AddConstraint(Simple("bookmark", Equal, "1"))
	sql, args := q.BuildCount()
	if want := "SELECT COUNT(rowid) FROM log2timeline WHERE (bookmark = ?)"; sql != want {
		t.Errorf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != "1" {
		t.Errorf("args = %v, want [1]", args)
	}
}

func TestNotesProjection(t *testing.T) {
	got := NotesProjection(DefaultDialect)
	for _, part := range []string{
		"SELECT -id AS rowid, replace(replace(datetime, 'T', ' '), 'Z', '') AS datetime, CAST(NULL AS TEXT) AS timezone",
		"'EXAMINER' AS source, 'Examiner Note' AS sourcetype",
		"description AS desc",
		"tag AS tag, color AS color",
		"CAST(NULL AS INTEGER) AS offset",
		"bookmark AS bookmark, CAST(NULL AS INTEGER) AS import_id FROM examiner_notes",
	} {
		if !strings.Contains(got, part) {
			t.Errorf("projection lacks %q:\n%s", part, got)
		}
	}
	if strings.Contains(got, "'' AS ") {
		t.Errorf("projection writes '' for an unmapped column; want NULL:\n%s", got)
	}

	pg := NotesProjection(pgTestDialect{})
	for _, part := range []string{"-id AS id, datetime AS datetime", `description AS "desc"`, `CAST(NULL AS TEXT) AS "user"`, `CAST(NULL AS BIGINT) AS "offset"`,
		"CAST(NULL AS INTEGER) AS store_number", "CAST(NULL AS INTEGER) AS import_id"} {
		if !strings.Contains(pg, part) {
			t.Errorf("PostgreSQL projection lacks %q:\n%s", part, pg)
		}
	}
	// Only offset widens; every other INT column keeps its INTEGER cast.
	if n := strings.Count(pg, "CAST(NULL AS BIGINT)"); n != 1 {
		t.Errorf("PostgreSQL projection has %d BIGINT casts, want 1:\n%s", n, pg)
	}
}

func TestBuildCombined(t *testing.T) {
	q := New(50)
	q.AddPredicate(Simple("host", Equal, "WS1"))
	q.OrderBy("datetime")
	q.SetPage(3)

	sql, args := q.BuildCombined()
	evidenceSQL, _ := New(0).Build()
	wantPrefix := "SELECT " + selectList(DefaultDialect) + " FROM (" + evidenceSQL + " UNION ALL " + NotesProjection(DefaultDialect) + ") AS log2timeline"
	if !strings.HasPrefix(sql, wantPrefix) {
		t.Fatalf("combined query does not start with the derived table:\n%s", sql)
	}
	if tail := sql[len(wantPrefix):]; tail != " WHERE (host = ?) ORDER BY datetime LIMIT 50 OFFSET 100" {
		t.Errorf("tail = %q", tail)
	}
	if len(args) != 1 || args[0] != "WS1" {
		t.Errorf("args = %v, want [WS1]", args)
	}
}

// The two count halves share one WHERE and one argument list, numbered from $1
// on PostgreSQL in each statement.
func TestBuildNotesCountMatchesEvidenceCount(t *testing.T) {
	q := New(0)
	q.SetDialect(pgTestDialect{})
	q.SetBase(Simple("desc", Like, "malware"))
	q.AddPredicate(Simple("color", Equal, "BLUE"))
	q.AddConstraint(Simple("bookmark", Equal, "1"))

	evSQL, evArgs := q.BuildCount()
	notesSQL, notesArgs := q.BuildNotesCount()
	where := ` WHERE ((("desc" LIKE $1) AND (color = $2)) AND (bookmark = $3))`
	if want := "SELECT COUNT(id) FROM log2timeline" + where; evSQL != want {
		t.Errorf("evidence count =\n  %s\nwant\n  %s", evSQL, want)
	}
	if want := "SELECT COUNT(*) FROM (" + NotesProjection(pgTestDialect{}) + ") AS log2timeline" + where; notesSQL != want {
		t.Errorf("notes count =\n  %s\nwant\n  %s", notesSQL, want)
	}
	if !reflect.DeepEqual(evArgs, notesArgs) {
		t.Errorf("args differ: evidence %v, notes %v", evArgs, notesArgs)
	}
}

func TestRawQueryCombined(t *testing.T) {
	clause := "desc LIKE '% order by %'"
	rq := NewRaw(10, clause)
	rq.OrderBy("datetime")

	sql, args := rq.BuildCombined()
	if args != nil {
		t.Errorf("args = %v, want nil", args)
	}
	want := "SELECT " + selectList(DefaultDialect) + " FROM " + CombinedSource(DefaultDialect) +
		" WHERE " + clause + " ORDER BY datetime LIMIT 10 OFFSET 0"
	if sql != want {
		t.Errorf("sql =\n  %s\nwant\n  %s", sql, want)
	}

	notesSQL, _ := rq.BuildNotesCount()
	if want := "SELECT COUNT(*) FROM (" + NotesProjection(DefaultDialect) + ") AS log2timeline WHERE " + clause; notesSQL != want {
		t.Errorf("notes count = %q", notesSQL)
	}

	empty := NewRaw(0, "")
	if sql, _ := empty.BuildNotesCount(); strings.Contains(sql, "WHERE") {
		t.Errorf("empty clause produced a WHERE: %q", sql)
	}
}

func TestBuildWhere(t *testing.T) {
	q := New(10)
	q.SetLogic(OR)
	q.AddPredicate(Simple("host", Equal, "WS1"))
	q.AddPredicate(Simple("color", Equal, "RED"))
	q.AddConstraint(Simple("bookmark", Equal, "1"))
	where, args := q.BuildWhere()
	if want := "(((host = ?) OR (color = ?)) AND (bookmark = ?))"; where != want {
		t.Errorf("where = %q, want %q", where, want)
	}
	if !reflect.DeepEqual(args, []interface{}{"WS1", "RED", "1"}) {
		t.Errorf("args = %v", args)
	}
	if where, args := New(0).BuildWhere(); where != "" || args != nil {
		t.Errorf("empty query: where %q, args %v; want empty", where, args)
	}
}

func TestNotesSourceAlias(t *testing.T) {
	if got, want := NotesSource(DefaultDialect), "("+NotesProjection(DefaultDialect)+") AS log2timeline"; got != want {
		t.Errorf("NotesSource = %q, want %q", got, want)
	}
	if !strings.HasSuffix(CombinedSource(DefaultDialect), ") AS log2timeline") {
		t.Errorf("CombinedSource is not aliased log2timeline: %s", CombinedSource(DefaultDialect))
	}
}
