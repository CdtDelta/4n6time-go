package main

import (
	"fmt"
	"strings"

	"github.com/cdtdelta/4n6time/internal/database"
	"github.com/cdtdelta/4n6time/internal/query"
)

// examinerNoteSource is the source every examiner note carries in the UNION
// (the EXAMINER literal in examinerNotesUnionPatternB).
const examinerNoteSource = "EXAMINER"

// noteFieldMap maps the log2timeline fields examiner_notes can satisfy to their
// column. source is handled by noteSourceTruth, and every other field is
// incompatible: a condition on it excludes notes.
var noteFieldMap = map[string]string{
	"datetime": "datetime",
	"bookmark": "bookmark",
	"tag":      "tag",
	"color":    "color",
	"desc":     "description",
}

// examinerCompatible reports whether examiner notes can satisfy a condition on
// field. Only source and the fields in noteFieldMap qualify.
func examinerCompatible(field string) bool {
	if field == "source" {
		return true
	}
	_, ok := noteFieldMap[field]
	return ok
}

// noteSourceTruth evaluates one simple-mode source condition for examiner notes.
//
// Every note's source is the same constant, so a source condition is either true
// for all notes or false for all notes; it never depends on a row. The value is
// compared the way the advanced-mode heuristic compares it (EqualFold). LIKE,
// NOT LIKE, and any other operator fail closed (false): patterns are not
// evaluated here, because LIKE case sensitivity differs between SQLite and
// PostgreSQL.
func noteSourceTruth(op, value string) bool {
	same := strings.EqualFold(value, examinerNoteSource)
	switch op {
	case "=":
		return same
	case "!=":
		return !same
	}
	return false
}

// simpleNotesPlan decides, for a simple-mode request, whether examiner notes are
// excluded and, when they are not, the condition that selects the notes. The
// grid, its count, and the simple-mode export all call it, so they agree.
//
// Source conditions fold to constants (noteSourceTruth):
//   - The tab base is always ANDed. A false source base excludes notes; a true
//     one adds no condition.
//   - Filters under AND: a false source term excludes notes; a true one adds
//     nothing.
//   - Filters under OR: a true source term satisfies the group for every note, so
//     the group adds nothing. Otherwise false source terms drop out, and a group
//     left with no terms excludes notes.
//
// Compatible filter fields (datetime, bookmark, tag, color, desc) become
// conditions on the notes table. Under AND they are ANDed, and under OR they are
// ORed, following the request's logic. Incompatible fields exclude notes, except
// inside an OR group that a true source term already satisfies.
//
// A compatible base field (datetime, bookmark, tag, color, desc) is applied to
// the notes side as an always-ANDed condition, using the same operator and value
// as the evidence side's base. An incompatible base field excludes notes.
//
// startIdx is the first placeholder number, so PostgreSQL $N values follow the
// main query's arguments.
func simpleNotesPlan(req QueryRequest, d query.QueryDialect, startIdx int) (bool, *database.NotesFilter) {
	if req.BaseField != "" {
		switch {
		case req.BaseField == "source":
			if !noteSourceTruth(baseOperator(req.BaseOp), req.BaseValue) {
				return true, nil
			}
		case !examinerCompatible(req.BaseField):
			return true, nil
		}
	}

	orLogic := req.Logic == "OR"

	// Pass 1: decide the outcome from the constants and the incompatible fields,
	// before any placeholder is numbered.
	satisfied, excluded := false, false
	for _, f := range req.Filters {
		if f.Field == "source" {
			truth := noteSourceTruth(f.Operator, f.Value)
			if orLogic && truth {
				satisfied = true
			}
			if !orLogic && !truth {
				excluded = true
			}
			continue
		}
		if !examinerCompatible(f.Field) {
			// An incompatible term excludes notes. Under OR, a satisfied group
			// still overrides it (checked below).
			excluded = true
		}
	}
	if excluded && !(orLogic && satisfied) {
		return true, nil
	}

	var parts []string
	var args []interface{}
	idx := startIdx

	if req.BookmarkOnly {
		parts = append(parts, fmt.Sprintf("bookmark = %s", d.Placeholder(idx)))
		args = append(args, int64(1))
		idx++
	}

	// The tab base, ANDed outside the filter group. It mirrors basePredicate on
	// the evidence side: LIKE wraps the value in %...%, and any other operator is
	// equality. Source bases are folded above and need no condition here.
	if req.BaseField != "" && req.BaseField != "source" {
		col := noteFieldMap[req.BaseField]
		op, val := "=", req.BaseValue
		if req.BaseOp == "LIKE" {
			op, val = "LIKE", "%"+req.BaseValue+"%"
		}
		parts = append(parts, fmt.Sprintf("%s %s %s", col, op, d.Placeholder(idx)))
		args = append(args, val)
		idx++
	}

	// Pass 2: build the compatible conditions. An OR group already satisfied
	// adds nothing, so its placeholders are never numbered.
	var group []string
	var groupArgs []interface{}
	if !(orLogic && satisfied) {
		for _, f := range req.Filters {
			noteCol, ok := noteFieldMap[f.Field]
			if !ok {
				continue
			}
			var op string
			switch f.Operator {
			case "=", "!=", ">=", "<=", "LIKE", "NOT LIKE":
				op = f.Operator
			default:
				continue
			}
			val := f.Value
			if f.Field == "datetime" {
				val = normalizeDate(val, f.Operator == "<=")
			}
			if op == "LIKE" || op == "NOT LIKE" {
				val = "%" + val + "%"
			}
			term := fmt.Sprintf("%s %s %s", noteCol, op, d.Placeholder(idx))
			idx++
			if orLogic {
				group = append(group, term)
				groupArgs = append(groupArgs, val)
			} else {
				parts = append(parts, term)
				args = append(args, val)
			}
		}
		if orLogic {
			if len(group) == 0 {
				// No term can match any note, so the OR group is empty.
				return true, nil
			}
			parts = append(parts, "("+strings.Join(group, " OR ")+")")
			args = append(args, groupArgs...)
		}
	}

	if len(parts) == 0 {
		return false, nil
	}
	return false, &database.NotesFilter{SQL: strings.Join(parts, " AND "), Args: args}
}
