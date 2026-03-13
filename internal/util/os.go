package util

import (
	"fmt"
	"strings"
)

// Supported windows versions, with matching distrobuilder abbreviations.
// Versions supported are an intersection of what's supported by distrobuilder and vCenter.
var windowsVersions = map[string]string{
	"10":          "w10",
	"11":          "w11",
	"Server 2016": "2k16",
	"Server 2019": "2k19",
	"Server 2022": "2k22",
	"Server 2025": "2k25",
}

// ToWindowsVersion returns the windows version for the given OS description.
func ToWindowsVersion(desc string) (string, error) {
	for v := range windowsVersions {
		compare := v
		if !strings.HasPrefix(compare, "Server ") {
			compare = "Windows " + compare
		}

		if strings.Contains(desc, compare) {
			return v, nil
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
