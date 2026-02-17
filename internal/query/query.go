package query

import (
	"fmt"
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"
)

// Logic determines how multiple predicates are combined.
type Logic int

const (
	AND Logic = iota
	OR
)

// Operator represents a SQL comparison operator.
type Operator string

const (
	Equal          Operator = "="
	NotEqual       Operator = "!="
	Like           Operator = "LIKE"
	NotLike        Operator = "NOT LIKE"
	GreaterOrEqual Operator = ">="
	LessOrEqual    Operator = "<="
)

// validOperators is the set of allowed operators for validation.
var validOperators = map[Operator]bool{
	Equal: true, NotEqual: true, Like: true, NotLike: true,
	GreaterOrEqual: true, LessOrEqual: true,
}

// Predicate represents a single filter condition or a composite of conditions.
// Predicates use parameterized values to prevent SQL injection.
type Predicate struct {
	kind  predicateKind
	field string
	op    Operator
	value string
	date1 string
	date2 string
	left  *Predicate
	right *Predicate
	logic Logic
}

type predicateKind int

const (
	predNone predicateKind = iota
	predSimple
	predDate
	predComposite
)

// Simple creates a predicate that compares a field to a value.
// Returns nil if the field name is invalid or the operator is unrecognized.
func Simple(field string, op Operator, value string) *Predicate {
	if !isValidField(field) || !validOperators[op] {
		return nil
	}
	return &Predicate{
		kind:  predSimple,
		field: field,
		op:    op,
		value: value,
	}
}

// DateRange creates a predicate filtering events between two datetimes (inclusive).
func DateRange(date1, date2 string) *Predicate {
	return &Predicate{
		kind:  predDate,
		date1: date1,
		date2: date2,
	}
}

// Combine joins multiple predicates with the given logic (AND or OR).
// Returns nil for an empty slice. Returns the single predicate if only one is given.
// Nil predicates in the slice are skipped.
func Combine(preds []*Predicate, logic Logic) *Predicate {
	// Filter out nils
	filtered := make([]*Predicate, 0, len(preds))
	for _, p := range preds {
		if p != nil {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}

	// Build a right-leaning tree, matching the original Python behavior
	result := &Predicate{
		kind:  predComposite,
		left:  filtered[0],
		right: filtered[1],
		logic: logic,
	}

	for i := 2; i < len(filtered); i++ {
		result = &Predicate{
			kind:  predComposite,
			left:  result,
			right: filtered[i],
			logic: logic,
		}
	}

	return result
}

// WhereClause returns the SQL WHERE fragment and its parameter values
// using the default dialect (SQLite).
// For example: "(source = ?)", []interface{}{"FILE"}
func (p *Predicate) WhereClause() (string, []interface{}) {
	sql, args, _ := p.whereClauseWithDialect(DefaultDialect, 1)
	return sql, args
}

// whereClauseWithDialect generates the WHERE fragment using the specified dialect.
// startIdx is the 1-based parameter index for numbered placeholder styles.
// Returns the SQL fragment, parameter values, and the next available parameter index.
func (p *Predicate) whereClauseWithDialect(d QueryDialect, startIdx int) (string, []interface{}, int) {
	if p == nil {
		return "", nil, startIdx
	}

	switch p.kind {
	case predNone:
		return "", nil, startIdx

	case predSimple:
		placeholder := d.Placeholder(startIdx)
		quotedField := d.QuoteColumn(p.field)
		if p.op == Like || p.op == NotLike {
			return fmt.Sprintf("(%s %s %s)", quotedField, p.op, placeholder),
				[]interface{}{"%" + p.value + "%"}, startIdx + 1
		}
		return fmt.Sprintf("(%s %s %s)", quotedField, p.op, placeholder),
			[]interface{}{p.value}, startIdx + 1

	case predDate:
		return d.DateBetweenSQL(startIdx, startIdx+1),
			[]interface{}{p.date1, p.date2}, startIdx + 2

	case predComposite:
		leftSQL, leftArgs, nextIdx := p.left.whereClauseWithDialect(d, startIdx)
		rightSQL, rightArgs, nextIdx2 := p.right.whereClauseWithDialect(d, nextIdx)

		if leftSQL == "" && rightSQL == "" {
			return "", nil, nextIdx2
		}
		if leftSQL == "" {
			return rightSQL, rightArgs, nextIdx2
		}
		if rightSQL == "" {
			return leftSQL, leftArgs, nextIdx
		}

		logicStr := "AND"
		if p.logic == OR {
			logicStr = "OR"
		}

		sql := fmt.Sprintf("(%s %s %s)", leftSQL, logicStr, rightSQL)
		args := append(leftArgs, rightArgs...)
		return sql, args, nextIdx2

	default:
		return "", nil, startIdx
	}
}

// Fields returns the list of field names referenced by this predicate tree.
func (p *Predicate) Fields() []string {
	if p == nil {
		return nil
	}

	switch p.kind {
	case predNone:
		return nil
	case predSimple:
		return []string{p.field}
	case predDate:
		return []string{"datetime"}
	case predComposite:
		seen := make(map[string]bool)
		var result []string
		for _, f := range p.left.Fields() {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
		for _, f := range p.right.Fields() {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
		return result
	default:
		return nil
	}
}

// Query builds a full SELECT statement from predicates, ordering, and pagination.
type Query struct {
	predicates []*Predicate
	logic      Logic
	orderBy    string
	pageSize   int
	page       int
	dialect    QueryDialect
}

// New creates a new Query with the given page size.
// Pass 0 for no pagination.
func New(pageSize int) *Query {
	return &Query{
		logic:    AND,
		pageSize: pageSize,
		page:     1,
		dialect:  DefaultDialect,
	}
}

// SetDialect sets the SQL dialect used for query generation.
func (q *Query) SetDialect(d QueryDialect) {
	q.dialect = d
}

// SetLogic sets how top-level predicates are combined (AND or OR).
func (q *Query) SetLogic(logic Logic) {
	q.logic = logic
}

// AddPredicate appends a predicate to the query. Nil predicates are ignored.
func (q *Query) AddPredicate(p *Predicate) {
	if p != nil {
		q.predicates = append(q.predicates, p)
	}
}

// RemovePredicate removes the first occurrence of a predicate from the query.
func (q *Query) RemovePredicate(p *Predicate) {
	for i, pred := range q.predicates {
		if pred == p {
			q.predicates = append(q.predicates[:i], q.predicates[i+1:]...)
			return
		}
	}
}

// ClearPredicates removes all predicates from the query.
func (q *Query) ClearPredicates() {
	q.predicates = nil
}

// OrderBy sets the column to sort results by.
// Pass an empty string to clear ordering.
// Returns an error if the field name is not valid.
func (q *Query) OrderBy(field string) error {
	if field == "" {
		q.orderBy = ""
		return nil
	}
	if !isValidField(field) && field != q.dialect.IDColumn() {
		return fmt.Errorf("invalid order by field: %s", field)
	}
	q.orderBy = field
	return nil
}

// SetPage sets the current page number (1-based).
func (q *Query) SetPage(page int) {
	if page >= 1 {
		q.page = page
	}
}

// PageNumber returns the current page number (1-based).
func (q *Query) PageNumber() int {
	return q.page
}

// Build generates the full SQL SELECT statement and its parameter values.
// Returns the SQL string and a slice of arguments for parameterized execution.
func (q *Query) Build() (string, []interface{}) {
	idCol := q.dialect.IDColumn()
	quotedFields := make([]string, len(model.Fields))
	for i, f := range model.Fields {
		quotedFields[i] = q.dialect.QuoteColumn(f)
	}
	selectFields := idCol + ", " + strings.Join(quotedFields, ", ")
	sql := "SELECT " + selectFields + " FROM log2timeline"

	var allArgs []interface{}

	// Build WHERE clause from predicates
	if len(q.predicates) > 0 {
		combined := Combine(q.predicates, q.logic)
		if combined != nil {
			whereSQL, whereArgs, _ := combined.whereClauseWithDialect(q.dialect, 1)
			if whereSQL != "" {
				sql += " WHERE " + whereSQL
				allArgs = append(allArgs, whereArgs...)
			}
		}
	}

	// ORDER BY
	if q.orderBy != "" {
		sql += " ORDER BY " + q.dialect.QuoteColumn(q.orderBy)
	}

	// LIMIT / OFFSET for pagination
	if q.pageSize > 0 {
		offset := q.pageSize * (q.page - 1)
		sql += fmt.Sprintf(" LIMIT %d OFFSET %d", q.pageSize, offset)
	}

	return sql, allArgs
}

// BuildCount generates a COUNT query using the same predicates.
func (q *Query) BuildCount() (string, []interface{}) {
	idCol := q.dialect.IDColumn()
	sql := "SELECT COUNT(" + idCol + ") FROM log2timeline"

	var allArgs []interface{}

	if len(q.predicates) > 0 {
		combined := Combine(q.predicates, q.logic)
		if combined != nil {
			whereSQL, whereArgs, _ := combined.whereClauseWithDialect(q.dialect, 1)
			if whereSQL != "" {
				sql += " WHERE " + whereSQL
				allArgs = append(allArgs, whereArgs...)
			}
		}
	}

	return sql, allArgs
}

// PredicateFields returns all field names referenced across all predicates.
func (q *Query) PredicateFields() []string {
	seen := make(map[string]bool)
	var result []string
	for _, p := range q.predicates {
		for _, f := range p.Fields() {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	return result
}

// RawQuery wraps a user-provided SQL WHERE clause for direct execution.
// This is the equivalent of the original SQLQuery class.
type RawQuery struct {
	Query
	rawWhere string
}

// NewRaw creates a query from a raw WHERE clause string.
// The raw clause is used as-is, so the caller is responsible for safety.
// Pagination and ordering still work normally on top of it.
func NewRaw(pageSize int, whereClause string) *RawQuery {
	return &RawQuery{
		Query:    *New(pageSize),
		rawWhere: whereClause,
	}
}

// SetRawWhere updates the raw WHERE clause.
func (rq *RawQuery) SetRawWhere(where string) {
	rq.rawWhere = where
}

// Build generates the SQL using the raw WHERE clause plus ordering and pagination.
func (rq *RawQuery) Build() (string, []interface{}) {
	idCol := rq.dialect.IDColumn()
	quotedFields := make([]string, len(model.Fields))
	for i, f := range model.Fields {
		quotedFields[i] = rq.dialect.QuoteColumn(f)
	}
	selectFields := idCol + ", " + strings.Join(quotedFields, ", ")
	sql := "SELECT " + selectFields + " FROM log2timeline"

	if rq.rawWhere != "" {
		sql += " WHERE " + rq.rawWhere
	}

	if rq.orderBy != "" {
		sql += " ORDER BY " + rq.dialect.QuoteColumn(rq.orderBy)
	}

	if rq.pageSize > 0 {
		offset := rq.pageSize * (rq.page - 1)
		sql += fmt.Sprintf(" LIMIT %d OFFSET %d", rq.pageSize, offset)
	}

	// Raw queries don't use parameterized args for the WHERE clause
	return sql, nil
}

// isValidField checks a field name against the known columns.
func isValidField(name string) bool {
	for _, f := range model.Fields {
		if f == name {
			return true
		}
	}
	return false
}
