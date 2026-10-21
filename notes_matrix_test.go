package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/model"
)

// The matrix runs in the default go test ./...: SQLite always, PostgreSQL when
// TEST_PG_DSN is set. -short skips the generated cases only. Run it alone with:
//   go test -count=1 -run NotesMatrix .
//
// Every mismatch fails the test. Failing generated cases are grouped by mode,
// failure kinds, and fields referenced (mxGroupKey). The log is bounded: a
// totals line, how often each field and failure kind appears, the largest
// mxLogGroups groups with mxLogExamples examples each, and a count of the
// rest. The report keeps every group with up to mxMaxExamples examples. It goes to
// testdata/notesmatrix_report.txt when a dialect fails or NOTESMATRIX_REPORT=1
// is set; a passing run leaves the file alone.
// The spec is in the oracle (notes_oracle_test.go); golden expectations here
// were worked out by hand from the fixture in notes_fixture_test.go.

const (
	mxPageSize   = 1000
	mxMaxPages   = 100
	mxReportDir  = "testdata"
	mxReportFile = "notesmatrix_report.txt"
	// mxMaxExamples caps the examples the report keeps per group, and the
	// non-RFC 3339 examples.
	mxMaxExamples = 10
	// mxLogGroups and mxLogExamples bound the generated-case failure log: the
	// largest mxLogGroups groups, with mxLogExamples examples each.
	mxLogGroups   = 20
	mxLogExamples = 3
	// mxLogFields is how many fields the log's field frequency summary lists.
	mxLogFields = 10
)

// mxExpect is the expected IDs for one golden case.
type mxExpect struct {
	ev, notes []string
}

func mxExp(ev, notes string) mxExpect {
	return mxExpect{ev: strings.Fields(ev), notes: strings.Fields(notes)}
}

// mxGoldenCase is one hand-written case. sqlite holds the expected IDs on
// SQLite. pg, when set, replaces them on PostgreSQL, where LIKE is case-sensitive.
type mxGoldenCase struct {
	id, label, why string
	c              mxCase
	sqlite         mxExpect
	pg             *mxExpect
	// hist, when set, is the expected histogram in mxRenderHist form, written
	// by hand and checked against both the oracle and the app. pgHist, when
	// set, replaces it on PostgreSQL.
	hist, pgHist string
}

// mxGolden returns the golden cases. Each expectation follows the spec and the
// fixture; none was copied from app output.
func mxGolden() []mxGoldenCase {
	adv := func(e oraExpr) mxCase { return mxCase{mode: "advanced", expr: e} }
	simple := func(logic string, filters ...oraFilter) mxCase {
		return mxCase{mode: "simple", logic: logic, filters: filters}
	}
	withDates := func(c mxCase) mxCase {
		c.dateFrom, c.dateTo = "2025-03-01", "2025-06-30"
		return c
	}
	pgOf := func(ev, notes string) *mxExpect {
		e := mxExp(ev, notes)
		return &e
	}
	allNotes := "N1 N2 N3 N4 N5 N6 N7"
	host := func(v string) oraFilter { return oraFilter{"host", "=", v} }
	user := func(v string) oraFilter { return oraFilter{"user", "=", v} }
	color := func(v string) oraFilter { return oraFilter{"color", "=", v} }
	base := func(field, value string) oraBase { return oraBase{field: field, value: value} }

	return []mxGoldenCase{
		{id: "A1", label: "color = 'RED' (advanced)", why: "notes compare on their own color column",
			c: adv(oraCmpExpr{"color", "=", "RED"}), sqlite: mxExp("E01 E04 E07 E11", "N1 N4")},
		{id: "A2", label: "datetime BETWEEN '2025-01-01' AND '2025-06-30' (advanced)",
			why: "raw literals: advanced datetimes are not normalized, so 2025-06-30 12:00 is after '2025-06-30'",
			c:   adv(oraBetweenExpr{"datetime", "2025-01-01", "2025-06-30"}), sqlite: mxExp("E01 E02 E03 E04 E05 E08 E13", "N1 N2 N5")},
		{id: "A3", label: "host = 'WS1' (advanced)", why: "notes have a NULL host, so the comparison is unknown and notes drop",
			c: adv(oraCmpExpr{"host", "=", "WS1"}), sqlite: mxExp("E01 E02 E03 E07 E08 E12", "")},
		{id: "A4", label: "desc LIKE '%beacon%' (advanced)", why: "SQLite LIKE folds ASCII case; PostgreSQL LIKE does not",
			c: adv(oraCmpExpr{"desc", "LIKE", "%beacon%"}), sqlite: mxExp("E02 E05 E09 E12", "N1 N5"), pg: pgOf("E02 E09", "N1")},
		{id: "A5", label: "bookmark = 1 (advanced)", why: "notes bookmark is their own value",
			c: adv(oraCmpExpr{"bookmark", "=", "1"}), sqlite: mxExp("E02 E06 E09 E13", "N1 N3")},
		{id: "A6", label: "tag LIKE '%lateral%' (advanced)", why: "only N1 has a tag (set through the store API)",
			c: adv(oraCmpExpr{"tag", "LIKE", "%lateral%"}), sqlite: mxExp("E02 E04 E06 E08", "N1")},
		{id: "A7", label: "source = 'EXAMINER' AND color = 'RED' (advanced)", why: "source is EXAMINER for every note, so color decides",
			c:      adv(oraBoolExpr{and: true, left: oraCmpExpr{"source", "=", "EXAMINER"}, right: oraCmpExpr{"color", "=", "RED"}}),
			sqlite: mxExp("", "N1 N4")},
		{id: "A8", label: "host = 'WS1' OR color = 'RED' (advanced)", why: "unknown OR true is true, so RED notes return",
			c:      adv(oraBoolExpr{and: false, left: oraCmpExpr{"host", "=", "WS1"}, right: oraCmpExpr{"color", "=", "RED"}}),
			sqlite: mxExp("E01 E02 E03 E04 E07 E08 E11 E12", "N1 N4")},
		{id: "A9", label: "source LIKE '%EXAM%' (advanced)", why: "every note's source is EXAMINER",
			c: adv(oraCmpExpr{"source", "LIKE", "%EXAM%"}), sqlite: mxExp("E14", allNotes)},
		{id: "A10", label: "source = 'examiner' (advanced)", why: "equality is case-sensitive on both dialects",
			c: adv(oraCmpExpr{"source", "=", "examiner"}), sqlite: mxExp("", "")},

		{id: "S1", label: "host = WS1 OR color = RED (simple, OR)", why: "an incompatible field under OR must not drop notes that RED satisfies",
			c: simple("OR", host("WS1"), color("RED")), sqlite: mxExp("E01 E02 E03 E04 E07 E08 E11 E12", "N1 N4")},
		{id: "S2", label: "color = RED (simple)", why: "compatible field, notes compare on their own color",
			c: simple("AND", color("RED")), sqlite: mxExp("E01 E04 E07 E11", "N1 N4")},
		{id: "S3", label: "host != WS1 (simple)", why: "notes have a NULL host, so != is unknown",
			c: simple("AND", oraFilter{"host", "!=", "WS1"}), sqlite: mxExp("E04 E05 E06 E09 E10 E11 E13 E14", "")},
		{id: "S4", label: "source = FILE (simple)", why: "source FILE is false for every note",
			c: simple("AND", oraFilter{"source", "=", "FILE"}), sqlite: mxExp("E01 E02 E06 E08 E12", "")},
		{id: "S5", label: "desc pivot base desc = 'beacon to c2' (simple)", why: "a compatible base applies to notes",
			c: mxCase{mode: "simple", logic: "AND", base: base("desc", "beacon to c2")}, sqlite: mxExp("E09", "N1")},
		{id: "S6", label: "keyword 'beacon' (simple)", why: "keyword search filters notes too",
			c: mxCase{mode: "simple", logic: "AND", search: "beacon"}, sqlite: mxExp("E02 E05 E09 E12", "N1 N5"), pg: pgOf("E02 E09", "N1")},

		{id: "G1", label: "NOT host = 'WS1' (advanced)", why: "NOT of unknown stays unknown, so notes drop",
			c: adv(oraNotExpr{oraCmpExpr{"host", "=", "WS1"}}), sqlite: mxExp("E04 E05 E06 E09 E10 E11 E13 E14", "")},
		{id: "G2", label: "sourcetype = 'Examiner Note' (advanced)", why: "notes carry sourcetype Examiner Note",
			c: adv(oraCmpExpr{"sourcetype", "=", "Examiner Note"}), sqlite: mxExp("E14", allNotes)},
		{id: "G3", label: "sourcetype != 'WinEVTX' (advanced)", why: "notes are not WinEVTX, so all return",
			c: adv(oraCmpExpr{"sourcetype", "!=", "WinEVTX"}), sqlite: mxExp("E01 E02 E03 E05 E06 E07 E08 E09 E11 E12 E14", allNotes)},
		{id: "G4", label: "desc LIKE '% order by %' (advanced)", why: "the clause contains ' order by ' inside a literal; the notes UNION split must not break it",
			c: adv(oraCmpExpr{"desc", "LIKE", "% order by %"}), sqlite: mxExp("E06", "")},
		{id: "G5", label: "source = 'EXAMINER' (simple)", why: "the evidence EXAMINER row returns with every note",
			c: simple("AND", oraFilter{"source", "=", "EXAMINER"}), sqlite: mxExp("E14", allNotes)},
		{id: "G5b", label: "source = 'EXAMINER' (advanced)", why: "same as G5 in advanced mode",
			c: adv(oraCmpExpr{"source", "=", "EXAMINER"}), sqlite: mxExp("E14", allNotes)},
		{id: "G6", label: "date range 2025-03-01 to 2025-06-30 (simple)", why: "N6 sits on the range end (23:59:59) and is included",
			c: withDates(mxCase{mode: "simple", logic: "AND"}), sqlite: mxExp("E03 E04 E05 E06 E07 E08 E13 E14", "N1 N2 N5 N6 N7")},
		{id: "G7", label: "desc pivot base + OR filter (simple)", why: "N1 is satisfied by its RED color under OR",
			c:      mxCase{mode: "simple", logic: "OR", base: base("desc", "beacon to c2"), filters: []oraFilter{host("WS1"), color("RED")}},
			sqlite: mxExp("", "N1")},
		{id: "G8", label: "keyword 'BEACON' (simple)", why: "keyword case folding differs by dialect",
			c: mxCase{mode: "simple", logic: "AND", search: "BEACON"}, sqlite: mxExp("E02 E05 E09 E12", "N1 N5"), pg: pgOf("E05", "")},
		{id: "G9", label: "bookmark-only + date range (simple)", why: "bookmark-only and the date range are ANDed",
			c: withDates(mxCase{mode: "simple", logic: "AND", bookmarkOnly: true}), sqlite: mxExp("E06 E13", "N1")},
		{id: "G10", label: "host = WS2 OR user = bob, bookmark-only (simple)", why: "bookmark-only is ANDed outside the OR group",
			c:      mxCase{mode: "simple", logic: "OR", bookmarkOnly: true, filters: []oraFilter{host("WS2"), user("bob")}},
			sqlite: mxExp("E06 E13", "")},
		{id: "G11", label: "host = WS1 OR user = bob, date range (simple)", why: "the date range is ANDed outside the OR group",
			c:      withDates(mxCase{mode: "simple", logic: "OR", filters: []oraFilter{host("WS1"), user("bob")}}),
			sqlite: mxExp("E03 E04 E07 E08", "")},
		{id: "G12", label: "import_id = 1 (simple)", why: "import_id is incompatible with notes",
			c: mxCase{mode: "simple", logic: "AND", filters: []oraFilter{{"import_id", "=", "1"}}}, sqlite: mxExp("E01 E08 E12", "")},
		{id: "G13", label: "tag = 'lateral' (simple)", why: "tag is compatible; N1 carries 'lateral'",
			c: simple("AND", oraFilter{"tag", "=", "lateral"}), sqlite: mxExp("E04 E06", "N1")},
		{id: "G14", label: "log2timeline.host = 'WS1' (advanced)", why: "the derived table is aliased log2timeline, so a qualified column resolves; same rows as A3",
			c: adv(mxQualified{oraCmpExpr{"host", "=", "WS1"}}), sqlite: mxExp("E01 E02 E03 E07 E08 E12", "")},
		{id: "G15", label: "keyword 'payload' (simple)", why: "only E10's URL contains payload, so keyword search covers URL",
			c: mxCase{mode: "simple", logic: "AND", search: "payload"}, sqlite: mxExp("E10", "")},
		{id: "G16", label: "keyword 'payload', OR mode, host = WS1 (simple)",
			why: "the keyword is ANDed outside the one-filter group: E10 has host WS2, no WS1 row contains payload, and notes have a NULL host",
			c:   mxCase{mode: "simple", logic: "OR", search: "payload", filters: []oraFilter{host("WS1")}}, sqlite: mxExp("", "")},
		{id: "G17", label: "desc LIKE clean (simple)",
			why: "E11 alone sets an hourly window; N4 (2026-01-10) is in the grid but outside the window, so the histogram has no notes",
			c:   simple("AND", oraFilter{"desc", "LIKE", "clean"}), sqlite: mxExp("E11", "N4"),
			hist: "[2026-01-05 10:00:00:1/0]"},
		{id: "G18", label: "desc = 'phone call with HR' (simple)",
			why: "no evidence matches, so N6's own range is the window: one hourly bucket with notes only",
			c:   simple("AND", oraFilter{"desc", "=", "phone call with HR"}), sqlite: mxExp("", "N6"),
			hist: "[2025-06-30 23:00:00:0/1]"},
		{id: "G19", label: "desc LIKE beacon, date range (simple)",
			why: "the explicit range sets a daily window, so N1 (2025-03-01) shows beside E05 and N5; PostgreSQL matches only N1",
			c:   withDates(simple("AND", oraFilter{"desc", "LIKE", "beacon"})), sqlite: mxExp("E05", "N1 N5"), pg: pgOf("", "N1"),
			hist: "[2025-03-01:0/1 2025-03-02:1/1]", pgHist: "[2025-03-01:0/1]"},
		{id: "G20", label: "date range 2025-06-30 to 2025-06-30 (simple)",
			why:    "N7 was stored as a pasted 2025-06-30T18:00:00Z and must read as 2025-06-30 18:00:00, inside a range that ends on its own day",
			c:      mxCase{mode: "simple", logic: "AND", dateFrom: "2025-06-30", dateTo: "2025-06-30"},
			sqlite: mxExp("E06 E07 E14", "N6 N7"),
			hist:   "[2025-06-30 10:00:00:1/0 2025-06-30 12:00:00:1/0 2025-06-30 18:00:00:0/1 2025-06-30 23:00:00:1/1]"},
	}
}

// mxQualified is a comparison written with the column qualified by the table
// name (log2timeline.host). It evaluates exactly like the unqualified form.
type mxQualified struct {
	oraCmpExpr
}

func (q mxQualified) sql() string { return "log2timeline." + q.oraCmpExpr.sql() }

// mxGenerate returns the deterministic generated cases. Simple singles cover
// every term across every base, search, bookmark, and date value. Pairs cover
// every pair under AND and OR, with the other dimensions rotated. Advanced
// cases cover atomic terms with and without NOT, pairs under AND and OR, and
// nested NOT forms over the first 30 atomic terms.
func mxGenerate() []mxCase {
	var out []mxCase
	terms := oraSimpleCatalog()
	bases := mxBaseChoices()
	searches := []string{"", "beacon", "BEACON"}

	n := 0
	for _, t := range terms {
		for _, b := range bases {
			for _, s := range searches {
				for _, bm := range []bool{false, true} {
					for _, dt := range []bool{false, true} {
						n++
						c := mxCase{id: fmt.Sprintf("sim1-%05d", n), mode: "simple", logic: "AND", base: b,
							filters: []oraFilter{t}, search: s, bookmarkOnly: bm}
						if dt {
							c.dateFrom, c.dateTo = "2025-03-01", "2025-06-30"
						}
						out = append(out, c)
					}
				}
			}
		}
	}

	pairOrd := 0
	for i := range terms {
		for j := i + 1; j < len(terms); j++ {
			for li, logic := range []string{"AND", "OR"} {
				idx := pairOrd*2 + li
				c := mxCase{id: fmt.Sprintf("sim2-%05d", idx+1), mode: "simple", logic: logic,
					base: bases[idx%len(bases)], filters: []oraFilter{terms[i], terms[j]},
					search: searches[(idx/6)%len(searches)], bookmarkOnly: (idx/18)%2 == 1}
				if (idx/36)%2 == 1 {
					c.dateFrom, c.dateTo = "2025-03-01", "2025-06-30"
				}
				out = append(out, c)
			}
			pairOrd++
		}
	}

	atoms := mxAtomics()
	n = 0
	for _, a := range atoms {
		for _, b := range bases {
			for _, neg := range []bool{false, true} {
				n++
				e := a
				if neg {
					e = oraNotExpr{a}
				}
				out = append(out, mxCase{id: fmt.Sprintf("adv1-%05d", n), mode: "advanced", base: b, expr: e})
			}
		}
	}

	pairOrd = 0
	for i := range atoms {
		for j := i + 1; j < len(atoms); j++ {
			for li, and := range []bool{true, false} {
				idx := pairOrd*2 + li
				out = append(out, mxCase{id: fmt.Sprintf("adv2-%05d", idx+1), mode: "advanced",
					base: bases[idx%len(bases)], expr: oraBoolExpr{and: and, left: atoms[i], right: atoms[j]}})
			}
			pairOrd++
		}
	}

	forms := []func(a, b oraExpr) oraExpr{
		func(a, b oraExpr) oraExpr { return oraBoolExpr{and: true, left: oraNotExpr{a}, right: b} },
		func(a, b oraExpr) oraExpr { return oraNotExpr{oraBoolExpr{and: false, left: a, right: b}} },
		func(a, b oraExpr) oraExpr { return oraBoolExpr{and: false, left: a, right: oraNotExpr{b}} },
	}
	nested := atoms[:30]
	ord := 0
	for i := range nested {
		for j := i + 1; j < len(nested); j++ {
			for f, form := range forms {
				out = append(out, mxCase{id: fmt.Sprintf("adv3-%05d", ord*3+f+1), mode: "advanced",
					base: bases[(ord*3+f)%len(bases)], expr: form(nested[i], nested[j])})
			}
			ord++
		}
	}
	return out
}

// mxBaseChoices are the tab bases the generator rotates through.
func mxBaseChoices() []oraBase {
	return []oraBase{
		{},
		{field: "source", value: "FILE"},
		{field: "source", value: "EXAMINER"},
		{field: "desc", value: "beacon to c2"},
		{field: "host", value: "WS1"},
		{field: "sourcetype", value: "Examiner Note"},
	}
}

// mxAtomics are the advanced-mode atomic conditions: the simple catalog with
// LIKE patterns wrapped as typed, plus IN and BETWEEN forms.
func mxAtomics() []oraExpr {
	var out []oraExpr
	for _, t := range oraSimpleCatalog() {
		v := t.value
		if t.op == "LIKE" || t.op == "NOT LIKE" {
			v = "%" + v + "%"
		}
		out = append(out, oraCmpExpr{t.field, t.op, v})
	}
	out = append(out,
		oraInExpr{"desc", []string{"beacon to c2", "BEACON out"}},
		oraInExpr{"sourcetype", []string{"Examiner Note", "WinEVTX"}},
		oraInExpr{"host", []string{"WS1", "WS2"}},
		oraInExpr{"import_id", []string{"1", "2"}},
		oraBetweenExpr{"datetime", "2025-03-01 00:00:00", "2025-06-30 23:59:59"},
	)
	return out
}

// mxSimpleRequest builds the request the frontend sends for a simple-mode case.
// The date range goes in DateFrom and DateTo, which the app ANDs outside the
// filter group.
func mxSimpleRequest(c mxCase) QueryRequest {
	req := QueryRequest{
		Logic:        c.logic,
		OrderBy:      "datetime",
		Page:         1,
		PageSize:     mxPageSize,
		SearchText:   c.search,
		SearchMode:   "simple",
		BookmarkOnly: c.bookmarkOnly,
		BaseField:    c.base.field,
		BaseOp:       "=",
		BaseValue:    c.base.value,
	}
	if req.Logic == "" {
		req.Logic = "AND"
	}
	for _, f := range c.filters {
		req.Filters = append(req.Filters, FilterItem{Field: f.field, Operator: f.op, Value: f.value})
	}
	req.DateFrom, req.DateTo = c.dateFrom, c.dateTo
	return req
}

// mxAdvancedGridClause builds the clause the frontend sends to AdvancedSearch:
// the base prefix, then the typed clause in parentheses.
func mxAdvancedGridClause(c mxCase) string {
	clause := c.expr.sql()
	if c.base.field != "" {
		clause = c.base.field + " = " + oraAdvSQL(c.base.field, c.base.value) + " AND (" + clause + ")"
	}
	return clause
}

// mxExportRequest is the request ExportCSV receives. Advanced exports carry the
// typed clause and the base fields separately, as the frontend sends them.
func mxExportRequest(c mxCase) QueryRequest {
	if c.mode == "advanced" {
		return QueryRequest{SearchMode: "advanced", SearchText: c.expr.sql(),
			BaseField: c.base.field, BaseOp: "=", BaseValue: c.base.value}
	}
	return mxSimpleRequest(c)
}

// mxAppOut is what the app returned for one case, with IDs mapped to fixture keys.
type mxAppOut struct {
	ev, notes []string
	unknown   []int64
	total     int64
	gridErr   error
	export    []string
	exportErr error
	// nonRFC3339 counts grid and export datetimes not in the display form, and
	// nonRFCExample is the first one.
	nonRFC3339    int
	nonRFCExample string
	hist          map[string]mxHistBucket
	histErr       error
}

// mxRunApp runs the grid (all pages, with its count) and the CSV export for one case.
func mxRunApp(app *App, ids mxIDs, c mxCase, exportPath string) mxAppOut {
	var out mxAppOut
	events, total, err := mxFetchGrid(app, c)
	out.gridErr = err
	out.total = total
	for _, e := range events {
		if !mxRFC3339.MatchString(e.Datetime) {
			out.nonRFC3339++
			if out.nonRFCExample == "" {
				out.nonRFCExample = fmt.Sprintf("grid %q (row %d)", e.Datetime, e.ID)
			}
		}
		if e.ID > 0 {
			if k, ok := ids.evidence[e.ID]; ok {
				out.ev = append(out.ev, k)
			} else {
				out.unknown = append(out.unknown, e.ID)
			}
		} else if k, ok := ids.notes[e.ID]; ok {
			out.notes = append(out.notes, k)
		} else {
			out.unknown = append(out.unknown, e.ID)
		}
	}
	var exportNonRFC int
	var exportExample string
	out.export, exportNonRFC, exportExample, out.exportErr = mxExport(app, c, exportPath)
	out.nonRFC3339 += exportNonRFC
	if out.nonRFCExample == "" && exportExample != "" {
		out.nonRFCExample = "export " + exportExample
	}
	out.hist, out.histErr = mxRunHistogram(app, c)
	return out
}

func mxFetchGrid(app *App, c mxCase) ([]*model.Event, int64, error) {
	var all []*model.Event
	for page := 1; page <= mxMaxPages; page++ {
		var res *QueryResponse
		var err error
		if c.mode == "advanced" {
			res, err = app.AdvancedSearch(mxAdvancedGridClause(c), page, mxPageSize)
		} else {
			req := mxSimpleRequest(c)
			req.Page = page
			res, err = app.QueryEvents(req)
		}
		if err != nil {
			return nil, 0, err
		}
		all = append(all, res.Events...)
		if len(res.Events) < mxPageSize {
			return all, res.TotalCount, nil
		}
	}
	return nil, 0, fmt.Errorf("more than %d pages", mxMaxPages)
}

// mxRFC3339 is the display form of every fixture datetime, which has whole
// seconds and no offset.
var mxRFC3339 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

// mxExport runs the CSV export and returns sorted datetime, source, desc lines,
// the number of datetimes not in the RFC 3339 display form, and the first one.
func mxExport(app *App, c mxCase, path string) ([]string, int, string, error) {
	if _, err := app.exportCSVTo(path, mxExportRequest(c), func(string) {}); err != nil {
		return nil, 0, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, "", fmt.Errorf("opening export: %w", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, 0, "", fmt.Errorf("reading export: %w", err)
	}
	if len(rows) == 0 {
		return nil, 0, "", fmt.Errorf("export is empty")
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[h] = i
	}
	di, okD := idx["datetime"]
	si, okS := idx["source"]
	ci, okC := idx["desc"]
	if !okD || !okS || !okC {
		return nil, 0, "", fmt.Errorf("export header lacks datetime, source, or desc: %v", rows[0])
	}
	var lines []string
	nonRFC := 0
	example := ""
	for i, r := range rows[1:] {
		if !mxRFC3339.MatchString(r[di]) {
			nonRFC++
			if example == "" {
				example = fmt.Sprintf("%q (data row %d)", r[di], i+1)
			}
		}
		lines = append(lines, mxCanonicalTime(r[di])+"\x1f"+r[si]+"\x1f"+r[ci])
	}
	sort.Strings(lines)
	return lines, nonRFC, example, nil
}

// mxDiff returns the multiset difference between want and got, sorted.
func mxDiff(want, got []string) (missing, extra []string) {
	counts := map[string]int{}
	for _, w := range want {
		counts[w]++
	}
	for _, g := range got {
		counts[g]--
	}
	for k, n := range counts {
		for ; n > 0; n-- {
			missing = append(missing, k)
		}
		for ; n < 0; n++ {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

// mxCompare lists the ways the app's output differs from the expected result.
func mxCompare(want oraResult, got mxAppOut) (issues, detail []string) {
	if got.gridErr != nil {
		issues = append(issues, "grid error")
		detail = append(detail, "grid error: "+got.gridErr.Error())
	} else {
		if miss, extra := mxDiff(want.evidence, got.ev); len(miss) > 0 || len(extra) > 0 {
			if len(miss) > 0 {
				issues = append(issues, "evidence missing")
				detail = append(detail, "evidence missing "+strings.Join(miss, " "))
			}
			if len(extra) > 0 {
				issues = append(issues, "evidence extra")
				detail = append(detail, "evidence extra "+strings.Join(extra, " "))
			}
		}
		if miss, extra := mxDiff(want.notes, got.notes); len(miss) > 0 || len(extra) > 0 {
			if len(miss) > 0 {
				issues = append(issues, "notes missing")
				detail = append(detail, "notes missing "+strings.Join(miss, " "))
			}
			if len(extra) > 0 {
				issues = append(issues, "notes extra")
				detail = append(detail, "notes extra "+strings.Join(extra, " "))
			}
		}
		if len(got.unknown) > 0 {
			issues = append(issues, "unknown id")
			detail = append(detail, fmt.Sprintf("unknown IDs %v", got.unknown))
		}
		wantTotal := int64(len(want.evidence) + len(want.notes))
		if got.total != wantTotal {
			issues = append(issues, "count differs")
			detail = append(detail, fmt.Sprintf("count %d, want %d", got.total, wantTotal))
		}
	}
	if got.exportErr != nil {
		issues = append(issues, "export error")
		detail = append(detail, "export error: "+got.exportErr.Error())
	} else if miss, extra := mxDiff(want.export, got.export); len(miss) > 0 || len(extra) > 0 {
		issues = append(issues, "export differs")
		detail = append(detail, fmt.Sprintf("export: %d rows missing (first %s), %d extra (first %s)",
			len(miss), mxFirstLine(miss), len(extra), mxFirstLine(extra)))
	}
	return issues, detail
}

func mxFirstLine(lines []string) string {
	if len(lines) == 0 {
		return "none"
	}
	return strings.ReplaceAll(lines[0], "\x1f", " | ")
}

// mxFailureKinds are the issue names mxCompare, mxCompareHistogram, and the
// datetime form check report, in the order a group label lists them.
var mxFailureKinds = []string{
	"notes missing", "notes extra", "evidence missing", "evidence extra",
	"count differs", "export differs", "histogram notes differ", "histogram evidence differ",
	"histogram buckets differ", "non-RFC 3339 datetime",
	"grid error", "export error", "histogram error", "unknown id",
}

// mxCaseFields returns the sorted set of fields a case references: filter
// terms, the advanced clause, and the tab base, plus the simple-mode toggles
// as the pseudo-fields keyword, bookmark-only, and date-range.
func mxCaseFields(c mxCase) []string {
	set := map[string]bool{}
	if c.base.field != "" {
		set[c.base.field] = true
	}
	if c.mode == "advanced" {
		if c.expr != nil {
			mxExprFields(c.expr, set)
		}
	} else {
		for _, f := range c.filters {
			set[f.field] = true
		}
		if c.search != "" {
			set["keyword"] = true
		}
		if c.bookmarkOnly {
			set["bookmark-only"] = true
		}
		if c.dateFrom != "" || c.dateTo != "" {
			set["date-range"] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// mxExprFields adds every field an advanced clause names to set.
func mxExprFields(e oraExpr, set map[string]bool) {
	switch x := e.(type) {
	case oraCmpExpr:
		set[x.field] = true
	case mxQualified:
		set[x.field] = true
	case oraInExpr:
		set[x.field] = true
	case oraBetweenExpr:
		set[x.field] = true
	case oraNotExpr:
		mxExprFields(x.inner, set)
	case oraBoolExpr:
		mxExprFields(x.left, set)
		mxExprFields(x.right, set)
	default:
		set[fmt.Sprintf("unknown expression %T", e)] = true
	}
}

// mxGroupKey is the group a failing case falls in, built only from observable
// facts: its mode, its failure kinds in mxFailureKinds order, and the fields it
// references. An issue outside mxFailureKinds is listed after them, so it is
// never dropped.
func mxGroupKey(c mxCase, issues []string) string {
	fields := strings.Join(mxCaseFields(c), ", ")
	if fields == "" {
		fields = "none"
	}
	return fmt.Sprintf("%s | %s | fields: %s", c.mode, strings.Join(mxFailureKindsOf(issues), ", "), fields)
}

// mxFailureKindsOf returns the distinct issues in mxFailureKinds order, then
// any issue outside mxFailureKinds, sorted.
func mxFailureKindsOf(issues []string) []string {
	seen := map[string]bool{}
	for _, is := range issues {
		seen[is] = true
	}
	var kinds []string
	for _, k := range mxFailureKinds {
		if seen[k] {
			kinds = append(kinds, k)
			delete(seen, k)
		}
	}
	var other []string
	for k := range seen {
		other = append(other, k)
	}
	sort.Strings(other)
	return append(kinds, other...)
}

// mxFrequency lists how many of total failing cases have each name, largest
// first (ties by name), as "count  percent  name". A name every failing case
// has is marked "common to every failure". limit caps the lines; 0 lists all.
func mxFrequency(counts map[string]int, total, limit int) []string {
	names := make([]string, 0, len(counts))
	for k := range counts {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}
	lines := make([]string, len(names))
	for i, k := range names {
		n := counts[k]
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(n) / float64(total)
		}
		line := fmt.Sprintf("%6d  %5.1f%%  %s", n, pct, k)
		if total > 0 && n == total {
			line += "  (common to every failure)"
		}
		lines[i] = line
	}
	return lines
}

// TestMxFrequency checks the frequency summary on synthetic failures: order,
// percentages, the limit, and the common-to-every-failure marker.
func TestMxFrequency(t *testing.T) {
	failing := []struct {
		c      mxCase
		issues []string
	}{
		{mxCase{mode: "simple", filters: []oraFilter{{"sourcetype", "=", "Examiner Note"}}, search: "beacon"}, []string{"notes missing", "count differs"}},
		{mxCase{mode: "simple", filters: []oraFilter{{"sourcetype", "!=", "WinEVTX"}, {"color", "=", "RED"}}, bookmarkOnly: true}, []string{"notes extra", "count differs"}},
		{mxCase{mode: "advanced", base: oraBase{field: "sourcetype", value: "Examiner Note"}, expr: oraCmpExpr{"color", "=", "RED"}}, []string{"notes missing", "count differs", "notes missing"}},
		{mxCase{mode: "advanced", expr: oraNotExpr{oraCmpExpr{"sourcetype", "=", "Examiner Note"}}}, []string{"notes extra", "count differs", "export differs"}},
	}
	fields, kinds := map[string]int{}, map[string]int{}
	for _, f := range failing {
		mxCountFailure(fields, kinds, f.c, f.issues)
	}
	check := func(label string, got, want []string) {
		t.Helper()
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s:\n got:\n%s\n want:\n%s", label, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	check("fields", mxFrequency(fields, len(failing), 0), []string{
		"     4  100.0%  sourcetype  (common to every failure)",
		"     2   50.0%  color",
		"     1   25.0%  bookmark-only",
		"     1   25.0%  keyword",
	})
	check("fields, limit 2", mxFrequency(fields, len(failing), 2), []string{
		"     4  100.0%  sourcetype  (common to every failure)",
		"     2   50.0%  color",
	})
	check("kinds", mxFrequency(kinds, len(failing), 0), []string{
		"     4  100.0%  count differs  (common to every failure)",
		"     2   50.0%  notes extra",
		"     2   50.0%  notes missing",
		"     1   25.0%  export differs",
	})
	check("no failures", mxFrequency(map[string]int{}, 0, 0), []string{})
}

// TestMxGroupKey checks the grouping on synthetic failures: a multi-kind
// failure, simple-mode toggles as pseudo-fields, an advanced clause with a
// qualified column and a base, an issue outside mxFailureKinds, and a case
// with no fields.
func TestMxGroupKey(t *testing.T) {
	cases := []struct {
		label  string
		c      mxCase
		issues []string
		want   string
	}{
		{"multi-kind, listed in canonical order with duplicates dropped",
			mxCase{mode: "simple", logic: "AND", filters: []oraFilter{{"sourcetype", "=", "Examiner Note"}, {"source", "=", "EXAMINER"}}},
			[]string{"histogram notes differ", "count differs", "notes missing", "export differs", "notes missing"},
			"simple | notes missing, count differs, export differs, histogram notes differ | fields: source, sourcetype"},
		{"toggles become pseudo-fields, and the base field is included",
			mxCase{mode: "simple", logic: "OR", base: oraBase{field: "desc", value: "beacon to c2"},
				filters: []oraFilter{{"host", "=", "WS1"}, {"color", "=", "RED"}},
				search:  "beacon", bookmarkOnly: true, dateFrom: "2025-03-01", dateTo: "2025-06-30"},
			[]string{"evidence extra", "non-RFC 3339 datetime"},
			"simple | evidence extra, non-RFC 3339 datetime | fields: bookmark-only, color, date-range, desc, host, keyword"},
		{"advanced fields come from every node of the clause",
			mxCase{mode: "advanced", base: oraBase{field: "sourcetype", value: "Examiner Note"},
				expr: oraBoolExpr{and: false, left: oraNotExpr{mxQualified{oraCmpExpr{"host", "=", "WS1"}}},
					right: oraBoolExpr{and: true, left: oraInExpr{"tag", []string{"a", "b"}}, right: oraBetweenExpr{"datetime", "2025-01-01", "2025-02-01"}}}},
			[]string{"notes extra"},
			"advanced | notes extra | fields: datetime, host, sourcetype, tag"},
		{"an issue outside the known kinds is kept after them",
			mxCase{mode: "advanced", expr: oraCmpExpr{"color", "=", "RED"}},
			[]string{"brand new issue", "grid error"},
			"advanced | grid error, brand new issue | fields: color"},
		{"no fields",
			mxCase{mode: "simple", logic: "AND"},
			[]string{"histogram evidence differ"},
			"simple | histogram evidence differ | fields: none"},
	}
	for _, c := range cases {
		if got := mxGroupKey(c.c, c.issues); got != c.want {
			t.Errorf("%s:\n  got  %q\n  want %q", c.label, got, c.want)
		}
	}
}

// mxCategory is one group of failing cases (mxGroupKey). examples holds the
// first mxMaxExamples of them.
type mxCategory struct {
	count    int
	examples []mxMismatch
}

type mxMismatch struct {
	id, group, detail, where string
}

// mxSummary collects one dialect's run for the report and the test log.
type mxSummary struct {
	dialect           oraDialect
	goldenLines       []string
	goldenDetails     []string
	goldenPass        int
	goldenAppMismatch int
	goldenOracleBug   []string
	goldenEvidence    []string
	genTotal          int
	genMismatch       int
	genEvidence       int
	genHist           int
	goldenHist        []string
	evidenceExamples  []mxMismatch
	genByKind         map[string]int
	categories        map[string]*mxCategory
	// fieldCounts and kindCounts count failing generated cases per field
	// referenced and per failure kind.
	fieldCounts    map[string]int
	kindCounts     map[string]int
	mismatches     []mxMismatch
	nonRFC3339     int
	nonRFCExamples []string
}

// addNonRFC adds one case's non-RFC 3339 datetimes to the summary.
func (s *mxSummary) addNonRFC(id string, got mxAppOut) {
	s.nonRFC3339 += got.nonRFC3339
	if got.nonRFC3339 > 0 && len(s.nonRFCExamples) < mxMaxExamples {
		s.nonRFCExamples = append(s.nonRFCExamples, fmt.Sprintf("%s: %d, first %s", id, got.nonRFC3339, got.nonRFCExample))
	}
}

func mxKind(id string) string {
	return id[:strings.IndexByte(id, '-')]
}

// mxHasEvidenceIssue reports whether any issue names evidence rows.
func mxHasEvidenceIssue(issues []string) bool {
	for _, is := range issues {
		if strings.HasPrefix(is, "evidence") {
			return true
		}
	}
	return false
}

// mxRunDialect builds the fixture, runs the golden cases and (outside -short)
// the generated cases as subtests, fails on every mismatch, and writes the
// report section when the dialect failed or NOTESMATRIX_REPORT=1 is set.
func mxRunDialect(t *testing.T, d oraDialect, store database.Store, driver string) {
	t.Helper()
	app := &App{store: store, driver: driver}
	ids := mxBuildFixture(t, app)
	rows := mxOracleRows()
	exportPath := filepath.Join(t.TempDir(), "matrix_export.csv")

	sum := &mxSummary{dialect: d, genByKind: map[string]int{}, categories: map[string]*mxCategory{},
		fieldCounts: map[string]int{}, kindCounts: map[string]int{}}

	t.Run("golden", func(t *testing.T) {
		mxRunGolden(t, app, ids, rows, exportPath, sum)
	})
	t.Run("generated", func(t *testing.T) {
		if testing.Short() {
			t.Skip("-short: skipping the generated notes matrix cases; golden cases still run")
		}
		mxRunGenerated(t, app, ids, rows, exportPath, sum)
	})

	if t.Failed() || os.Getenv("NOTESMATRIX_REPORT") == "1" {
		if err := mxWriteReport(d, mxRenderSection(sum)); err != nil {
			t.Fatalf("writing report: %v", err)
		}
		t.Logf("full report written to %s", filepath.Join(mxReportDir, mxReportFile))
	}
}

// mxRunGolden runs the hand-written cases. Each golden/oracle disagreement and
// each app mismatch is its own error.
func mxRunGolden(t *testing.T, app *App, ids mxIDs, rows []oraRow, exportPath string, sum *mxSummary) {
	d := sum.dialect
	for _, g := range mxGolden() {
		want := g.sqlite
		if d == oraPostgres && g.pg != nil {
			want = *g.pg
		}
		oracle := oraEvaluate(d, g.c, rows)
		graded := oraResult{evidence: mxSorted(want.ev), notes: mxSorted(want.notes), export: oracle.export}
		got := mxRunApp(app, ids, g.c, exportPath)
		sum.addNonRFC(g.id, got)

		var statuses []string
		if !mxSameSet(oracle.evidence, graded.evidence) || !mxSameSet(oracle.notes, graded.notes) {
			bug := fmt.Sprintf("%s (%s): golden ev [%s] notes [%s], oracle ev [%s] notes [%s]",
				g.id, g.label, strings.Join(graded.evidence, " "), strings.Join(graded.notes, " "),
				strings.Join(oracle.evidence, " "), strings.Join(oracle.notes, " "))
			sum.goldenOracleBug = append(sum.goldenOracleBug, bug)
			statuses = append(statuses, "GOLDEN/ORACLE DISAGREE")
			t.Errorf("golden/oracle disagreement: %s", bug)
		}
		goldenHist := g.hist
		if d == oraPostgres && g.pgHist != "" {
			goldenHist = g.pgHist
		}
		if goldenHist != "" {
			if oh := mxRenderHist(oraHistogram(g.c, graded.evidence, graded.notes)); oh != goldenHist {
				bug := fmt.Sprintf("%s (%s): golden histogram %s, oracle %s", g.id, g.label, goldenHist, oh)
				sum.goldenOracleBug = append(sum.goldenOracleBug, bug)
				statuses = append(statuses, "GOLDEN/ORACLE DISAGREE")
				t.Errorf("golden/oracle disagreement: %s", bug)
			}
		}
		issues, detail := mxCompare(graded, got)
		hIssues, hDetail := mxCompareHistogram(g.c, graded, got.hist, got.histErr, true)
		if goldenHist != "" && got.histErr == nil && mxRenderHist(got.hist) != goldenHist {
			hIssues = append(hIssues, "histogram buckets differ")
			hDetail = append(hDetail, fmt.Sprintf("histogram %s, golden %s", mxRenderHist(got.hist), goldenHist))
		}
		if len(hIssues) > 0 {
			sum.goldenHist = append(sum.goldenHist, g.id)
		}
		issues, detail = append(issues, hIssues...), append(detail, hDetail...)
		issues, detail = mxAddNonRFC(got, issues, detail)
		if len(issues) > 0 {
			sum.goldenAppMismatch++
			statuses = append(statuses, "APP MISMATCH: "+strings.Join(issues, ", "))
			if mxHasEvidenceIssue(issues) {
				sum.goldenEvidence = append(sum.goldenEvidence, g.id)
			}
			entry := fmt.Sprintf("%s %s\n    why: %s\n    %s", g.id, g.label, g.why, strings.Join(detail, "\n    "))
			sum.goldenDetails = append(sum.goldenDetails, entry)
			t.Errorf("%s golden %s", d, entry)
		} else {
			sum.goldenPass++
			statuses = append(statuses, "PASS")
		}
		sum.goldenLines = append(sum.goldenLines, fmt.Sprintf("%-5s | %-44s | exp ev [%s] notes [%s] | app ev [%s] notes [%s] | oracle ev [%s] notes [%s] | %s",
			g.id, g.label,
			strings.Join(graded.evidence, " "), strings.Join(graded.notes, " "),
			strings.Join(mxSorted(got.ev), " "), strings.Join(mxSorted(got.notes), " "),
			strings.Join(oracle.evidence, " "), strings.Join(oracle.notes, " "), strings.Join(statuses, "; ")))
	}
	t.Logf("%s: %d golden cases, %d pass, %d app mismatch, %d golden/oracle disagreements, histogram mismatches %v",
		d, len(sum.goldenLines), sum.goldenPass, sum.goldenAppMismatch, len(sum.goldenOracleBug), sum.goldenHist)
}

// mxRunGenerated runs the generated cases against the oracle. Failing cases
// are grouped by mxGroupKey. The log gets the totals, the field and failure
// kind frequencies, and the largest mxLogGroups groups with mxLogExamples
// examples each, so its size does not grow with the number of groups.
func mxRunGenerated(t *testing.T, app *App, ids mxIDs, rows []oraRow, exportPath string, sum *mxSummary) {
	d := sum.dialect
	cases := mxGenerate()
	sum.genTotal = len(cases)
	for _, c := range cases {
		want := oraEvaluate(d, c, rows)
		got := mxRunApp(app, ids, c, exportPath)
		sum.addNonRFC(c.id, got)
		sum.genByKind[mxKind(c.id)]++
		issues, detail := mxCompare(want, got)
		hIssues, hDetail := mxCompareHistogram(c, want, got.hist, got.histErr, false)
		if len(hIssues) > 0 {
			sum.genHist++
		}
		issues, detail = append(issues, hIssues...), append(detail, hDetail...)
		issues, detail = mxAddNonRFC(got, issues, detail)
		if len(issues) == 0 {
			continue
		}
		sum.genMismatch++
		mxCountFailure(sum.fieldCounts, sum.kindCounts, c, issues)
		key := mxGroupKey(c, issues)
		entry := sum.categories[key]
		if entry == nil {
			entry = &mxCategory{}
			sum.categories[key] = entry
		}
		m := mxMismatch{id: c.id, group: key, detail: strings.Join(detail, "; "), where: oraRenderWhere(c)}
		entry.count++
		if len(entry.examples) < mxMaxExamples {
			entry.examples = append(entry.examples, m)
		}
		sum.mismatches = append(sum.mismatches, m)
		if mxHasEvidenceIssue(issues) {
			sum.genEvidence++
			if len(sum.evidenceExamples) < 15 {
				sum.evidenceExamples = append(sum.evidenceExamples, m)
			}
		}
	}

	totals := fmt.Sprintf("%s: %d generated cases, %d failing, %d groups", d, sum.genTotal, sum.genMismatch, len(sum.categories))
	if sum.genMismatch == 0 {
		t.Log(totals)
		return
	}
	var b strings.Builder
	b.WriteString(totals)
	fmt.Fprintf(&b, "\nFields referenced by failing cases (top %d):", mxLogFields)
	for _, line := range mxFrequency(sum.fieldCounts, sum.genMismatch, mxLogFields) {
		b.WriteString("\n  " + line)
	}
	b.WriteString("\nFailure kinds in failing cases:")
	for _, line := range mxFrequency(sum.kindCounts, sum.genMismatch, 0) {
		b.WriteString("\n  " + line)
	}
	t.Error(b.String())

	keys := mxCategoriesBySize(sum.categories)
	for i, k := range keys {
		if i == mxLogGroups {
			break
		}
		c := sum.categories[k]
		var g strings.Builder
		n := min(len(c.examples), mxLogExamples)
		fmt.Fprintf(&g, "%s: group %d of %d, %d cases: %s (first %d)", d, i+1, len(keys), c.count, k, n)
		for _, m := range c.examples[:n] {
			fmt.Fprintf(&g, "\n  %s: %s\n      where: %s", m.id, m.detail, m.where)
		}
		t.Error(g.String())
	}
	if len(keys) > mxLogGroups {
		rest := 0
		for _, k := range keys[mxLogGroups:] {
			rest += sum.categories[k].count
		}
		t.Errorf("%s: %d more groups (%d cases) in %s", d, len(keys)-mxLogGroups, rest, filepath.Join(mxReportDir, mxReportFile))
	}
}

// mxCountFailure adds one failing case to the field and failure kind counts.
func mxCountFailure(fields, kinds map[string]int, c mxCase, issues []string) {
	for _, f := range mxCaseFields(c) {
		fields[f]++
	}
	for _, k := range mxFailureKindsOf(issues) {
		kinds[k]++
	}
}

// mxAddNonRFC adds the non-RFC 3339 datetime failure kind when the case's grid
// or export returned a datetime in another form.
func mxAddNonRFC(got mxAppOut, issues, detail []string) ([]string, []string) {
	if got.nonRFC3339 == 0 {
		return issues, detail
	}
	return append(issues, "non-RFC 3339 datetime"),
		append(detail, fmt.Sprintf("%d grid and export datetimes not RFC 3339, first %s", got.nonRFC3339, got.nonRFCExample))
}

// mxCategoriesBySize returns the group keys, largest first.
func mxCategoriesBySize(cats map[string]*mxCategory) []string {
	names := make([]string, 0, len(cats))
	for k := range cats {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		ci, cj := cats[names[i]].count, cats[names[j]].count
		if ci != cj {
			return ci > cj
		}
		return names[i] < names[j]
	})
	return names
}

func mxSorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func mxSameSet(a, b []string) bool {
	x, y := mxSorted(a), mxSorted(b)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// mxRenderSection writes one dialect's report section.
func mxRenderSection(s *mxSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "==================== %s ====================\n\n", s.dialect)
	b.WriteString("Examiner notes matrix: app results against the spec oracle. Every mismatch below failed the test.\n")
	fmt.Fprintf(&b, "Fixture: %d evidence rows and %d notes (notes_fixture_test.go), loaded through InsertEvents, AddExaminerNote,\n",
		len(mxEvidenceRows), len(mxNoteRows))
	b.WriteString("  store.InsertExaminerNote (N1 only, for its tag), BulkUpdateColor, and BulkSetBookmark, then read back and checked.\n\n")

	fmt.Fprintf(&b, "Generated cases: %d run, %d failing, %d groups\n", s.genTotal, s.genMismatch, len(s.categories))
	if s.genMismatch > 0 {
		b.WriteString("Fields referenced by failing cases:\n")
		for _, line := range mxFrequency(s.fieldCounts, s.genMismatch, 0) {
			b.WriteString("  " + line + "\n")
		}
		b.WriteString("Failure kinds in failing cases:\n")
		for _, line := range mxFrequency(s.kindCounts, s.genMismatch, 0) {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString("Evidence-side and shared semantics encoded in the oracle (for confirmation):\n")
	for _, line := range mxSemantics() {
		b.WriteString("  - " + line + "\n")
	}
	b.WriteString("\nAmbiguities resolved while encoding the spec:\n")
	for _, line := range mxAmbiguities() {
		b.WriteString("  - " + line + "\n")
	}

	fmt.Fprintf(&b, "\nGolden cases: %d run, %d pass, %d app mismatch, %d golden/oracle disagreement(s)\n",
		len(s.goldenLines), s.goldenPass, s.goldenAppMismatch, len(s.goldenOracleBug))
	b.WriteString("Columns: case | label | expected (from golden) | app grid | oracle | status\n")
	for _, line := range s.goldenLines {
		b.WriteString("  " + line + "\n")
	}
	if len(s.goldenOracleBug) > 0 {
		b.WriteString("\n*** ORACLE BUGS (golden and oracle disagree; fix the oracle first) ***\n")
		for _, bug := range s.goldenOracleBug {
			b.WriteString("  " + bug + "\n")
		}
	}
	if len(s.goldenDetails) > 0 {
		b.WriteString("\nGolden app mismatch details:\n")
		for _, d := range s.goldenDetails {
			b.WriteString("  " + d + "\n")
		}
	}

	fmt.Fprintf(&b, "\nGenerated cases: %d total (simple singles %d, simple pairs %d, advanced singles %d, advanced pairs %d, advanced nested %d)\n",
		s.genTotal, s.genByKind["sim1"], s.genByKind["sim2"], s.genByKind["adv1"], s.genByKind["adv2"], s.genByKind["adv3"])
	fmt.Fprintf(&b, "Generated cases with at least one mismatch: %d (grid, count, and export are checked per case)\n", s.genMismatch)
	fmt.Fprintf(&b, "Generated cases with an evidence-side mismatch: %d\n", s.genEvidence)
	fmt.Fprintf(&b, "Golden cases with an evidence-side mismatch: %d %v\n", len(s.goldenEvidence), s.goldenEvidence)
	fmt.Fprintf(&b, "Histogram mismatches (totals for every case, per-bucket counts for golden cases): %d golden %v, %d generated\n",
		len(s.goldenHist), s.goldenHist, s.genHist)
	fmt.Fprintf(&b, "Grid and export datetimes not in RFC 3339 (YYYY-MM-DDTHH:MM:SSZ): %d\n", s.nonRFC3339)
	for _, ex := range s.nonRFCExamples {
		b.WriteString("  " + ex + "\n")
	}
	if len(s.evidenceExamples) > 0 {
		b.WriteString("\nEvidence-side mismatch examples (first 15 generated):\n")
		for _, m := range s.evidenceExamples {
			fmt.Fprintf(&b, "  %s | %s\n      %s\n      where: %s\n", m.id, m.group, m.detail, m.where)
		}
	}

	fmt.Fprintf(&b, "\nMismatch groups (mode | failure kinds | fields referenced), largest first, up to %d examples each:\n", mxMaxExamples)
	for _, k := range mxCategoriesBySize(s.categories) {
		c := s.categories[k]
		fmt.Fprintf(&b, "  %6d  %s\n", c.count, k)
		for _, m := range c.examples {
			fmt.Fprintf(&b, "      %s: %s\n          where: %s\n", m.id, m.detail, m.where)
		}
	}

	b.WriteString("\nFull mismatch list (case, then group, detail, where):\n")
	for _, m := range s.mismatches {
		fmt.Fprintf(&b, "  %s\n      %s\n      %s\n      where: %s\n", m.id, m.group, m.detail, m.where)
	}
	b.WriteString("\n")
	return b.String()
}

func mxSemantics() []string {
	return []string{
		"Simple filter values: datetime input passes through normalizeDate (end of day for <=). LIKE and NOT LIKE wrap the value as %value%. = != >= <= compare the value as typed.",
		"Integer columns (bookmark, import_id) compare as numbers. import_id is a simple-mode = only (importIDValue, SimpleInt).",
		"Simple filter group: the user's filters combine with AND or OR (the request's logic). An empty group is true.",
		"Keyword search: LIKE '%term%' ORed over desc, filename, source, sourcetype, type, user, host, extra, tag, url, source_name, computer_name, format, notes (QueryEvents). Case folding: ASCII on SQLite, none on PostgreSQL.",
		"Date range: sent as QueryRequest.DateFrom and DateTo, normalized with normalizeDate (DateTo to the end of its period), and ANDed outside the filter group.",
		"Bookmark-only: bookmark = 1 (QueryEvents). Spec: ANDed outside the filter group.",
		"Tab base: field = value, ANDed outside the filter group (spec and SetBase). Source, desc, host, and sourcetype bases are exercised.",
		"Advanced mode: the clause runs verbatim after the base prefix field = 'v' AND (clause), as advancedClauseWithBase builds it. ORDER BY datetime is added by the app. Datetime literals are not normalized.",
		"Advanced clauses use SQL semantics: NOT (unknown) is unknown; IN is an OR of equalities; BETWEEN is inclusive.",
		"Evidence rows: empty text is stored as '' (not NULL); import_id is NULL when 0. Notes: every column except datetime, source, sourcetype, desc, tag, color, and bookmark is NULL.",
		"Export: same filters and base as the grid, ordered by datetime. Compared as a multiset of (datetime, source, desc). Count: TotalCount, compared with the number of returned rows.",
		"Histogram: the same WHERE plus the junk-date guard (datetime > '1970-01-01' AND datetime < '2100-01-01'), evidence and notes counted separately. Totals are checked for every case; golden cases also check per-bucket counts. The window is the explicit date range, else the matching evidence's min/max, else the matching notes' min/max; its endpoints set the format (hourly within a day, daily within a month or year, monthly across years), and notes outside the window are dropped.",
		"Text comparison is byte order (SQLite BINARY). Datetime comparison is text order over the fixed-width form, which matches TIMESTAMP order for these values.",
	}
}

func mxAmbiguities() []string {
	return []string{
		"Note tag and color: AddExaminerNote stores '' for both, so an empty note tag or color is the empty string, not NULL. The oracle treats '' as a value.",
		"Note tags: AddExaminerNote hardcodes an empty tag (app.go:1349-1357). N1's tag went in through store.InsertExaminerNote, the store API that takes a tag. Real notes never get a tag through the App.",
		"Advanced datetimes: the spec says to compare after normalizeDate, but the app never normalizes an advanced clause. The oracle compares the literal as written; A2 and G15-style partial dates show the effect.",
		"Date range under OR: the spec says ANDed. The harness sends the range in DateFrom and DateTo, which the app ANDs; datetime rows in the filter list remain ordinary filters.",
		"Notes sourcetype: the spec says 'Examiner Note', and the projection carries it.",
		"Datetime display: grid and export datetimes are RFC 3339 on both dialects (displayDatetime on SQLite, the driver on PostgreSQL), and any other form fails the test. Row comparisons use the canonical 'YYYY-MM-DD HH:MM:SS' form.",
		"Keyword wildcards: % and _ typed by the user act as LIKE wildcards with no escape, as the spec says. The fixture uses file_name.txt and 50% done to exercise this.",
		"NOT of a comparison with NULL stays unknown, so an advanced NOT over a notes-incompatible field drops notes (A3, G1).",
	}
}

// mxWriteReport rewrites testdata/notesmatrix_report.txt with every dialect run so far.
func mxWriteReport(d oraDialect, section string) error {
	mxReportMu.Lock()
	defer mxReportMu.Unlock()
	mxReportSections[d] = section
	var b strings.Builder
	for _, name := range []oraDialect{oraSQLite, oraPostgres} {
		if s, ok := mxReportSections[name]; ok {
			b.WriteString(s)
		}
	}
	if err := os.MkdirAll(mxReportDir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", mxReportDir, err)
	}
	return os.WriteFile(filepath.Join(mxReportDir, mxReportFile), []byte(b.String()), 0o600)
}

var (
	mxReportMu       sync.Mutex
	mxReportSections = map[oraDialect]string{}
)

// mxDSNWithDatabase returns dsn with its database replaced by name. It accepts
// the libpq keyword form and a postgres:// URL.
func mxDSNWithDatabase(dsn, name string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("parsing TEST_PG_DSN: %w", err)
		}
		u.Path = "/" + name
		return u.String(), nil
	}
	fields := strings.Fields(dsn)
	found := false
	for i, f := range fields {
		if strings.HasPrefix(f, "dbname=") {
			fields[i] = "dbname=" + name
			found = true
		}
	}
	if !found {
		fields = append(fields, "dbname="+name)
	}
	return strings.Join(fields, " "), nil
}

// mxPGName returns a database name of the form fntest_<unix time>_<random>.
func mxPGName() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating database name: %w", err)
	}
	return fmt.Sprintf("fntest_%d_%s", time.Now().Unix(), hex.EncodeToString(b)), nil
}

func mxQuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// mxLeftoverAge is how old a fntest_ database must be before a run treats it
// as a leftover. Runs can overlap (two people in one container), so a younger
// database may belong to a run still in progress.
const mxLeftoverAge = time.Hour

// mxPGNameRe matches the names mxPGName generates.
var mxPGNameRe = regexp.MustCompile(`^fntest_(\d+)_[0-9a-f]{8}$`)

// mxPGNameTime returns the creation time embedded in a mxPGName name. ok is
// false when the name does not have that form.
func mxPGNameTime(name string) (time.Time, bool) {
	m := mxPGNameRe.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}

// mxIsStaleLeftover reports whether name is a mxPGName database created more
// than mxLeftoverAge before now. parsed is false for a name that does not
// parse, which is never stale.
func mxIsStaleLeftover(name string, now time.Time) (stale, parsed bool) {
	created, ok := mxPGNameTime(name)
	if !ok {
		return false, false
	}
	return now.Sub(created) > mxLeftoverAge, true
}

// mxDropLeftovers drops fntest_% databases the current role owns that an
// earlier run left behind: only those whose embedded time is more than
// mxLeftoverAge old, so a concurrent run keeps its database. Names that do not
// parse are left alone and logged.
func mxDropLeftovers(t *testing.T, admin *sql.DB) {
	t.Helper()
	rows, err := admin.Query(`SELECT datname FROM pg_database WHERE datname LIKE 'fntest\_%' AND pg_get_userbyid(datdba) = current_user`)
	if err != nil {
		t.Fatalf("listing leftover fntest databases: %v", err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			t.Fatalf("scanning leftover fntest database name: %v", err)
		}
		names = append(names, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("listing leftover fntest databases: %v", err)
	}
	now := time.Now()
	for _, n := range names {
		stale, parsed := mxIsStaleLeftover(n, now)
		if !parsed {
			t.Logf("leaving database %s: name has no fntest_<unix time>_<random> timestamp", n)
			continue
		}
		if !stale {
			continue
		}
		if _, err := admin.Exec("DROP DATABASE " + mxQuoteIdent(n)); err != nil {
			t.Fatalf("dropping leftover database %s: %v", n, err)
		}
		t.Logf("dropped leftover database %s", n)
	}
}

// TestMxIsStaleLeftover checks the leftover name parsing and the age rule.
func TestMxIsStaleLeftover(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	name := func(sec int64) string { return fmt.Sprintf("fntest_%d_0a1b2c3d", sec) }
	cases := []struct {
		label, name   string
		stale, parsed bool
	}{
		{"two hours old", name(now.Unix() - 7200), true, true},
		{"just over an hour", name(now.Unix() - 3601), true, true},
		{"exactly an hour", name(now.Unix() - 3600), false, true},
		{"fresh", name(now.Unix() - 60), false, true},
		{"from the future", name(now.Unix() + 60), false, true},
		{"no random part", "fntest_1790000000", false, false},
		{"text timestamp", "fntest_abc_0a1b2c3d", false, false},
		{"uppercase hex", "fntest_1790000000_0A1B2C3D", false, false},
		{"short random", "fntest_1790000000_0a1b", false, false},
		{"other prefix", "fntestx_1790000000_0a1b2c3d", false, false},
		{"trailing text", "fntest_1790000000_0a1b2c3d_old", false, false},
		{"overflowing timestamp", "fntest_99999999999999999999_0a1b2c3d", false, false},
	}
	for _, c := range cases {
		stale, parsed := mxIsStaleLeftover(c.name, now)
		if stale != c.stale || parsed != c.parsed {
			t.Errorf("%s: mxIsStaleLeftover(%q) = stale %v, parsed %v; want %v, %v", c.label, c.name, stale, parsed, c.stale, c.parsed)
		}
	}

	generated, err := mxPGName()
	if err != nil {
		t.Fatalf("mxPGName: %v", err)
	}
	if stale, parsed := mxIsStaleLeftover(generated, time.Now()); stale || !parsed {
		t.Errorf("mxPGName() = %q: stale %v, parsed %v; want fresh and parsed", generated, stale, parsed)
	}
}

// TestNotesMatrixSQLite runs the matrix against a fresh SQLite database.
func TestNotesMatrixSQLite(t *testing.T) {
	store, err := database.CreateSQLite(filepath.Join(t.TempDir(), "notesmatrix.db"), nil)
	if err != nil {
		t.Fatalf("CreateSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	mxRunDialect(t, oraSQLite, store, "sqlite")
}

// TestNotesMatrixPostgres runs the matrix against a fresh PostgreSQL database
// created on the server named by TEST_PG_DSN. The database is dropped in
// t.Cleanup. Connection errors are reported as-is and are not retried.
func TestNotesMatrixPostgres(t *testing.T) {
	store, err := database.CreatePostgres(mxCreatePGDatabase(t), nil)
	if err != nil {
		t.Fatalf("CreatePostgres: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	mxRunDialect(t, oraPostgres, store, "postgres")
}

// mxCreatePGDatabase creates a fresh fntest_ database on the server named by
// TEST_PG_DSN, drops it in t.Cleanup, and returns its connection string. It
// skips the test when TEST_PG_DSN is not set.
func mxCreatePGDatabase(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN is not set; skipping the PostgreSQL test")
	}
	maintDSN, err := mxDSNWithDatabase(dsn, "postgres")
	if err != nil {
		t.Fatalf("%v", err)
	}
	admin, err := sql.Open("pgx", maintDSN)
	if err != nil {
		t.Fatalf("opening maintenance connection: %v", err)
	}
	t.Cleanup(func() { admin.Close() })
	if err := admin.Ping(); err != nil {
		t.Fatalf("connecting to maintenance database: %v", err)
	}
	mxDropLeftovers(t, admin)

	name, err := mxPGName()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + mxQuoteIdent(name)); err != nil {
		t.Fatalf("creating database %s: %v", name, err)
	}
	t.Logf("created database %s", name)
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + mxQuoteIdent(name)); err != nil {
			t.Logf("dropping database %s: %v", name, err)
			return
		}
		t.Logf("dropped database %s", name)
	})

	connStr, err := mxDSNWithDatabase(dsn, name)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return connStr
}
