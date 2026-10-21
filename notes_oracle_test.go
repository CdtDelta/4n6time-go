package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The oracle evaluates the examiner notes spec over in-memory rows, without
// calling the query builder or the store. normalizeDate is the one app helper
// it uses, because the spec defines simple-mode datetime input as normalized by
// it. The spec it encodes:
//
//   - Projected notes: each note is a log2timeline row with source EXAMINER,
//     sourcetype Examiner Note, desc from its description, its own datetime,
//     tag, color, and bookmark, and every other column NULL.
//   - Derived table: grid, count, and export read evidence UNION ALL projected
//     notes, and one WHERE applies to both with SQL three-valued logic, so a
//     condition on a column notes leave NULL is unknown for every note.
//   - Histogram (notes_histogram_test.go): the same WHERE plus the junk-date
//     guard, with evidence and notes counted separately; notes count only
//     inside the window that the explicit date range, the matching evidence,
//     or, with no matching evidence, the notes themselves set.
//   - Datetime display: grid and export datetimes are RFC 3339
//     (YYYY-MM-DDTHH:MM:SSZ) on both dialects; the matrix checks the form, and
//     the oracle compares the stored "YYYY-MM-DD HH:MM:SS" text.

// oraDialect selects the SQL dialect rules that change results: SQLite LIKE
// folds ASCII case and PostgreSQL LIKE does not. Both compare = and != exactly.
type oraDialect string

const (
	oraSQLite   oraDialect = "sqlite"
	oraPostgres oraDialect = "postgres"
)

// oraTV is SQL's three-valued logic result. A row is returned only when its
// WHERE evaluates to oraTrue.
type oraTV int

const (
	oraFalse oraTV = iota
	oraUnknown
	oraTrue
)

func oraOf(b bool) oraTV {
	if b {
		return oraTrue
	}
	return oraFalse
}

func oraAnd(a, b oraTV) oraTV {
	if a == oraFalse || b == oraFalse {
		return oraFalse
	}
	if a == oraUnknown || b == oraUnknown {
		return oraUnknown
	}
	return oraTrue
}

func oraOr(a, b oraTV) oraTV {
	if a == oraTrue || b == oraTrue {
		return oraTrue
	}
	if a == oraUnknown || b == oraUnknown {
		return oraUnknown
	}
	return oraFalse
}

func oraNot(a oraTV) oraTV {
	switch a {
	case oraTrue:
		return oraFalse
	case oraFalse:
		return oraTrue
	}
	return oraUnknown
}

// oraRow is one row of the spec's view: an evidence row or a projected note. A
// nil cell is SQL NULL, and a column absent from cells is NULL too.
type oraRow struct {
	key   string
	note  bool
	cells map[string]interface{}
}

// oraKeywordCols are the columns simple keyword search covers. The list is
// copied from QueryEvents, which ORs LIKE '%term%' over these fields.
var oraKeywordCols = []string{
	"desc", "filename", "source", "sourcetype", "type", "user", "host", "extra",
	"tag", "url", "source_name", "computer_name", "format", "notes",
}

// oraIntCols are integer columns. Their literals compare as numbers.
var oraIntCols = map[string]bool{"bookmark": true, "import_id": true}

// oraLit is a comparison operand. text is always set; n is set when num is true.
type oraLit struct {
	text string
	n    int64
	num  bool
}

func oraText(s string) oraLit { return oraLit{text: s} }

func oraNum(s string) (oraLit, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return oraLit{text: s}, false
	}
	return oraLit{text: s, n: n, num: true}, true
}

// oraCmp applies one comparison to one cell. Any comparison with a NULL cell is
// unknown, including != and LIKE.
func oraCmp(d oraDialect, cell interface{}, op string, l oraLit) oraTV {
	switch c := cell.(type) {
	case string:
		return oraCmpText(d, c, op, l.text)
	case int64:
		if l.num {
			switch op {
			case "=":
				return oraOf(c == l.n)
			case "!=":
				return oraOf(c != l.n)
			case ">=":
				return oraOf(c >= l.n)
			case "<=":
				return oraOf(c <= l.n)
			}
		}
		// An integer cell never equals a text literal.
		if !l.num {
			if op == "=" {
				return oraFalse
			}
			if op == "!=" {
				return oraTrue
			}
		}
		return oraCmpText(d, strconv.FormatInt(c, 10), op, l.text)
	}
	return oraUnknown
}

// oraCmpText compares a non-NULL text cell with a literal. Ordering operators
// compare bytes, which matches SQLite BINARY collation and the fixed-width
// datetime text the fixture uses.
func oraCmpText(d oraDialect, c, op, text string) oraTV {
	switch op {
	case "=":
		return oraOf(c == text)
	case "!=":
		return oraOf(c != text)
	case ">=":
		return oraOf(c >= text)
	case "<=":
		return oraOf(c <= text)
	case "LIKE":
		return oraOf(oraLike(text, c, d == oraSQLite))
	case "NOT LIKE":
		return oraOf(!oraLike(text, c, d == oraSQLite))
	}
	panic("oracle: unsupported operator " + op)
}

// oraLike reports whether s matches the SQL LIKE pattern. % matches any run of
// characters and _ matches one character. foldASCII models SQLite, which folds
// ASCII case. No escape character is used, and the fixture has no backslashes.
func oraLike(pattern, s string, foldASCII bool) bool {
	p, t := []byte(pattern), []byte(s)
	if foldASCII {
		p = oraFoldASCII(p)
		t = oraFoldASCII(t)
	}
	// dp[i][j] is true when p[:i] matches t[:j].
	dp := make([][]bool, len(p)+1)
	for i := range dp {
		dp[i] = make([]bool, len(t)+1)
	}
	dp[0][0] = true
	for i := 1; i <= len(p); i++ {
		dp[i][0] = p[i-1] == '%' && dp[i-1][0]
		for j := 1; j <= len(t); j++ {
			switch p[i-1] {
			case '%':
				dp[i][j] = dp[i-1][j] || dp[i][j-1]
			case '_':
				dp[i][j] = dp[i-1][j-1]
			default:
				dp[i][j] = dp[i-1][j-1] && p[i-1] == t[j-1]
			}
		}
	}
	return dp[len(p)][len(t)]
}

func oraFoldASCII(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

// oraFilter is one simple-mode filter as the user enters it.
type oraFilter struct {
	field, op, value string
}

// oraFilterLit is the operand the app compares for a simple-mode filter.
// Datetime input passes through normalizeDate (end of day for <=). LIKE and
// NOT LIKE wrap the value in %...%, as query.Simple does. Integer columns
// compare as numbers.
func oraFilterLit(f oraFilter) oraLit {
	v := f.value
	if f.field == "datetime" {
		v = normalizeDate(v, f.op == "<=")
	}
	if f.op == "LIKE" || f.op == "NOT LIKE" {
		v = "%" + v + "%"
	}
	if oraIntCols[f.field] {
		if l, ok := oraNum(v); ok {
			return l
		}
	}
	return oraText(v)
}

// oraExpr is one node of an advanced-mode clause. sql renders the clause text
// the app receives.
type oraExpr interface {
	truth(d oraDialect, row oraRow) oraTV
	sql() string
}

// oraCmpExpr is one comparison typed in an advanced clause. Its value is the
// literal exactly as written: the app does not normalize advanced-mode datetimes.
type oraCmpExpr struct {
	field, op, value string
}

func (e oraCmpExpr) truth(d oraDialect, row oraRow) oraTV {
	return oraCmp(d, row.cells[e.field], e.op, oraAdvLit(e.field, e.value))
}

func (e oraCmpExpr) sql() string {
	return e.field + " " + e.op + " " + oraAdvSQL(e.field, e.value)
}

// oraInExpr is field IN (v1, v2, ...). Its three-valued result is an OR of
// equality tests.
type oraInExpr struct {
	field  string
	values []string
}

func (e oraInExpr) truth(d oraDialect, row oraRow) oraTV {
	t := oraFalse
	for _, v := range e.values {
		t = oraOr(t, oraCmpExpr{e.field, "=", v}.truth(d, row))
	}
	return t
}

func (e oraInExpr) sql() string {
	parts := make([]string, len(e.values))
	for i, v := range e.values {
		parts[i] = oraAdvSQL(e.field, v)
	}
	return e.field + " IN (" + strings.Join(parts, ", ") + ")"
}

// oraBetweenExpr is field BETWEEN lo AND hi, inclusive on both ends.
type oraBetweenExpr struct {
	field, lo, hi string
}

func (e oraBetweenExpr) truth(d oraDialect, row oraRow) oraTV {
	return oraAnd(
		oraCmpExpr{e.field, ">=", e.lo}.truth(d, row),
		oraCmpExpr{e.field, "<=", e.hi}.truth(d, row),
	)
}

func (e oraBetweenExpr) sql() string {
	return e.field + " BETWEEN " + oraAdvSQL(e.field, e.lo) + " AND " + oraAdvSQL(e.field, e.hi)
}

// oraNotExpr is NOT (inner). NOT of unknown stays unknown.
type oraNotExpr struct {
	inner oraExpr
}

func (e oraNotExpr) truth(d oraDialect, row oraRow) oraTV {
	return oraNot(e.inner.truth(d, row))
}

func (e oraNotExpr) sql() string {
	return "NOT (" + e.inner.sql() + ")"
}

// oraBoolExpr is (left AND right) or (left OR right).
type oraBoolExpr struct {
	and         bool
	left, right oraExpr
}

func (e oraBoolExpr) truth(d oraDialect, row oraRow) oraTV {
	l, r := e.left.truth(d, row), e.right.truth(d, row)
	if e.and {
		return oraAnd(l, r)
	}
	return oraOr(l, r)
}

func (e oraBoolExpr) sql() string {
	op := " OR "
	if e.and {
		op = " AND "
	}
	return "(" + e.left.sql() + op + e.right.sql() + ")"
}

// oraAdvLit is the operand for an advanced-clause literal.
func oraAdvLit(field, value string) oraLit {
	if oraIntCols[field] {
		if l, ok := oraNum(value); ok {
			return l
		}
	}
	return oraText(value)
}

// oraAdvSQL renders a literal as it appears in an advanced clause. Integer
// columns take bare numbers; text is single-quoted with quotes doubled.
func oraAdvSQL(field, value string) string {
	if oraIntCols[field] {
		if _, ok := oraNum(value); ok {
			return value
		}
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// oraBase is the tab base query: field = value, ANDed outside the user's
// filters. An empty field means there is no base.
type oraBase struct {
	field, value string
}

// mxCase is one matrix case. Simple mode uses the filter fields; advanced mode
// uses expr. Both use base.
type mxCase struct {
	id   string
	mode string // "simple" or "advanced"
	base oraBase

	// Simple mode.
	filters      []oraFilter
	logic        string // "AND" or "OR"
	search       string
	bookmarkOnly bool
	dateFrom     string
	dateTo       string

	// Advanced mode.
	expr oraExpr
}

// oraEvalCase is the oracle's WHERE for one case over one row.
func oraEvalCase(d oraDialect, c mxCase, row oraRow) oraTV {
	base := oraTrue
	if c.base.field != "" {
		base = oraCmp(d, row.cells[c.base.field], "=", oraText(c.base.value))
	}
	if c.mode == "advanced" {
		return oraAnd(base, c.expr.truth(d, row))
	}
	return oraAnd(base, oraSimpleBody(d, c, row))
}

// oraSimpleBody is the simple-mode condition without the tab base. The filter
// group follows the request's logic. Keyword search, the bookmark toggle, and
// the date range are ANDed outside that group, as the spec requires.
func oraSimpleBody(d oraDialect, c mxCase, row oraRow) oraTV {
	body := oraTrue
	if len(c.filters) > 0 {
		group := oraTrue
		if c.logic == "OR" {
			group = oraFalse
		}
		for _, f := range c.filters {
			t := oraCmp(d, row.cells[f.field], f.op, oraFilterLit(f))
			if c.logic == "OR" {
				group = oraOr(group, t)
			} else {
				group = oraAnd(group, t)
			}
		}
		body = group
	}
	if c.search != "" {
		kw := oraFalse
		for _, col := range oraKeywordCols {
			kw = oraOr(kw, oraCmp(d, row.cells[col], "LIKE", oraText("%"+c.search+"%")))
		}
		body = oraAnd(body, kw)
	}
	if c.bookmarkOnly {
		body = oraAnd(body, oraCmp(d, row.cells["bookmark"], "=", oraLit{text: "1", n: 1, num: true}))
	}
	if c.dateFrom != "" {
		body = oraAnd(body, oraCmp(d, row.cells["datetime"], ">=", oraText(normalizeDate(c.dateFrom, false))))
		body = oraAnd(body, oraCmp(d, row.cells["datetime"], "<=", oraText(normalizeDate(c.dateTo, true))))
	}
	return body
}

// oraResult is the oracle's expected output for one case. evidence and notes
// hold row keys. export holds one datetime, source, and desc line per returned
// row, with the fields joined by \x1f. All slices are sorted.
type oraResult struct {
	evidence []string
	notes    []string
	export   []string
}

// oraEvaluate returns the expected result for one case over rows.
func oraEvaluate(d oraDialect, c mxCase, rows []oraRow) oraResult {
	var res oraResult
	for _, r := range rows {
		if oraEvalCase(d, c, r) != oraTrue {
			continue
		}
		if r.note {
			res.notes = append(res.notes, r.key)
		} else {
			res.evidence = append(res.evidence, r.key)
		}
		res.export = append(res.export, oraExportLine(r))
	}
	sort.Strings(res.evidence)
	sort.Strings(res.notes)
	sort.Strings(res.export)
	return res
}

// oraExportLine is the export comparison key for one row.
func oraExportLine(r oraRow) string {
	return fmt.Sprint(r.cells["datetime"]) + "\x1f" + fmt.Sprint(r.cells["source"]) + "\x1f" + fmt.Sprint(r.cells["desc"])
}

// oraRenderWhere writes the condition a case stands for. Advanced mode sends
// this clause to the app. Simple mode sends a request, not SQL, so the text
// documents the equivalent condition and is never executed.
func oraRenderWhere(c mxCase) string {
	var parts []string
	if c.base.field != "" {
		parts = append(parts, fmt.Sprintf("%s = '%s'", c.base.field, c.base.value))
	}
	if c.mode == "advanced" {
		parts = append(parts, "("+c.expr.sql()+")")
		return strings.Join(parts, " AND ")
	}
	if len(c.filters) > 0 {
		var terms []string
		for _, f := range c.filters {
			terms = append(terms, fmt.Sprintf("%s %s '%s'", f.field, f.op, f.value))
		}
		sep := " " + c.logic + " "
		parts = append(parts, "("+strings.Join(terms, sep)+")")
	}
	if c.search != "" {
		parts = append(parts, fmt.Sprintf("(keyword LIKE '%%%s%%' over 14 text columns)", c.search))
	}
	if c.bookmarkOnly {
		parts = append(parts, "bookmark = 1")
	}
	if c.dateFrom != "" {
		parts = append(parts, fmt.Sprintf("datetime >= '%s' AND datetime <= '%s'", c.dateFrom, c.dateTo))
	}
	if len(parts) == 0 {
		return "(no condition)"
	}
	return strings.Join(parts, " AND ")
}

// oraSimpleCatalog lists the simple-mode filter terms the generator combines.
// Values are chosen to hit notes only, evidence only, both, and neither, and
// include case variants and LIKE wildcards.
func oraSimpleCatalog() []oraFilter {
	var out []oraFilter
	add := func(field string, ops []string, values ...string) {
		for _, op := range ops {
			for _, v := range values {
				out = append(out, oraFilter{field, op, v})
			}
		}
	}
	all := []string{"=", "!=", "LIKE", "NOT LIKE"}
	add("source", all, "EXAMINER", "examiner", "FILE")
	add("sourcetype", all, "Examiner Note", "WinEVTX", "note")
	add("desc", all, "beacon to c2", "beacon", "50% done", "phone", "zzz")
	add("tag", all, "lateral", "")
	add("color", all, "RED", "PINK")
	add("host", all, "WS1", "WS9")
	add("user", all, "alice")
	add("datetime", []string{"=", "!=", ">=", "<="}, "2025-03-01 09:00:00", "2025-06-30")
	add("bookmark", []string{"=", "!="}, "1")
	add("import_id", []string{"="}, "1", "2")
	return out
}
