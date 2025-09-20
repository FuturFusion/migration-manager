package util

import (
	"fmt"
	"strings"
)

// MapWindowsVersionToAbbrev takes a full version string and returns the abbreviation used by distrobuilder logic.
// Versions supported are an intersection of what's supported by distrobuilder and vCenter.
func MapWindowsVersionToAbbrev(version string) (string, error) {
	switch {
	case strings.Contains(version, "Windows 10"):
		return "w10", nil
	case strings.Contains(version, "Windows 11"):
		return "w11", nil
	case strings.Contains(version, "Server 2016"):
		return "2k16", nil
	case strings.Contains(version, "Server 2019"):
		return "2k19", nil
	case strings.Contains(version, "Server 2022"):
		return "2k22", nil
	case strings.Contains(version, "Server 2025"):
		return "2k25", nil
	default:
		return "", fmt.Errorf("%q is not currently supported", version)
	}
}
