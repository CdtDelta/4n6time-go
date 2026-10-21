package macaptparser

import (
	"fmt"
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"
)

// Labels are the sourcetype stored on every event from the output. Sources
// are the uppercase output names stored in the source column.
const (
	installHistoryLabel   = "mac_apt InstallHistory"
	safariLabel           = "mac_apt Safari"
	xprotectLabel         = "mac_apt XProtect_Behavior"
	dockItemsLabel        = "mac_apt Dock Items"
	sudoLastRunLabel      = "mac_apt SudoLastRun"
	internetAccountsLabel = "mac_apt InternetAccounts"
	tccLabel              = "mac_apt TCC"
	utmpxLabel            = "mac_apt Utmpx"
	termSessionsLabel     = "mac_apt TermSessions"
	quarantineLabel       = "mac_apt Quarantine"
	cookiesLabel          = "mac_apt Cookies"
	recentItemsLabel      = "mac_apt RecentItems"
	spotlightLabel        = "mac_apt SpotlightShortcuts"
	usersLabel            = "mac_apt Users"
)

const (
	installHistorySource   = "INSTALLHISTORY"
	safariSource           = "SAFARI"
	xprotectSource         = "XPROTECT"
	dockItemsSource        = "DOCKITEMS"
	sudoLastRunSource      = "SUDOLASTRUN"
	internetAccountsSource = "INETACCOUNTS"
	tccSource              = "TCC"
	utmpxSource            = "UTMPX"
	termSessionsSource     = "TERMSESSIONS"
	quarantineSource       = "QUARANTINE"
	cookiesSource          = "COOKIES"
	recentItemsSource      = "RECENTITEMS"
	spotlightSource        = "SPOTLIGHTSHORTCUTS"
	usersSource            = "USERS"
)

// Fixed event types for outputs whose type is not the timestamp column name.
const (
	installHistoryType  = "Install"
	xprotectType        = "XProtect behavior rule violation"
	sudoLastRunType     = "sudo last run"
	internetAccountType = "Account created"
	tccType             = "TCC permission modified"
	quarantineType      = "File quarantined"
	spotlightType       = "Spotlight shortcut last used"
	safariTypePrefix    = "Safari"
	installedByPhrase   = "installed by "
	quarantineFallback  = "quarantine event"
)

// Column lists from mac_apt v1.36.2, copied verbatim. Detection compares them
// exactly.
var (
	installHistoryColumns = []string{
		"ContentType", "Date", "DisplayName", "DisplayVersion",
		"PackageIdentifiers", "ProcessName", "Source",
	}
	safariColumns = []string{
		"Type", "Name_or_Title", "URL", "Date", "Other_Info", "User", "Source",
	}
	xprotectColumns = []string{
		"Id", "Date", "Violated_Rule", "Exec_Path", "Exec_Cdhash",
		"Exec_Signing_ID", "Exec_Team_ID", "Exec_Sha256", "Exec_Is_Notarized",
		"Responsible_Path", "Responsible_Cdhash", "Responsible_Signing_ID",
		"Responsible_Team_ID", "Responsible_Sha256", "Responsible_Is_Notarized",
		"Reported", "Profile_Hash", "Source",
	}
	dockItemsColumns = []string{
		"File Label", "Parent Modified", "File Modified", "Recently Used",
		"File Type", "File Path", "GUID", "Bundle Identifier", "User", "Source",
	}
	sudoLastRunColumns      = []string{"Uid", "Run Date", "User", "Source"}
	internetAccountsColumns = []string{
		"ID", "Type", "Name", "Date", "UserDisplayName", "Username", "Parent_ID",
		"Bundle", "UUID_or_AccountDSID", "User", "Source",
	}
	tccColumns = []string{
		"Last_Modified", "Service", "Client", "Client_Type", "Allowed",
		"Auth_Reason", "Indirect_Object_Identifier", "User", "Source",
	}
	utmpxColumns = []string{
		"User", "Terminal_ID", "Terminal", "PID", "Type", "Type_Name", "Date",
		"Hostname",
	}
	termSessionsColumns = []string{
		"Source_Type", "Session_Start", "Session_End", "Session_Commands",
		"All_Commands", "User", "Session_UUID", "Source",
	}
	quarantineColumns = []string{
		"EventID", "TimeStamp", "AgentBundleID", "AgentName", "DataUrl",
		"SenderName", "SenderAddress", "TypeNumber", "OriginTitle", "OriginUrl",
		"OriginAlias", "User", "Source",
	}
	cookiesColumns = []string{
		"Host", "Create Time", "Expiry Time", "Name", "Path", "Value", "User",
		"Source",
	}
	recentItemsColumns = []string{
		"Type", "Name", "URL", "Info", "Date Last Seen", "Date Target Created",
		"Volume Name", "Volume Size", "Volume UUID", "Volume Creation Date",
		"User", "Source",
	}
	spotlightColumns = []string{
		"User", "UserTyped", "DisplayName", "LastUsed", "URL", "Identifier",
		"Source",
	}
	usersColumns = []string{
		"Username", "Realname", "Homedir", "UID", "GID", "UUID", "CreationDate",
		"DeletedDate", "FailedLoginCount", "FailedLoginTime", "LastLoginTime",
		"PasswordLastSetTime", "PasswordHint", "Password", "DARWIN_USER_DIR",
		"DARWIN_USER_TEMP_DIR", "DARWIN_USER_CACHE_DIR", "Source",
	}
)

// mapTimestampColumns builds one event per timestamp column in cols whose
// value parses. fill sets the event's fields and returns the fields that
// follow the timestamp in extra. The timestamp column's raw value leads extra,
// so the examiner sees the full value that produced the event time. Empty
// values yield no event and are not counted; unparseable values are counted.
// The event type defaults to the column name; fill may override it.
func mapTimestampColumns(row map[string]string, cols []string, source, sourceType string, fill func(e *model.Event) []field) ([]*model.Event, int) {
	var events []*model.Event
	skipped := 0
	for _, col := range cols {
		raw := row[col]
		when, ok, bad := parseColumnTimestamp(raw)
		skipped += bad
		if !ok {
			continue
		}
		e := newEvent(source, sourceType, when, row["Source"])
		e.Type = col
		extra := append([]field{{col, raw}}, fill(e)...)
		e.Extra = renderExtra(extra...)
		events = append(events, e)
	}
	return events, skipped
}

// columnFields pairs each named column with its value, in the order given.
// Mappers use it to list the columns that go to extra.
func columnFields(row map[string]string, names ...string) []field {
	out := make([]field, 0, len(names))
	for _, n := range names {
		out = append(out, field{n, row[n]})
	}
	return out
}

// userValue maps mac_apt's "-" and "N/A" placeholders to an empty user.
func userValue(u string) string {
	if u == "-" || u == "N/A" {
		return ""
	}
	return u
}

// naBlank maps mac_apt's "N/A" placeholder to empty, for description parts.
func naBlank(s string) string {
	if s == "N/A" {
		return ""
	}
	return s
}

// joinNonEmpty joins the non-empty parts, each trimmed, with sep. It is how
// descriptions drop an empty part without leaving stray separators.
func joinNonEmpty(sep string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

// prefixed returns "prefix: body", or whichever side is non-empty alone.
func prefixed(prefix, body string) string {
	return joinNonEmpty(": ", prefix, body)
}

// mapInstallHistory maps one InstallHistory row to one event per Date.
func mapInstallHistory(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Date"}, installHistorySource, installHistoryLabel,
		func(e *model.Event) []field {
			e.Type = installHistoryType
			head := joinNonEmpty(" ", row["DisplayName"], row["DisplayVersion"])
			by := ""
			if pn := strings.TrimSpace(row["ProcessName"]); pn != "" {
				by = installedByPhrase + pn
			}
			e.Desc = joinNonEmpty(" ", head, by)
			return columnFields(row, "ContentType", "PackageIdentifiers")
		})
}

// mapSafari maps one Safari row to one event per Date.
func mapSafari(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Date"}, safariSource, safariLabel,
		func(e *model.Event) []field {
			e.Type = joinNonEmpty(" ", safariTypePrefix, row["Type"])
			e.Desc = prefixed(row["Type"], joinNonEmpty(" ", row["Name_or_Title"], row["URL"]))
			e.URL = row["URL"]
			e.User = userValue(row["User"])
			e.EventType = row["Type"]
			return columnFields(row, "Other_Info")
		})
}

// mapXProtectBehavior maps one XProtect_Behavior row to one event per Date.
func mapXProtectBehavior(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Date"}, xprotectSource, xprotectLabel,
		func(e *model.Event) []field {
			e.Type = xprotectType
			e.Desc = prefixed(row["Violated_Rule"], row["Exec_Path"])
			e.EventType = row["Violated_Rule"]
			e.RecordNumber = row["Id"]
			return columnFields(row,
				"Exec_Cdhash", "Exec_Signing_ID", "Exec_Team_ID", "Exec_Sha256",
				"Exec_Is_Notarized", "Responsible_Path", "Responsible_Cdhash",
				"Responsible_Signing_ID", "Responsible_Team_ID", "Responsible_Sha256",
				"Responsible_Is_Notarized", "Reported", "Profile_Hash")
		})
}

// mapDockItems maps one Dock Items row to one event per non-empty timestamp.
func mapDockItems(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Parent Modified", "File Modified"},
		dockItemsSource, dockItemsLabel,
		func(e *model.Event) []field {
			e.Desc = prefixed(row["File Label"], row["File Path"])
			e.User = userValue(row["User"])
			return columnFields(row, "Recently Used", "File Type", "GUID", "Bundle Identifier")
		})
}

// mapSudoLastRun maps one SudoLastRun row to one event per Run Date.
func mapSudoLastRun(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Run Date"}, sudoLastRunSource, sudoLastRunLabel,
		func(e *model.Event) []field {
			e.Type = sudoLastRunType
			e.Desc = joinNonEmpty(" ", "sudo last run by UID", row["Uid"])
			e.User = userValue(row["User"])
			return nil
		})
}

// mapInternetAccounts maps one InternetAccounts row to one event per Date.
func mapInternetAccounts(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Date"}, internetAccountsSource, internetAccountsLabel,
		func(e *model.Event) []field {
			e.Type = internetAccountType
			desc := joinNonEmpty(" ", row["Type"], "account", row["Username"])
			if b := strings.TrimSpace(row["Bundle"]); b != "" {
				desc = joinNonEmpty(" ", desc, "("+b+")")
			}
			e.Desc = desc
			e.User = userValue(row["User"])
			e.EventType = row["Type"]
			e.RecordNumber = row["ID"]
			return columnFields(row, "Name", "UserDisplayName", "Parent_ID", "UUID_or_AccountDSID")
		})
}

// mapTCC maps one TCC row to one event per Last_Modified.
func mapTCC(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Last_Modified"}, tccSource, tccLabel,
		func(e *model.Event) []field {
			e.Type = tccType
			client := joinNonEmpty(" ", row["Client"], "(Allowed="+row["Allowed"]+")")
			e.Desc = prefixed(row["Service"], client)
			e.User = userValue(row["User"])
			e.EventType = row["Service"]
			return columnFields(row, "Client_Type", "Auth_Reason", "Indirect_Object_Identifier")
		})
}

// mapUtmpx maps one Utmpx row to one event per Date. Utmpx has no Source
// column, so the filename stays empty.
func mapUtmpx(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Date"}, utmpxSource, utmpxLabel,
		func(e *model.Event) []field {
			e.Type = row["Type_Name"]
			e.Desc = joinNonEmpty(" ", naBlank(row["Type_Name"]), userValue(row["User"]), naBlank(row["Terminal"]))
			e.Host = row["Hostname"]
			e.User = userValue(row["User"])
			return columnFields(row, "Terminal_ID", "PID", "Type")
		})
}

// commandsDesc renders a command block on one line. Outer whitespace and line
// breaks are trimmed first, so a trailing newline does not leave a dangling
// separator.
func commandsDesc(commands string) string {
	trimmed := strings.Trim(commands, "\r\n \t")
	return strings.TrimSpace(lineBreakRun.ReplaceAllString(trimmed, " ; "))
}

// mapTermSessions maps one TermSessions row to one event per session
// timestamp. All_Commands goes in extra only when it differs from
// Session_Commands, because the description already shows that block.
func mapTermSessions(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Session_Start", "Session_End"}, termSessionsSource, termSessionsLabel,
		func(e *model.Event) []field {
			e.Desc = commandsDesc(row["Session_Commands"])
			e.User = userValue(row["User"])
			e.EventType = row["Source_Type"]
			e.RecordNumber = row["Session_UUID"]
			if row["All_Commands"] == row["Session_Commands"] {
				return nil
			}
			return columnFields(row, "All_Commands")
		})
}

// quarantineURL returns the URL an event is about: the download URL when
// present, otherwise the origin URL.
func quarantineURL(row map[string]string) string {
	if row["DataUrl"] != "" {
		return row["DataUrl"]
	}
	return row["OriginUrl"]
}

// mapQuarantine maps one Quarantine row to one event per TimeStamp. The URL
// column not used for the event's URL goes to extra, so neither URL is lost.
func mapQuarantine(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"TimeStamp"}, quarantineSource, quarantineLabel,
		func(e *model.Event) []field {
			e.Type = quarantineType
			url := quarantineURL(row)
			if url == "" {
				e.Desc = joinNonEmpty(" ", row["AgentName"], quarantineFallback)
			} else {
				e.Desc = joinNonEmpty(" ", row["AgentName"], "downloaded", url)
			}
			e.URL = url
			e.User = userValue(row["User"])
			e.RecordNumber = row["EventID"]

			names := []string{"AgentBundleID", "SenderName", "SenderAddress", "TypeNumber", "OriginTitle"}
			if row["DataUrl"] != "" {
				names = append(names, "OriginUrl")
			}
			names = append(names, "OriginAlias")
			return columnFields(row, names...)
		})
}

// mapCookies maps one Cookies row to one event per Create Time or Expiry Time.
func mapCookies(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"Create Time", "Expiry Time"}, cookiesSource, cookiesLabel,
		func(e *model.Event) []field {
			e.Desc = joinNonEmpty(" ", row["Name"], "cookie for", row["Host"]+row["Path"])
			e.User = userValue(row["User"])
			return columnFields(row, "Value")
		})
}

// mapRecentItems maps one RecentItems row to one event per non-empty
// timestamp.
func mapRecentItems(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row,
		[]string{"Date Last Seen", "Date Target Created", "Volume Creation Date"},
		recentItemsSource, recentItemsLabel,
		func(e *model.Event) []field {
			e.Desc = prefixed(row["Type"], joinNonEmpty(" ", row["Name"], row["URL"]))
			e.URL = row["URL"]
			e.User = userValue(row["User"])
			e.EventType = row["Type"]
			return columnFields(row, "Info", "Volume Name", "Volume Size", "Volume UUID")
		})
}

// mapSpotlightShortcuts maps one SpotlightShortcuts row to one event per
// LastUsed.
func mapSpotlightShortcuts(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"LastUsed"}, spotlightSource, spotlightLabel,
		func(e *model.Event) []field {
			e.Type = spotlightType
			e.Desc = strings.TrimSpace(fmt.Sprintf(`"%s" -> %s`, row["UserTyped"], row["DisplayName"]))
			e.URL = row["URL"]
			e.User = userValue(row["User"])
			return columnFields(row, "Identifier")
		})
}

// usersDesc renders "<Username> (<Realname>) UID <UID>", dropping empty parts.
func usersDesc(row map[string]string) string {
	realname := ""
	if rn := strings.TrimSpace(row["Realname"]); rn != "" {
		realname = "(" + rn + ")"
	}
	uid := ""
	if u := strings.TrimSpace(row["UID"]); u != "" {
		uid = "UID " + u
	}
	return joinNonEmpty(" ", row["Username"], realname, uid)
}

// mapUsers maps one Users row to one event per non-empty timestamp. Password
// is never read, so its hash data cannot reach any event field.
func mapUsers(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row,
		[]string{"CreationDate", "DeletedDate", "FailedLoginTime", "LastLoginTime", "PasswordLastSetTime"},
		usersSource, usersLabel,
		func(e *model.Event) []field {
			e.Desc = usersDesc(row)
			e.User = userValue(row["Username"])
			return columnFields(row,
				"Homedir", "GID", "UUID", "FailedLoginCount", "PasswordHint",
				"DARWIN_USER_DIR", "DARWIN_USER_TEMP_DIR", "DARWIN_USER_CACHE_DIR")
		})
}
