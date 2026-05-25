package eztoolparser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"
)

// Tool type constants for detected EZ Tool CSV formats.
const (
	ToolEvtxECmd                     = "EvtxECmd"
	ToolPECmd                        = "PECmd"
	ToolLECmd                        = "LECmd"
	ToolJLECmdAutomatic              = "JLECmd Automatic"
	ToolJLECmdCustom                 = "JLECmd Custom"
	ToolAmcacheUnassociatedFiles     = "AmcacheParser UnassociatedFileEntries"
	ToolAmcacheAssociatedFileEntries = "AmcacheParser AssociatedFileEntries"
	ToolAmcacheProgramEntries        = "AmcacheParser ProgramEntries"
	ToolAmcacheDeviceContainers      = "AmcacheParser DeviceContainers"
	ToolAmcacheDevicePnps            = "AmcacheParser DevicePnps"
	ToolAmcacheDriveBinaries         = "AmcacheParser DriveBinaries"
	ToolAmcacheDriverPackages        = "AmcacheParser DriverPackages"
	ToolAmcacheShortCuts             = "AmcacheParser ShortCuts"
	ToolSrumECmdAppTimeline          = "SrumECmd AppTimeline"
	ToolSrumECmdEnergyUsage          = "SrumECmd EnergyUsage"
	ToolSrumECmdNetworkConnections   = "SrumECmd NetworkConnections"
	ToolSrumECmdNetworkUsages        = "SrumECmd NetworkUsages"
	ToolSrumECmdPushNotifications    = "SrumECmd PushNotifications"
	ToolSrumECmdVfuprov              = "SrumECmd vfuprov"
	ToolMFTECmdMFT                    = "MFTECmd $MFT"
	ToolMFTECmdJ                      = "MFTECmd $J"
	ToolMFTECmdBoot                   = "MFTECmd $Boot"
	ToolMFTECmdSDS                    = "MFTECmd $SDS"
	ToolSBECmd                        = "SBECmd"
	ToolRBCmd                         = "RBCmd"
	ToolAppCompatCacheParser          = "AppCompatCacheParser"
	ToolSrumECmdAppResourceUseInfo    = "SrumECmd AppResourceUseInfo"
	ToolWxTCmdActivity                = "WxTCmd Activity"
	ToolWxTCmdPackageIDs              = "WxTCmd PackageIDs"
)

// NoTimestampFormats contains tool names for recognized formats that have no
// timestamp columns and therefore produce no timeline events. Files matching
// these formats are skipped with an explanatory message rather than treated
// as unknown.
var NoTimestampFormats = map[string]struct{}{
	ToolMFTECmdBoot:   {},
	ToolMFTECmdSDS:    {},
	ToolWxTCmdPackageIDs: {},
}

// ReadResult contains the outcome of an EZ Tool CSV import operation.
type ReadResult struct {
	Events   []*model.Event
	Count    int
	Excluded int
	Tool     string
}

// DetectTool reads the CSV header of a file and returns the detected EZ Tool
// type name. Returns an empty string with a nil error if the file is a valid
// CSV but not a recognized EZ Tool format. Returns a non-nil error only for
// file open or header read failures.
func DetectTool(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(newBOMStrippingReader(f))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return "", fmt.Errorf("reading header: %w", err)
	}

	header = trimHeader(header)
	return detectTool(header, path), nil
}

// ValidateFile reads the header of a CSV file and returns nil if it is a
// recognized EZ Tool CSV format. Returns an error otherwise.
func ValidateFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(newBOMStrippingReader(f))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	header = trimHeader(header)
	tool := detectTool(header, path)
	if tool == "" {
		return fmt.Errorf("not a recognized EZ Tool CSV (columns: %s)", strings.Join(header, ", "))
	}
	return nil
}

// ReadEvents parses a single EZ Tool CSV file and returns expanded timeline events.
// The onProgress callback is called every 10,000 source rows if non-nil.
func ReadEvents(path string, onProgress func(int)) (*ReadResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(newBOMStrippingReader(f))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	header = trimHeader(header)
	tool := detectTool(header, path)
	if tool == "" {
		return nil, fmt.Errorf("not a recognized EZ Tool CSV")
	}

	colIndex := buildColIndex(header)
	tsColumns := timestampColumnsForTool(tool)

	result := &ReadResult{Tool: tool}
	rowNum := 0

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Excluded++
			continue
		}
		rowNum++

		events := expandRow(tool, row, colIndex, header, tsColumns)
		if len(events) == 0 {
			result.Excluded++
			continue
		}

		result.Events = append(result.Events, events...)
		result.Count += len(events)

		if onProgress != nil && rowNum%10000 == 0 {
			onProgress(result.Count)
		}
	}

	return result, nil
}

// ReadDirectory reads all .csv files in a directory, processes each through
// ReadEvents, and combines results.
func ReadDirectory(dirPath string, onProgress func(int)) (*ReadResult, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	combined := &ReadResult{}
	totalCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
			continue
		}

		path := filepath.Join(dirPath, entry.Name())

		// Wrap progress to accumulate across files
		wrappedProgress := func(n int) {
			if onProgress != nil {
				onProgress(totalCount + n)
			}
		}

		res, err := ReadEvents(path, wrappedProgress)
		if err != nil {
			// Skip files that are not recognized EZ Tool CSVs
			continue
		}

		if combined.Tool == "" {
			combined.Tool = res.Tool
		} else if res.Tool != combined.Tool {
			combined.Tool = "Mixed"
		}

		combined.Events = append(combined.Events, res.Events...)
		combined.Count += res.Count
		combined.Excluded += res.Excluded
		totalCount += res.Count
	}

	if combined.Count == 0 && combined.Excluded == 0 {
		return nil, fmt.Errorf("no recognized EZ Tool CSV files found in directory")
	}

	return combined, nil
}

// bomStrippingReader wraps an io.Reader and strips a leading UTF-8 BOM if present.
type bomStrippingReader struct {
	r       io.Reader
	checked bool
}

func newBOMStrippingReader(r io.Reader) io.Reader {
	return &bomStrippingReader{r: r}
}

func (b *bomStrippingReader) Read(p []byte) (int, error) {
	if b.checked {
		return b.r.Read(p)
	}
	b.checked = true

	// Read enough to check for BOM
	buf := make([]byte, 3)
	n, err := io.ReadFull(b.r, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0, err
	}

	start := 0
	if n >= 3 && buf[0] == 0xEF && buf[1] == 0xBB && buf[2] == 0xBF {
		start = 3
	}

	// Copy remaining BOM-stripped bytes into p
	remaining := buf[start:n]
	copy(p, remaining)
	if len(remaining) >= len(p) {
		return len(p), nil
	}

	// Read more from underlying reader
	nn, err := b.r.Read(p[len(remaining):])
	return len(remaining) + nn, err
}

// trimHeader trims whitespace and carriage returns from header columns.
func trimHeader(header []string) []string {
	for i, col := range header {
		header[i] = strings.TrimSpace(strings.TrimRight(col, "\r"))
	}
	return header
}

// buildColIndex creates a map from column name to index in the row.
func buildColIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[col] = i
	}
	return idx
}

// colVal returns the trimmed value at the given column name, or empty string if not present.
func colVal(row []string, colIndex map[string]int, col string) string {
	i, ok := colIndex[col]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(row[i], "\r"))
}

// detectTool identifies the EZ Tool type from CSV header columns.
// path is used only when two formats share an identical header and the filename
// is the only reliable discriminant (AssociatedFileEntries vs UnassociatedFileEntries).
func detectTool(header []string, path string) string {
	has := make(map[string]bool, len(header))
	for _, col := range header {
		has[col] = true
	}

	// EvtxECmd
	if has["RecordNumber"] && has["EventRecordId"] && has["TimeCreated"] && has["Channel"] {
		return ToolEvtxECmd
	}

	// PECmd
	if has["ExecutableName"] && has["Hash"] && has["RunCount"] && has["LastRun"] {
		return ToolPECmd
	}

	// JLECmd Automatic vs Custom (must be checked before LECmd, since JLECmd
	// CSVs also contain the LECmd signature columns)
	if has["AppId"] && has["AppIdDescription"] {
		if has["DestListVersion"] && has["EntryNumber"] && has["InteractionCount"] {
			return ToolJLECmdAutomatic
		}
		if has["EntryName"] && has["TargetCreated"] && !has["DestListVersion"] {
			return ToolJLECmdCustom
		}
	}

	// LECmd
	if has["SourceFile"] && has["TargetCreated"] && has["TargetModified"] && has["LocalPath"] && has["DriveType"] {
		return ToolLECmd
	}

	// AmcacheParser subtypes
	if has["InstallDateArpLastModified"] {
		return ToolAmcacheProgramEntries
	}
	if has["ApplicationName"] && has["ProgramId"] && has["SHA1"] && has["FileKeyLastWriteTimestamp"] {
		lower := strings.ToLower(filepath.Base(path))
		if !strings.Contains(lower, "unassociated") && strings.Contains(lower, "associatedfile") {
			return ToolAmcacheAssociatedFileEntries
		}
		return ToolAmcacheUnassociatedFiles
	}
	if has["KeyName"] && has["KeyLastWriteTimestamp"] {
		if has["LnkName"] {
			return ToolAmcacheShortCuts
		}
		if has["DriverTimeStamp"] {
			return ToolAmcacheDriveBinaries
		}
		if has["BusReportedDescription"] {
			return ToolAmcacheDevicePnps
		}
		if has["Hwids"] {
			return ToolAmcacheDriverPackages
		}
		if has["DiscoveryMethod"] {
			return ToolAmcacheDeviceContainers
		}
	}

	// SrumECmd (all types share base columns)
	if has["Id"] && has["Timestamp"] && has["ExeInfo"] && has["SidType"] {
		if has["DurationMs"] && has["EndTime"] {
			return ToolSrumECmdAppTimeline
		}
		if has["ChargeLevel"] && has["DesignedCapacity"] {
			return ToolSrumECmdEnergyUsage
		}
		if has["ConnectedTime"] && has["ConnectStartTime"] && has["InterfaceLuid"] {
			return ToolSrumECmdNetworkConnections
		}
		if has["BytesReceived"] && has["BytesSent"] {
			return ToolSrumECmdNetworkUsages
		}
		if has["NotificationType"] && has["PayloadSize"] {
			return ToolSrumECmdPushNotifications
		}
		if has["BackgroundBytesRead"] {
			return ToolSrumECmdAppResourceUseInfo
		}
		if has["Flags"] && has["Duration"] && has["StartTime"] && !has["ChargeLevel"] {
			return ToolSrumECmdVfuprov
		}
	}

	// MFTECmd $Boot (volume boot record; no timestamp columns)
	if has["EntryPoint"] && has["Signature"] && has["BytesPerSector"] && has["SectorsPerCluster"] {
		return ToolMFTECmdBoot
	}

	// MFTECmd $SDS (NTFS security descriptor stream; no timestamp columns)
	if has["Hash"] && has["OwnerSid"] && has["GroupSid"] && has["SaclAceCount"] {
		return ToolMFTECmdSDS
	}

	// MFTECmd $J (USN journal; UpdateTimestamp is the unique discriminant)
	if has["UpdateTimestamp"] && has["UpdateReasons"] {
		return ToolMFTECmdJ
	}

	// MFTECmd $MFT
	if has["EntryNumber"] && has["SequenceNumber"] && has["ParentEntryNumber"] && !has["UpdateTimestamp"] {
		return ToolMFTECmdMFT
	}

	// SBECmd
	if has["BagPath"] && has["AbsolutePath"] && has["ShellType"] && has["FirstInteracted"] {
		return ToolSBECmd
	}

	// RBCmd ($Recycle.Bin)
	if has["SourceName"] && has["FileType"] && has["FileName"] && has["FileSize"] && has["DeletedOn"] {
		return ToolRBCmd
	}

	// AppCompatCacheParser (ShimCache)
	if has["ControlSet"] && has["CacheEntryPosition"] && has["Path"] && has["LastModifiedTimeUTC"] && has["Executed"] && has["Duplicate"] {
		return ToolAppCompatCacheParser
	}

	// WxTCmd Activity (Windows 10 Timeline)
	if has["ActivityTypeOrg"] && has["ActivityType"] && has["Executable"] && has["StartTime"] && has["ETag"] {
		return ToolWxTCmdActivity
	}

	// WxTCmd PackageIDs (lookup table; no forensic timestamps)
	if has["Platform"] && has["AdditionalInformation"] && has["Expires"] && !has["PackageIdHash"] {
		return ToolWxTCmdPackageIDs
	}

	return ""
}

// timestampColumnsForTool returns the list of timestamp column names to expand for a tool.
func timestampColumnsForTool(tool string) []string {
	switch tool {
	case ToolEvtxECmd:
		return []string{"TimeCreated"}
	case ToolPECmd:
		return []string{"LastRun", "PreviousRun0", "PreviousRun1", "PreviousRun2", "PreviousRun3",
			"PreviousRun4", "PreviousRun5", "PreviousRun6",
			"SourceCreated", "SourceModified", "SourceAccessed",
			"Volume0Created", "Volume1Created"}
	case ToolLECmd:
		return []string{"TargetCreated", "TargetModified", "TargetAccessed",
			"SourceCreated", "SourceModified", "SourceAccessed", "TrackerCreatedOn"}
	case ToolJLECmdAutomatic:
		return []string{"CreationTime", "LastModified", "TargetCreated", "TargetModified",
			"TargetAccessed", "SourceCreated", "SourceModified", "SourceAccessed", "TrackerCreatedOn"}
	case ToolJLECmdCustom:
		return []string{"TargetCreated", "TargetModified", "TargetAccessed",
			"SourceCreated", "SourceModified", "SourceAccessed", "TrackerCreatedOn"}
	case ToolAmcacheUnassociatedFiles, ToolAmcacheAssociatedFileEntries:
		return []string{"FileKeyLastWriteTimestamp", "LinkDate"}
	case ToolAmcacheProgramEntries:
		return []string{"KeyLastWriteTimestamp", "InstallDate", "InstallDateArpLastModified",
			"InstallDateMsi", "InstallDateFromLinkFile"}
	case ToolAmcacheDeviceContainers:
		return []string{"KeyLastWriteTimestamp"}
	case ToolAmcacheDevicePnps:
		return []string{"KeyLastWriteTimestamp", "DriverVerDate"}
	case ToolAmcacheDriveBinaries:
		return []string{"KeyLastWriteTimestamp", "DriverTimeStamp", "DriverLastWriteTime"}
	case ToolAmcacheDriverPackages:
		return []string{"KeyLastWriteTimestamp", "Date"}
	case ToolAmcacheShortCuts:
		return []string{"KeyLastWriteTimestamp"}
	case ToolSrumECmdAppTimeline:
		return []string{"Timestamp", "ExeTimestamp", "EndTime"}
	case ToolSrumECmdEnergyUsage:
		return []string{"Timestamp", "ExeTimestamp", "EventTimestamp"}
	case ToolSrumECmdNetworkConnections:
		return []string{"Timestamp", "ExeTimestamp", "ConnectStartTime"}
	case ToolSrumECmdNetworkUsages:
		return []string{"Timestamp", "ExeTimestamp"}
	case ToolSrumECmdPushNotifications:
		return []string{"Timestamp", "ExeTimestamp"}
	case ToolSrumECmdVfuprov:
		return []string{"Timestamp", "ExeTimestamp", "StartTime", "EndTime"}
	case ToolSrumECmdAppResourceUseInfo:
		return []string{"Timestamp", "ExeTimestamp"}
	case ToolRBCmd:
		return []string{"DeletedOn"}
	case ToolAppCompatCacheParser:
		return []string{"LastModifiedTimeUTC"}
	case ToolWxTCmdActivity:
		return []string{"StartTime", "EndTime", "LastModifiedTime",
			"LastModifiedOnClient", "OriginalLastModifiedOnClient"}
	case ToolMFTECmdMFT:
		return []string{"Created0x10", "LastModified0x10", "LastRecordChange0x10", "LastAccess0x10",
			"Created0x30", "LastModified0x30", "LastRecordChange0x30", "LastAccess0x30"}
	case ToolMFTECmdJ:
		return []string{"UpdateTimestamp"}
	case ToolSBECmd:
		return []string{"CreatedOn", "ModifiedOn", "AccessedOn", "LastWriteTime",
			"FirstInteracted", "LastInteracted"}
	}
	return nil
}

// macbForTool returns the MACB string for the given tool and timestamp column.
// Most tools use the generic deriveMACB derivation. Tools with format-specific
// conventions (e.g. WxTCmd) override per column.
func macbForTool(tool, colName string) string {
	switch tool {
	case ToolWxTCmdActivity:
		switch colName {
		case "StartTime", "EndTime":
			return "M..."
		case "LastModifiedTime", "LastModifiedOnClient", "OriginalLastModifiedOnClient":
			return ".AB."
		}
	}
	return deriveMACB(colName)
}

// deriveMACB derives the MACB value from a timestamp column name.
func deriveMACB(colName string) string {
	lower := strings.ToLower(colName)

	// Generic timestamp types that contain "created" as a substring but are
	// not file-creation timestamps (e.g., "TimeCreated"). Check before the
	// broader "created" match below.
	if lower == "timecreated" {
		return "...."
	}

	// Created/Birth types
	if strings.Contains(lower, "created") || strings.Contains(lower, "creation") ||
		strings.Contains(lower, "birth") {
		return "...B"
	}

	// Modified/LastRun/LastWrite/Deleted types
	if strings.Contains(lower, "modified") || strings.Contains(lower, "lastrun") ||
		strings.Contains(lower, "previousrun") || strings.Contains(lower, "lastwrite") ||
		strings.Contains(lower, "deleted") {
		return "M..."
	}

	// Accessed types
	if strings.Contains(lower, "accessed") || strings.Contains(lower, "access") ||
		strings.Contains(lower, "interacted") {
		return ".A.."
	}

	// Change types
	if strings.Contains(lower, "change") || strings.Contains(lower, "recordchange") {
		return "..C."
	}

	// Timestamp/Time types (generic)
	if strings.Contains(lower, "timestamp") || strings.Contains(lower, "timecreated") ||
		strings.Contains(lower, "linkdate") || strings.Contains(lower, "exetimestamp") ||
		strings.Contains(lower, "eventtimestamp") || strings.Contains(lower, "endtime") ||
		strings.Contains(lower, "starttime") || strings.Contains(lower, "connectstarttime") {
		return "...."
	}

	return "...."
}

// normalizeDatetime converts various datetime formats to "YYYY-MM-DD HH:MM:SS".
// Handles EZ Tool formats: "YYYY-MM-DD HH:MM:SS.nnnnnnn" and "MM/DD/YYYY HH:MM:SS +HH:MM".
func normalizeDatetime(dt string) string {
	dt = strings.TrimSpace(strings.TrimRight(dt, "\r"))

	if dt == "" || dt == "0001-01-01 00:00:00" {
		return ""
	}

	// Check for "0001-01-01" prefix (with any time or fractional seconds)
	if strings.HasPrefix(dt, "0001-01-01") {
		return ""
	}

	// Handle MM/DD/YYYY HH:MM:SS +HH:MM format (SrumECmd ExeTimestamp)
	if len(dt) > 10 && dt[2] == '/' && dt[5] == '/' {
		return normalizeSlashDate(dt)
	}

	// Standard YYYY-MM-DD HH:MM:SS with optional fractional seconds
	// Strip fractional seconds
	if idx := strings.Index(dt, "."); idx > 10 {
		dt = dt[:idx]
	}

	// Trim to 19 chars if longer
	if len(dt) > 19 {
		dt = dt[:19]
	}

	// Validate basic format
	if len(dt) == 19 && dt[4] == '-' && dt[7] == '-' && dt[10] == ' ' {
		return dt
	}

	return ""
}

// normalizeSlashDate handles MM/DD/YYYY HH:MM:SS +HH:MM format.
var slashDateRe = regexp.MustCompile(`^(\d{2})/(\d{2})/(\d{4})\s+(\d{2}:\d{2}:\d{2})`)

func normalizeSlashDate(dt string) string {
	m := slashDateRe.FindStringSubmatch(dt)
	if m == nil {
		return ""
	}
	month, day, year, time := m[1], m[2], m[3], m[4]
	result := year + "-" + month + "-" + day + " " + time
	// Check for 0001-01-01
	if strings.HasPrefix(result, "0001-01-01") {
		return ""
	}
	return result
}

// isEmptyTimestamp returns true if the timestamp value should be skipped.
func isEmptyTimestamp(val string) bool {
	val = strings.TrimSpace(strings.TrimRight(val, "\r"))
	if val == "" {
		return true
	}
	if val == "NA" {
		return true
	}
	if strings.HasPrefix(val, "0001-01-01") {
		return true
	}
	// Check for whitespace-only
	if strings.TrimSpace(val) == "" {
		return true
	}
	return false
}

// expandRow generates one Event per non-empty timestamp column for the given row.
func expandRow(tool string, row []string, colIndex map[string]int, header []string, tsColumns []string) []*model.Event {
	var events []*model.Event

	for _, tsCol := range tsColumns {
		rawTS := colVal(row, colIndex, tsCol)
		if isEmptyTimestamp(rawTS) {
			continue
		}
		dt := normalizeDatetime(rawTS)
		if dt == "" {
			continue
		}

		e := mapFieldsForTool(tool, row, colIndex, header, tsCol, tsColumns)
		e.Source = strings.ToUpper(e.Source)
		e.Datetime = dt
		e.Type = tsCol
		e.MACB = macbForTool(tool, tsCol)
		e.Timezone = "UTC"

		events = append(events, e)
	}

	return events
}

// mapFieldsForTool maps CSV columns to Event struct fields based on the detected tool.
func mapFieldsForTool(tool string, row []string, colIndex map[string]int, header []string, tsCol string, tsColumns []string) *model.Event {
	e := &model.Event{}

	switch tool {
	case ToolEvtxECmd:
		mapEvtxECmd(e, row, colIndex)
	case ToolPECmd:
		mapPECmd(e, row, colIndex)
	case ToolLECmd:
		mapLECmd(e, row, colIndex)
	case ToolJLECmdAutomatic:
		mapJLECmdAutomatic(e, row, colIndex)
	case ToolJLECmdCustom:
		mapJLECmdCustom(e, row, colIndex)
	case ToolAmcacheUnassociatedFiles, ToolAmcacheAssociatedFileEntries:
		mapAmcacheUnassociatedFiles(e, row, colIndex)
	case ToolAmcacheProgramEntries:
		mapAmcacheProgramEntries(e, row, colIndex)
	case ToolAmcacheDeviceContainers:
		mapAmcacheDeviceContainers(e, row, colIndex)
	case ToolAmcacheDevicePnps:
		mapAmcacheDevicePnps(e, row, colIndex)
	case ToolAmcacheDriveBinaries:
		mapAmcacheDriveBinaries(e, row, colIndex)
	case ToolAmcacheDriverPackages:
		mapAmcacheDriverPackages(e, row, colIndex)
	case ToolAmcacheShortCuts:
		mapAmcacheShortCuts(e, row, colIndex)
	case ToolSrumECmdAppTimeline:
		mapSrumBase(e, row, colIndex, "SRUM App Timeline")
	case ToolSrumECmdEnergyUsage:
		mapSrumBase(e, row, colIndex, "SRUM Energy Usage")
	case ToolSrumECmdNetworkConnections:
		mapSrumBase(e, row, colIndex, "SRUM Network Connection")
	case ToolSrumECmdNetworkUsages:
		mapSrumBase(e, row, colIndex, "SRUM Network Usage")
	case ToolSrumECmdPushNotifications:
		mapSrumBase(e, row, colIndex, "SRUM Push Notification")
	case ToolSrumECmdVfuprov:
		mapSrumBase(e, row, colIndex, "SRUM VFU Provider")
	case ToolSrumECmdAppResourceUseInfo:
		mapSrumBase(e, row, colIndex, "SRUM App Resource Use")
	case ToolRBCmd:
		mapRBCmd(e, row, colIndex)
	case ToolAppCompatCacheParser:
		mapAppCompatCacheParser(e, row, colIndex)
	case ToolWxTCmdActivity:
		mapWxTCmdActivity(e, row, colIndex)
	case ToolMFTECmdMFT:
		mapMFTECmdMFT(e, row, colIndex)
	case ToolMFTECmdJ:
		mapMFTECmdJ(e, row, colIndex)
	case ToolSBECmd:
		mapSBECmd(e, row, colIndex)
	}

	// Build extra from tool-specific extra fields + remaining unmapped columns
	extra := buildExtra(tool, row, colIndex, header, tsColumns)
	if extra != "" {
		e.Extra = extra
	}

	return e
}

// firstNonEmpty returns the first non-empty value from the given column names.
func firstNonEmpty(row []string, colIndex map[string]int, cols ...string) string {
	for _, col := range cols {
		v := colVal(row, colIndex, col)
		if v != "" {
			return v
		}
	}
	return ""
}

// joinNonEmpty builds a string by joining non-empty values of the given columns
// with sep. Columns that are empty or missing are skipped.
func joinNonEmpty(row []string, colIndex map[string]int, sep string, cols ...string) string {
	var parts []string
	for _, col := range cols {
		if v := colVal(row, colIndex, col); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}

func mapEvtxECmd(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "EVTX"
	e.SourceType = colVal(row, colIndex, "Channel")
	e.Host = colVal(row, colIndex, "Computer")
	e.User = colVal(row, colIndex, "UserName")
	if e.User == "" {
		e.User = colVal(row, colIndex, "UserId")
	}
	e.Filename = colVal(row, colIndex, "SourceFile")
	e.EventID = colVal(row, colIndex, "EventId")
	e.EventType = colVal(row, colIndex, "Level")
	e.Format = "eztool_evtxecmd"

	// Build desc from MapDescription + PayloadData1-6
	var parts []string
	if v := colVal(row, colIndex, "MapDescription"); v != "" {
		parts = append(parts, v)
	}
	for _, col := range []string{"PayloadData1", "PayloadData2", "PayloadData3", "PayloadData4", "PayloadData5", "PayloadData6"} {
		if v := colVal(row, colIndex, col); v != "" {
			parts = append(parts, v)
		}
	}
	e.Desc = strings.Join(parts, " | ")
}

func mapPECmd(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "PF"
	e.SourceType = "Prefetch"
	e.Desc = colVal(row, colIndex, "ExecutableName")
	e.Filename = colVal(row, colIndex, "SourceFilename")
	e.Format = "eztool_pecmd"
}

func mapLECmd(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "LNK"
	e.SourceType = "LNK File"
	e.Desc = firstNonEmpty(row, colIndex, "LocalPath", "NetworkPath", "TargetIDAbsolutePath", "RelativePath")
	e.Filename = colVal(row, colIndex, "SourceFile")
	e.Format = "eztool_lecmd"
}

func mapJLECmdAutomatic(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "JUMPLIST"
	e.SourceType = "Automatic JumpList"
	e.Desc = firstNonEmpty(row, colIndex, "Path", "LocalPath", "TargetIDAbsolutePath")
	e.Filename = colVal(row, colIndex, "SourceFile")
	e.Format = "eztool_jlecmd"
	e.Notes = colVal(row, colIndex, "AppIdDescription")
}

func mapJLECmdCustom(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "JUMPLIST"
	e.SourceType = "Custom JumpList"
	e.Desc = firstNonEmpty(row, colIndex, "LocalPath", "TargetIDAbsolutePath")
	e.Filename = colVal(row, colIndex, "SourceFile")
	e.Format = "eztool_jlecmd"
	e.Notes = colVal(row, colIndex, "AppIdDescription")
}

func mapAmcacheUnassociatedFiles(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "AMCACHE"
	e.SourceType = "Amcache Unassociated File Entry"
	e.Desc = firstNonEmpty(row, colIndex, "FullPath", "Name")
	e.Filename = colVal(row, colIndex, "FullPath")
	e.Format = "eztool_amcacheparser"
}

func mapAmcacheDeviceContainers(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "Amcache"
	e.SourceType = "Amcache DeviceContainers"
	e.Desc = firstNonEmpty(row, colIndex, "FriendlyName", "KeyName")
	e.Notes = joinNonEmpty(row, colIndex, "; ", "Manufacturer", "ModelName", "ModelNumber")
	e.Format = "eztool_amcacheparser"
}

func mapAmcacheDevicePnps(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "Amcache"
	e.SourceType = "Amcache DevicePnps"
	e.Desc = firstNonEmpty(row, colIndex, "Description", "KeyName")
	e.Notes = joinNonEmpty(row, colIndex, "; ", "Class", "Manufacturer", "DriverName", "DriverVerVersion")
	e.Format = "eztool_amcacheparser"
}

func mapAmcacheDriveBinaries(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "Amcache"
	e.SourceType = "Amcache DriveBinaries"
	e.Desc = colVal(row, colIndex, "DriverName")
	e.Notes = joinNonEmpty(row, colIndex, "; ", "DriverCompany", "DriverVersion", "Product", "ProductVersion")
	e.Format = "eztool_amcacheparser"
}

func mapAmcacheDriverPackages(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "Amcache"
	e.SourceType = "Amcache DriverPackages"
	e.Desc = firstNonEmpty(row, colIndex, "Inf", "KeyName")
	e.Notes = joinNonEmpty(row, colIndex, "; ", "Class", "Provider", "Version", "Directory")
	e.Format = "eztool_amcacheparser"
}

func mapAmcacheShortCuts(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "Amcache"
	e.SourceType = "Amcache ShortCuts"
	e.Desc = colVal(row, colIndex, "LnkName")
	e.Notes = colVal(row, colIndex, "KeyName")
	e.Format = "eztool_amcacheparser"
}

func mapSrumBase(e *model.Event, row []string, colIndex map[string]int, sourceType string) {
	e.Source = "SRUM"
	e.SourceType = sourceType
	e.Desc = colVal(row, colIndex, "ExeInfo")
	e.User = colVal(row, colIndex, "UserName")
	if e.User == "" {
		e.User = colVal(row, colIndex, "Sid")
	}
	e.Format = "eztool_srumecmd"
	e.Notes = colVal(row, colIndex, "ExeInfoDescription")
}

func mapAmcacheProgramEntries(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "Amcache"
	e.SourceType = "Amcache ProgramEntries"
	e.Desc = firstNonEmpty(row, colIndex, "Name", "ProgramName")
	e.Notes = joinNonEmpty(row, colIndex, "; ", "Publisher", "Version", "Type")
	e.Format = "eztool_amcacheparser"
}

func mapMFTECmdMFT(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "FileSystem"
	e.SourceType = "MFT"
	e.Desc = firstNonEmpty(row, colIndex, "FileName", "Name")
	e.Filename = colVal(row, colIndex, "ParentPath")
	e.Format = "eztool_mftecmd"
}

func mapMFTECmdJ(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "FileSystem"
	e.SourceType = "MFT Journal"
	e.Desc = colVal(row, colIndex, "Name")
	e.Filename = colVal(row, colIndex, "ParentPath")
	e.Format = "eztool_mftecmd"
}

func mapSBECmd(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "SHELLBAGS"
	e.SourceType = "Shellbag Entry"
	e.Desc = colVal(row, colIndex, "AbsolutePath")
	e.Format = "eztool_sbecmd"
}

func mapRBCmd(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "FileSystem"
	e.SourceType = "$Recycle.Bin"
	e.Desc = colVal(row, colIndex, "FileName")
	e.Filename = colVal(row, colIndex, "SourceName")
	e.Notes = colVal(row, colIndex, "FileType")
	e.Format = "eztool_rbcmd"
}

func mapAppCompatCacheParser(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "Registry"
	e.SourceType = "AppCompatCache"
	e.Desc = colVal(row, colIndex, "Path")
	e.Filename = colVal(row, colIndex, "SourceFile")
	e.Notes = joinNonEmpty(row, colIndex, "; ", "ControlSet", "CacheEntryPosition")
	e.Format = "eztool_appcompatcacheparser"
}

func mapWxTCmdActivity(e *model.Event, row []string, colIndex map[string]int) {
	e.Source = "WindowsTimeline"
	e.SourceType = "Windows 10 Timeline"
	e.Desc = firstNonEmpty(row, colIndex, "DisplayText", "Executable")
	e.Notes = joinNonEmpty(row, colIndex, "; ", "Executable", "ActivityType", "DevicePlatform")
	e.User = colVal(row, colIndex, "TimeZone")
	e.Format = "eztool_wxtcmd"
}

// extraColumnsForTool returns the tool-specific columns that should be included in extra.
func extraColumnsForTool(tool string) []string {
	switch tool {
	case ToolPECmd:
		return []string{"Hash", "RunCount", "Size", "Version", "Volume0Name", "Volume0Serial",
			"Directories", "FilesLoaded"}
	case ToolLECmd:
		return []string{"FileSize", "DriveType", "VolumeSerialNumber", "VolumeLabel",
			"MachineID", "MachineMACAddress", "MACVendor", "Arguments", "WorkingDirectory"}
	case ToolJLECmdAutomatic:
		return []string{"AppId", "EntryNumber", "InteractionCount", "PinStatus",
			"Hostname", "MacAddress", "MachineID", "MachineMACAddress", "Arguments"}
	case ToolJLECmdCustom:
		return []string{"AppId", "EntryName", "MachineID", "MachineMACAddress", "Arguments"}
	case ToolAmcacheUnassociatedFiles, ToolAmcacheAssociatedFileEntries:
		return []string{"SHA1", "ApplicationName", "ProgramId", "Size", "Version",
			"ProductVersion", "ProductName", "BinaryType", "Language", "Description", "FileExtension"}
	case ToolAmcacheProgramEntries:
		return []string{"ProgramId", "Language", "Source", "RootDirPath", "UninstallString"}
	case ToolAmcacheDeviceContainers:
		return []string{"Categories", "PrimaryCategory", "State", "IsActive", "IsConnected",
			"IsPaired", "IsNetworked", "IsMachineContainer"}
	case ToolAmcacheDevicePnps:
		return []string{"ClassGuid", "Compid", "ContainerId", "DriverId", "DriverPackageStrongName",
			"Enumerator", "HWID", "Inf", "InstallState", "MatchingId", "Model", "Provider",
			"Service", "Stackid", "ProblemCode"}
	case ToolAmcacheDriveBinaries:
		return []string{"DriverInBox", "DriverIsKernelMode", "DriverSigned", "DriverCheckSum",
			"DriverId", "DriverPackageStrongName", "DriverType", "ImageSize", "Inf", "Service",
			"WdfVersion"}
	case ToolAmcacheDriverPackages:
		return []string{"DriverInBox", "Hwids", "SubmissionId", "SYSFILE"}
	case ToolAmcacheShortCuts:
		return nil
	case ToolRBCmd:
		return []string{"FileSize"}
	case ToolAppCompatCacheParser:
		return []string{"Executed", "Duplicate"}
	case ToolWxTCmdActivity:
		return []string{"Duration", "IsLocalOnly", "CreatedInCloud", "ETag"}
	case ToolMFTECmdMFT:
		return []string{"SequenceNumber", "InUse", "FileSize", "Extension", "IsDirectory",
			"HasAds", "IsAds", "Copied", "SiFlags", "NameType", "ReferenceCount",
			"SecurityId", "UpdateSequenceNumber", "ZoneIdContents"}
	case ToolMFTECmdJ:
		return []string{"UpdateReasons", "EntryNumber", "SequenceNumber",
			"ParentEntryNumber", "ParentSequenceNumber"}
	case ToolSBECmd:
		return []string{"BagPath", "ShellType", "Value", "Slot", "NodeSlot", "MRUPosition",
			"MFTEntry", "MFTSequenceNumber", "HasExplored", "Miscellaneous"}
	}

	// SRUM subtypes
	switch tool {
	case ToolSrumECmdAppTimeline:
		return []string{"DurationMs"}
	case ToolSrumECmdEnergyUsage:
		return []string{"ChargeLevel", "DesignedCapacity", "FullChargedCapacity"}
	case ToolSrumECmdNetworkConnections:
		return []string{"ConnectedTime", "InterfaceType", "ProfileName"}
	case ToolSrumECmdNetworkUsages:
		return []string{"BytesReceived", "BytesSent", "InterfaceType", "ProfileName"}
	case ToolSrumECmdPushNotifications:
		return []string{"NetworkType", "NotificationType", "PayloadSize"}
	case ToolSrumECmdVfuprov:
		return []string{"Flags", "Duration"}
	case ToolSrumECmdAppResourceUseInfo:
		return []string{"BackgroundBytesRead", "BackgroundBytesWritten", "ForegroundBytesRead",
			"ForegroundBytesWritten", "FaceTime"}
	}

	return nil
}

// mappedColumnsForTool returns the columns that are already mapped to Event fields
// (including timestamp columns), so they are excluded from the "unmapped" extra.
func mappedColumnsForTool(tool string) []string {
	switch tool {
	case ToolEvtxECmd:
		return []string{"Channel", "Computer", "UserName", "UserId", "SourceFile",
			"EventId", "Level", "MapDescription", "PayloadData1", "PayloadData2",
			"PayloadData3", "PayloadData4", "PayloadData5", "PayloadData6"}
	case ToolPECmd:
		return []string{"ExecutableName", "SourceFilename"}
	case ToolLECmd:
		return []string{"LocalPath", "NetworkPath", "TargetIDAbsolutePath", "RelativePath", "SourceFile"}
	case ToolJLECmdAutomatic:
		return []string{"Path", "LocalPath", "TargetIDAbsolutePath", "SourceFile", "AppIdDescription"}
	case ToolJLECmdCustom:
		return []string{"LocalPath", "TargetIDAbsolutePath", "SourceFile", "AppIdDescription"}
	case ToolAmcacheUnassociatedFiles, ToolAmcacheAssociatedFileEntries:
		return []string{"FullPath", "Name"}
	case ToolAmcacheProgramEntries:
		return []string{"Name", "ProgramName", "Publisher", "Version", "Type"}
	case ToolAmcacheDeviceContainers:
		return []string{"KeyName", "FriendlyName", "Manufacturer", "ModelName", "ModelNumber"}
	case ToolAmcacheDevicePnps:
		return []string{"KeyName", "Description", "Class", "Manufacturer", "DriverName", "DriverVerVersion"}
	case ToolAmcacheDriveBinaries:
		return []string{"DriverName", "DriverCompany", "DriverVersion", "Product", "ProductVersion"}
	case ToolAmcacheDriverPackages:
		return []string{"KeyName", "Inf", "Class", "Provider", "Version", "Directory"}
	case ToolAmcacheShortCuts:
		return []string{"KeyName", "LnkName"}
	case ToolSrumECmdAppTimeline, ToolSrumECmdEnergyUsage, ToolSrumECmdNetworkConnections,
		ToolSrumECmdNetworkUsages, ToolSrumECmdPushNotifications, ToolSrumECmdVfuprov,
		ToolSrumECmdAppResourceUseInfo:
		return []string{"ExeInfo", "UserName", "Sid", "ExeInfoDescription"}
	case ToolRBCmd:
		return []string{"SourceName", "FileName", "FileType"}
	case ToolAppCompatCacheParser:
		return []string{"Path", "SourceFile", "ControlSet", "CacheEntryPosition"}
	case ToolWxTCmdActivity:
		return []string{"DisplayText", "Executable", "ActivityType", "DevicePlatform", "TimeZone"}
	case ToolMFTECmdMFT:
		return []string{"FileName", "Name", "ParentPath"}
	case ToolMFTECmdJ:
		return []string{"Name", "ParentPath"}
	case ToolSBECmd:
		return []string{"AbsolutePath"}
	}
	return nil
}

// buildExtra builds the extra field from tool-specific extra columns plus remaining unmapped columns.
func buildExtra(tool string, row []string, colIndex map[string]int, header []string, tsColumns []string) string {
	var parts []string

	// Add tool-specific extra columns (in order, non-empty only)
	for _, col := range extraColumnsForTool(tool) {
		v := colVal(row, colIndex, col)
		if v != "" {
			parts = append(parts, col+": "+v)
		}
	}

	// Build set of columns already accounted for
	skip := make(map[string]bool)
	for _, col := range tsColumns {
		skip[col] = true
	}
	for _, col := range extraColumnsForTool(tool) {
		skip[col] = true
	}
	for _, col := range mappedColumnsForTool(tool) {
		skip[col] = true
	}

	// Add remaining unmapped non-empty columns
	for i, val := range row {
		if i >= len(header) {
			break
		}
		val = strings.TrimSpace(strings.TrimRight(val, "\r"))
		if val == "" {
			continue
		}
		col := header[i]
		if skip[col] {
			continue
		}
		parts = append(parts, col+": "+val)
	}

	return strings.Join(parts, "; ")
}
