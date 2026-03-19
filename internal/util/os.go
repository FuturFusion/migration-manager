package util

import (
	"fmt"
	"strings"
)

// Supported windows versions, with matching distrobuilder abbreviations.
// Versions supported are an intersection of what's supported by distrobuilder and vCenter.
var windowsVersions = map[string]string{
	"10":             "w10",
	"11":             "w11",
	"Server 2003":    "2k3",
	"Server 2008":    "2k8",
	"Server 2008 R2": "2k8R2",
	"Server 2012":    "2k12",
	"Server 2012 R2": "2k12R2",
	"Server 2016":    "2k16",
	"Server 2019":    "2k19",
	"Server 2022":    "2k22",
	"Server 2025":    "2k25",
}

var windowsAliases = map[string][]string{
	"Server 2008 R2": {"Server R2 2008"},
	"Server 2012 R2": {"Server R2 2012"},
}

// WindowsDirectory returns the path to the C:\Windows directory for the given version code.
func WindowsDirectory(code string) string {
	if code == "2k3" {
		return "WINDOWS"
	}

	return "Windows"
}

// ToWindowsVersion returns the windows version for the given OS description.
func ToWindowsVersion(desc string) (string, error) {
	for v := range windowsVersions {
		compare := v
		if !strings.HasPrefix(compare, "Server ") {
			compare = "Windows " + compare
		}

		if strings.Contains(desc, " R2") && !strings.HasSuffix(compare, " R2") {
			continue
		}

		if strings.Contains(desc, compare) {
			return v, nil
		}

		aliases, ok := windowsAliases[v]
		if ok {
			for _, alias := range aliases {
				if strings.Contains(desc, alias) {
					return v, nil
				}
			}
		}
	}

	return "", fmt.Errorf("Windows version %q is unknown or unsupported", desc)
}

// MapWindowsVersionToAbbrev takes a full version string and returns the abbreviation used by distrobuilder logic.
func MapWindowsVersionToAbbrev(version string) (string, error) {
	code, ok := windowsVersions[version]
	if !ok {
		return "", fmt.Errorf("Invalid Windows version %q", version)
	}

	return code, nil
}

// ValidateWindowsVersion checks if the given Windows version is valid.
func ValidateWindowsVersion(v string) error {
	_, ok := windowsVersions[v]
	if !ok {
		return fmt.Errorf("Unknown windows version %q", v)
	}

	return nil
}

// SupportsNetworkAssignment returns whether the Windows version supports offline network reassignment.
// Windows 10 and up keeps a history of MAC addresses to network config GUIDs in the registry.
// Earlier Windows versions do not have this feature and thus we can't determine which NIC should be assigned to which network config post-migration.
func SupportsNetworkAssignment(code string) bool {
	return !strings.HasPrefix(code, "2k3") && !strings.HasPrefix(code, "2k8") && !strings.HasPrefix(code, "2k12")
}
