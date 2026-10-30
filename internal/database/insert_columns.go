package database

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/cdtdelta/4n6time/internal/model"
)

// insertEventColumns lists the columns of InsertEventSQL in parameter order,
// unquoted. TestInsertEventColumnsMatchSQL parses both dialects' InsertEventSQL
// and fails if this list drifts from either one, and
// TestPgInsertEventArgsOrder checks pgInsertEventArgs against it.
var insertEventColumns = []string{
	"timezone", "MACB", "source", "sourcetype", "type", "user", "host", "desc", "filename",
	"inode", "notes", "format", "extra", "datetime", "reportnotes", "inreport", "tag", "color",
	"offset", "store_number", "store_index", "vss_store_number", "URL", "record_number",
	"event_identifier", "event_type", "source_name", "user_sid", "computer_name", "bookmark", "import_id",
}

// pgInsertEventArgs returns e's InsertEventSQL arguments for PostgreSQL, in
// insertEventColumns order, sanitized for PostgreSQL.
func pgInsertEventArgs(e *model.Event) []interface{} {
	return []interface{}{
		pgSanitizeString(e.Timezone), pgSanitizeString(e.MACB),
		pgSanitizeString(e.Source), pgSanitizeString(e.SourceType), pgSanitizeString(e.Type),
		pgSanitizeString(e.User), pgSanitizeString(e.Host), pgSanitizeString(e.Desc),
		pgSanitizeString(e.Filename), pgSanitizeString(e.Inode),
		pgSanitizeString(e.Notes), pgSanitizeString(e.Format), pgSanitizeString(e.Extra),
		pgSanitizeDatetime(e.Datetime), pgSanitizeString(e.ReportNotes),
		pgSanitizeString(e.InReport), pgSanitizeString(e.Tag), pgSanitizeString(e.Color),
		e.Offset, e.StoreNumber,
		e.StoreIndex, e.VSSStoreNumber, pgSanitizeString(e.URL),
		pgSanitizeString(e.RecordNumber),
		pgSanitizeString(e.EventID), pgSanitizeString(e.EventType),
		pgSanitizeString(e.SourceName), pgSanitizeString(e.UserSID),
		pgSanitizeString(e.ComputerName),
		e.Bookmark, importIDArg(e.ImportID),
	}
}

// encodeArgPattern finds the zero-based parameter index in a pgx encode
// error ("failed to encode args[18]: ..."). pgx builds that text with
// fmt.Errorf and has no typed error to read the index from.
var encodeArgPattern = regexp.MustCompile(`args\[(\d+)\]`)

// maxInsertValueLen caps a string value quoted in an insert error.
const maxInsertValueLen = 80

// wrapInsertError wraps an InsertEvents failure on event number eventNum
// (1-based). When err names a parameter index within args, the column name
// and the value are added, so "failed to encode args[18]" also says
// column "offset" and the offending number. Otherwise only the event number
// is added, as before. The original error is always kept in full.
func wrapInsertError(err error, eventNum int, args []interface{}) error {
	if m := encodeArgPattern.FindStringSubmatch(err.Error()); m != nil {
		idx, convErr := strconv.Atoi(m[1])
		if convErr == nil && idx >= 0 && idx < len(insertEventColumns) && idx < len(args) {
			return fmt.Errorf("inserting event %d: column %q value %s: %w",
				eventNum, insertEventColumns[idx], formatInsertValue(args[idx]), err)
		}
	}
	return fmt.Errorf("inserting event %d: %w", eventNum, err)
}

// formatInsertValue renders an insert argument for an error message: NULL
// for nil, a quoted string cut to maxInsertValueLen bytes, or the value.
func formatInsertValue(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case string:
		if len(x) > maxInsertValueLen {
			cut := maxInsertValueLen
			for cut > 0 && x[cut]&0xC0 == 0x80 {
				cut--
			}
			return strconv.Quote(x[:cut]) + "..."
		}
		return strconv.Quote(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}
