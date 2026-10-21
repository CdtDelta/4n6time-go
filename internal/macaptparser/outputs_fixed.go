package macaptparser

import (
	"strings"

	"github.com/cdtdelta/4n6time/internal/model"
)

// Labels, sources, and types for the fixed-schema outputs in this file. The
// three untimed outputs have no source or type because they produce no events.
const (
	spotlightConfigLabel          = "mac_apt SpotlightConfig"
	spotlightConfigSource         = "SPOTLIGHT"
	spotlightConfigStoreCreated   = "Spotlight store created"
	spotlightConfigConfigModified = "Spotlight configuration modified"
	networkDHCPLabel              = "mac_apt Network_DHCP"
	networkDHCPSource             = "NETWORKING"
	networkDHCPType               = "DHCP lease start"
	networkDetailsLabel           = "mac_apt Network_Details"
	networkInterfacesLabel        = "mac_apt Network_Interfaces"
	fileSharingLabel              = "mac_apt FileSharing"
	spotlightConfigStoreColumn    = "StoreCreationDate"
	spotlightConfigModifiedColumn = "ConfigurationModificationDate"
)

// Column lists from mac_apt v1.36.2, copied verbatim. Detection compares them
// exactly.
var (
	spotlightConfigColumns = []string{
		"StoreUUID", "StoreCreationDate", "Version", "IndexVersion", "PartialPath",
		"ConfigurationModificationDate", "ConfigurationModificationVersion",
		"ConfigurationVolumeUUID", "Source",
	}
	networkDHCPColumns = []string{
		"Interface", "MAC_Address", "IPAddress", "LeaseLength", "LeaseStartDate",
		"PacketData", "RouterHardwareAddress", "RouterIPAddress", "SSID", "Source",
	}
	networkDetailsColumns = []string{
		"UUID", "IPv4.ConfigMethod", "IPv6.ConfigMethod", "DeviceName", "Hardware",
		"Type", "SubType", "UserDefinedName", "Proxies.ExceptionsList",
		"SMB.NetBIOSName", "SMB.Workgroup", "PPP", "Modem", "Source",
	}
	networkInterfacesColumns = []string{
		"Category", "Active", "BSD Name", "IOBuiltin", "IOInterfaceNamePrefix",
		"IOInterfaceType", "IOInterfaceUnit", "IOMACAddress", "IOPathMatch",
		"SCNetworkInterfaceInfo", "SCNetworkInterfaceType", "Source",
	}
	fileSharingColumns = []string{
		"Name", "Directory_Path", "SMB_Guest_Access", "AFP_Guest_Access", "Source",
	}
)

// mapSpotlightConfig maps one SpotlightConfig row to one event per store or
// configuration timestamp. The event type comes from the column name, which
// mapTimestampColumns sets, and is then replaced with the readable type.
func mapSpotlightConfig(row map[string]string) ([]*model.Event, int) {
	events, skipped := mapTimestampColumns(row,
		[]string{spotlightConfigStoreColumn, spotlightConfigModifiedColumn},
		spotlightConfigSource, spotlightConfigLabel,
		func(e *model.Event) []field {
			desc := joinNonEmpty(" ", "Spotlight store", row["StoreUUID"])
			if p := strings.TrimSpace(row["PartialPath"]); p != "" {
				desc = joinNonEmpty(" ", desc, "at", p)
			}
			e.Desc = desc
			e.RecordNumber = row["StoreUUID"]
			return columnFields(row, "Version", "IndexVersion",
				"ConfigurationModificationVersion", "ConfigurationVolumeUUID")
		})
	for _, e := range events {
		switch e.Type {
		case spotlightConfigStoreColumn:
			e.Type = spotlightConfigStoreCreated
		case spotlightConfigModifiedColumn:
			e.Type = spotlightConfigConfigModified
		}
	}
	return events, skipped
}

// networkDHCPDesc renders "<Interface> leased <IPAddress>", adding the router
// and SSID only when present.
func networkDHCPDesc(row map[string]string) string {
	desc := joinNonEmpty(" ", row["Interface"], "leased", row["IPAddress"])
	if r := strings.TrimSpace(row["RouterIPAddress"]); r != "" {
		desc = joinNonEmpty(" ", desc, "from router", r)
	}
	if s := strings.TrimSpace(row["SSID"]); s != "" {
		desc = joinNonEmpty(" ", desc, "on SSID", s)
	}
	return desc
}

// mapNetworkDHCP maps one Network_DHCP row to one event per lease start.
// PacketData is the raw DHCP packet in hex and is never read, so it reaches no
// event field.
func mapNetworkDHCP(row map[string]string) ([]*model.Event, int) {
	return mapTimestampColumns(row, []string{"LeaseStartDate"}, networkDHCPSource, networkDHCPLabel,
		func(e *model.Event) []field {
			e.Type = networkDHCPType
			e.Desc = networkDHCPDesc(row)
			return columnFields(row, "MAC_Address", "LeaseLength", "RouterHardwareAddress")
		})
}

// noTimeline is the mapper for the untimed outputs. These outputs have no
// timestamp columns, so they yield no events and count no skips. Single-file
// import completes with zero events, as untimed EZ Tools files do.
func noTimeline(row map[string]string) ([]*model.Event, int) {
	return nil, 0
}

// mapNetworkDetails maps a Network_Details row. The output has no timestamps.
func mapNetworkDetails(row map[string]string) ([]*model.Event, int) {
	return noTimeline(row)
}

// mapNetworkInterfaces maps a Network_Interfaces row. The output has no timestamps.
func mapNetworkInterfaces(row map[string]string) ([]*model.Event, int) {
	return noTimeline(row)
}

// mapFileSharing maps a FileSharing row. The output has no timestamps.
func mapFileSharing(row map[string]string) ([]*model.Event, int) {
	return noTimeline(row)
}
