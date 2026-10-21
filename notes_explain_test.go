package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// mxExplainRows is the evidence volume for the plan check, matching the Pass 0
// spike (100k rows).
const mxExplainRows = 100000

// TestNotesMatrixExplainPlan checks the SQLite query plans, before and after
// ANALYZE, for the derived-table grid query with a host filter and with a
// datetime range, and for the count's evidence half with a host filter. Each
// must name its index in the plan text, so a plan regression fails the build.
// The count's notes half is run with no expectation (examiner_notes has no
// host column to index).
func TestNotesMatrixExplainPlan(t *testing.T) {
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "explain.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	hosts := []string{"WS1", "WS2", "WS3", "WS4", "WS5", "WS6", "WS7", "WS8", "WS9", "WS10"}
	events := make([]*model.Event, 0, mxExplainRows)
	for i := 0; i < mxExplainRows; i++ {
		events = append(events, &model.Event{
			Datetime: fmt.Sprintf("2025-%02d-%02d %02d:%02d:00", i%12+1, i%28+1, i%24, i%60),
			Source:   "FILE",
			Host:     hosts[i%len(hosts)],
			Desc:     fmt.Sprintf("event %d", i),
		})
	}
	if _, err := store.InsertEvents(events, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := store.InsertExaminerNote(fmt.Sprintf("2025-06-%02d 12:00:00", i+1), fmt.Sprintf("note %d", i), "", ""); err != nil {
			t.Fatalf("InsertExaminerNote: %v", err)
		}
	}
	app := &App{store: store, driver: "sqlite"}

	hostReq := QueryRequest{Logic: "AND", OrderBy: "datetime", Filters: []FilterItem{{Field: "host", Operator: "=", Value: "WS1"}}}
	dateReq := QueryRequest{Logic: "AND", OrderBy: "datetime", DateFrom: "2025-03-01", DateTo: "2025-03-31"}

	// want is text the plan must contain; empty means no expectation.
	type plan struct {
		label, want string
		sql         string
		args        []interface{}
	}
	var plans []plan
	for _, r := range []struct {
		label, want string
		req         QueryRequest
	}{
		{"grid, host = ?", "USING INDEX host_idx", hostReq},
		{"grid, datetime range", "USING INDEX datetime_idx", dateReq},
	} {
		q, err := buildSimpleQuery(r.req, app.queryDialect(), 1000)
		if err != nil {
			t.Fatalf("buildSimpleQuery: %v", err)
		}
		q.OrderBy(r.req.OrderBy)
		sqlStr, args := q.BuildCombined()
		plans = append(plans, plan{r.label, r.want, sqlStr, args})
	}
	q, err := buildSimpleQuery(hostReq, app.queryDialect(), 1000)
	if err != nil {
		t.Fatalf("buildSimpleQuery: %v", err)
	}
	countSQL, countArgs := q.BuildCount()
	plans = append(plans, plan{"count evidence half, host = ?", "USING COVERING INDEX host_idx", countSQL, countArgs})
	notesSQL, notesArgs := q.BuildNotesCount()
	plans = append(plans, plan{"count notes half, host = ?", "", notesSQL, notesArgs})

	conn := store.Conn()
	for _, stage := range []string{"before ANALYZE", "after ANALYZE"} {
		if stage == "after ANALYZE" {
			if _, err := conn.Exec("ANALYZE"); err != nil {
				t.Fatalf("ANALYZE: %v", err)
			}
		}
		for _, p := range plans {
			rows, err := conn.Query("EXPLAIN QUERY PLAN "+p.sql, p.args...)
			if err != nil {
				t.Fatalf("EXPLAIN %s: %v", p.label, err)
			}
			var lines []string
			for rows.Next() {
				var id, parent, notused int
				var detail string
				if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
					rows.Close()
					t.Fatalf("scanning plan: %v", err)
				}
				lines = append(lines, fmt.Sprintf("    [%d<-%d] %s", id, parent, detail))
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				t.Fatalf("reading plan for %s: %v", p.label, err)
			}
			text := strings.Join(lines, "\n")
			if p.want != "" && !strings.Contains(text, p.want) {
				t.Errorf("%s, %s: plan does not contain %q\n  sql: %s\n  args: %v\n%s", stage, p.label, p.want, p.sql, p.args, text)
			}
		}
	}
}

// TestNotesMatrixExportDatetimeForm checks that a SQLite CSV export writes
// evidence and note datetimes in the RFC 3339 display form.
func TestNotesMatrixExportDatetimeForm(t *testing.T) {
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "exportform.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.InsertEvents([]*model.Event{{Datetime: "2025-01-10 10:00:00", Source: "FILE", Host: "WS1", Desc: "malware.exe"}}, nil); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}
	if _, err := store.InsertExaminerNote("2025-03-01 09:00:00", "beacon to c2", "", ""); err != nil {
		t.Fatalf("InsertExaminerNote: %v", err)
	}
	app := &App{store: store, driver: "sqlite"}
	path := filepath.Join(t.TempDir(), "form.csv")
	if _, err := app.exportCSVTo(path, QueryRequest{OrderBy: "datetime"}, func(string) {}); err != nil {
		t.Fatalf("exportCSVTo: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening export: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}
	if len(rows) == 0 || len(rows[0]) == 0 || rows[0][0] != "datetime" {
		t.Fatalf("export header = %v, want datetime first", rows)
	}
	var got []string
	for _, r := range rows[1:] {
		got = append(got, r[0])
	}
	want := []string{"2025-01-10T10:00:00Z", "2025-03-01T09:00:00Z"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("export datetimes = %v, want %v", got, want)
	}
}
