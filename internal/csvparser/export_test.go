package csvparser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cdtdelta/4n6time/internal/model"
)

// TestWriteEventsGoldenOutput pins the exact bytes WriteEvents produces for a
// row with commas, quotes, and empty fields. The expected text was written
// from the export format before ExportRow was extracted, so any change to the
// column order or escaping fails here.
func TestWriteEventsGoldenOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "golden.csv")
	events := []*model.Event{{
		Datetime:       "2025-01-15 10:30:00",
		Timezone:       "UTC",
		Source:         "FILE",
		SourceType:     "NTFS,ext",
		Type:           `Last "Written"`,
		Host:           "WS1",
		Desc:           "a, b",
		Filename:       "/t.txt",
		Format:         "mft",
		Color:          "RED",
		StoreNumber:    -1,
		StoreIndex:     -1,
		VSSStoreNumber: -1,
		Bookmark:       1,
	}}

	if err := WriteEvents(path, events); err != nil {
		t.Fatalf("WriteEvents: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := "datetime,timezone,MACB,source,sourcetype,type,user,host,desc,filename,inode,notes,format,extra,reportnotes,inreport,tag,color,offset,store_number,store_index,vss_store_number,bookmark\n" +
		`2025-01-15 10:30:00,UTC,,FILE,"NTFS,ext","Last ""Written""",,WS1,"a, b",/t.txt,,,mft,,,,,RED,0,-1,-1,-1,1` + "\n"
	if string(got) != want {
		t.Errorf("WriteEvents output changed\n got: %q\nwant: %q", got, want)
	}
}

func TestExportHeaderIsCopy(t *testing.T) {
	h := ExportHeader()
	if len(h) != 23 {
		t.Fatalf("ExportHeader has %d columns, want 23", len(h))
	}
	h[0] = "mutated"
	if ExportHeader()[0] != "datetime" {
		t.Error("mutating the returned header changed the package header")
	}
}

func TestExportRowMatchesHeaderWidth(t *testing.T) {
	if got := len(ExportRow(&model.Event{})); got != len(ExportHeader()) {
		t.Errorf("ExportRow has %d values, header has %d columns", got, len(ExportHeader()))
	}
}
