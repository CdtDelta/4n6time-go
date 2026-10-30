package database

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

// insertSQLColumns returns the column list of an InsertEventSQL statement,
// unquoted, and its placeholder count.
func insertSQLColumns(t *testing.T, sql string) ([]string, int) {
	t.Helper()
	open := strings.Index(sql, "log2timeline (")
	values := strings.Index(sql, ") VALUES (")
	if open < 0 || values < 0 {
		t.Fatalf("unexpected InsertEventSQL shape: %s", sql)
	}
	var cols []string
	for _, c := range strings.Split(sql[open+len("log2timeline ("):values], ",") {
		cols = append(cols, strings.Trim(strings.TrimSpace(c), `"`))
	}
	placeholders := len(regexp.MustCompile(`\?|\$\d+`).FindAllString(sql[values:], -1))
	return cols, placeholders
}

func TestInsertEventColumnsMatchSQL(t *testing.T) {
	for _, d := range []Dialect{&SQLiteDialect{}, &PostgresDialect{}} {
		cols, placeholders := insertSQLColumns(t, d.InsertEventSQL())
		if strings.Join(cols, ",") != strings.Join(insertEventColumns, ",") {
			t.Errorf("%T InsertEventSQL columns\n got  %v\n want %v (insertEventColumns)", d, cols, insertEventColumns)
		}
		if placeholders != len(insertEventColumns) {
			t.Errorf("%T InsertEventSQL has %d placeholders, want %d", d, placeholders, len(insertEventColumns))
		}
	}
	if len(insertEventColumns) != len(model.Fields) {
		t.Errorf("insertEventColumns has %d columns, model.Fields has %d", len(insertEventColumns), len(model.Fields))
	}
}

// TestPgInsertEventArgsOrder gives every field a distinct value and checks
// that pgInsertEventArgs puts each one at its column's index.
func TestPgInsertEventArgsOrder(t *testing.T) {
	e := &model.Event{
		Timezone: "tz", MACB: "macb", Source: "src", SourceType: "stype", Type: "typ",
		User: "usr", Host: "hst", Desc: "dsc", Filename: "fname", Inode: "ino",
		Notes: "nts", Format: "fmt", Extra: "xtra", Datetime: "2025-01-10 10:00:00",
		ReportNotes: "rnotes", InReport: "inrep", Tag: "tg", Color: "clr",
		Offset: 5747310864, StoreNumber: 102, StoreIndex: 103, VSSStoreNumber: 104,
		URL: "u", RecordNumber: "recno", EventID: "evid", EventType: "evtype",
		SourceName: "sname", UserSID: "sid", ComputerName: "comp", Bookmark: 1, ImportID: 7,
	}
	want := map[string]string{
		"timezone": "tz", "MACB": "macb", "source": "src", "sourcetype": "stype", "type": "typ",
		"user": "usr", "host": "hst", "desc": "dsc", "filename": "fname", "inode": "ino",
		"notes": "nts", "format": "fmt", "extra": "xtra", "datetime": "2025-01-10 10:00:00",
		"reportnotes": "rnotes", "inreport": "inrep", "tag": "tg", "color": "clr",
		"offset": "5747310864", "store_number": "102", "store_index": "103", "vss_store_number": "104",
		"URL": "u", "record_number": "recno", "event_identifier": "evid", "event_type": "evtype",
		"source_name": "sname", "user_sid": "sid", "computer_name": "comp", "bookmark": "1", "import_id": "7",
	}
	args := pgInsertEventArgs(e)
	if len(args) != len(insertEventColumns) {
		t.Fatalf("pgInsertEventArgs returned %d args, want %d", len(args), len(insertEventColumns))
	}
	for i, col := range insertEventColumns {
		if got := fmt.Sprint(args[i]); got != want[col] {
			t.Errorf("args[%d] (%s) = %q, want %q", i, col, got, want[col])
		}
	}
}

func TestWrapInsertErrorEachColumn(t *testing.T) {
	args := make([]interface{}, len(insertEventColumns))
	for i := range args {
		args[i] = int64(i * 10)
	}
	for i, col := range insertEventColumns {
		orig := fmt.Errorf("failed to encode args[%d]: unable to encode", i)
		err := wrapInsertError(orig, 42, args)
		want := fmt.Sprintf("inserting event 42: column %q value %d: failed to encode args[%d]: unable to encode", col, i*10, i)
		if err.Error() != want {
			t.Errorf("index %d: got %q, want %q", i, err.Error(), want)
		}
		if !errors.Is(err, orig) {
			t.Errorf("index %d: the original error is not wrapped", i)
		}
	}
}

func TestWrapInsertErrorExample(t *testing.T) {
	args := pgInsertEventArgs(&model.Event{Offset: 5747310864})
	orig := errors.New("failed to encode args[18]: unable to encode 5747310864 into binary format for int4 (OID 23): 5747310864 is greater than maximum value for int4")
	want := `inserting event 15017285: column "offset" value 5747310864: ` + orig.Error()
	if got := wrapInsertError(orig, 15017285, args).Error(); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestWrapInsertErrorPassThrough(t *testing.T) {
	args := pgInsertEventArgs(&model.Event{})
	for _, orig := range []error{
		errors.New("duplicate key value violates unique constraint"),
		errors.New("failed to encode args[31]: out of range"),
		errors.New("failed to encode args[99999999999999999999]: overflow"),
	} {
		want := "inserting event 3: " + orig.Error()
		err := wrapInsertError(orig, 3, args)
		if err.Error() != want {
			t.Errorf("got %q, want %q (unchanged apart from the event number)", err.Error(), want)
		}
		if !errors.Is(err, orig) {
			t.Errorf("%q: the original error is not wrapped", orig)
		}
	}
}

func TestFormatInsertValue(t *testing.T) {
	long := strings.Repeat("é", 100)
	cases := []struct {
		v    interface{}
		want string
	}{
		{nil, "NULL"},
		{"WS1", `"WS1"`},
		{int64(5747310864), "5747310864"},
		{long, fmt.Sprintf("%q...", strings.Repeat("é", maxInsertValueLen/2))},
	}
	for _, c := range cases {
		if got := formatInsertValue(c.v); got != c.want {
			t.Errorf("formatInsertValue(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}
