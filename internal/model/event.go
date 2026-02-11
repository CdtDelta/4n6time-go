package model

// Fields is the ordered list of column names in the log2timeline table.
// Used for query building, field validation, and index management.
var Fields = []string{
	"datetime", "timezone", "MACB", "source", "sourcetype", "type",
	"user", "host", "desc", "filename", "inode",
	"notes", "format", "extra", "reportnotes", "inreport",
	"tag", "color", "offset", "store_number", "store_index",
	"vss_store_number", "URL", "record_number", "event_identifier",
	"event_type", "source_name", "user_sid", "computer_name",
}

// Event represents a single timeline event from a Plaso/log2timeline output.
// Field names and structure match the original 4n6time SQLite schema.
type Event struct {
	ID             int64  `json:"id" db:"rowid"`
	Timezone       string `json:"timezone" db:"timezone"`
	MACB           string `json:"macb" db:"MACB"`
	Source         string `json:"source" db:"source"`
	SourceType     string `json:"sourcetype" db:"sourcetype"`
	Type           string `json:"type" db:"type"`
	User           string `json:"user" db:"user"`
	Host           string `json:"host" db:"host"`
	Desc           string `json:"desc" db:"desc"`
	Filename       string `json:"filename" db:"filename"`
	Inode          string `json:"inode" db:"inode"`
	Notes          string `json:"notes" db:"notes"`
	Format         string `json:"format" db:"format"`
	Extra          string `json:"extra" db:"extra"`
	Datetime       string `json:"datetime" db:"datetime"`
	ReportNotes    string `json:"reportnotes" db:"reportnotes"`
	InReport       string `json:"inreport" db:"inreport"`
	Tag            string `json:"tag" db:"tag"`
	Color          string `json:"color" db:"color"`
	Offset         int64  `json:"offset" db:"offset"`
	StoreNumber    int64  `json:"store_number" db:"store_number"`
	StoreIndex     int64  `json:"store_index" db:"store_index"`
	VSSStoreNumber int64  `json:"vss_store_number" db:"vss_store_number"`
	URL            string `json:"url" db:"URL"`
	RecordNumber   string `json:"record_number" db:"record_number"`
	EventID        string `json:"event_identifier" db:"event_identifier"`
	EventType      string `json:"event_type" db:"event_type"`
	SourceName     string `json:"source_name" db:"source_name"`
	UserSID        string `json:"user_sid" db:"user_sid"`
	ComputerName   string `json:"computer_name" db:"computer_name"`
}
