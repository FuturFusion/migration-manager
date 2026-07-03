package util

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// WindowsDirectory returns the path to the C:\Windows directory for the given version code.
func WindowsDirectory(code string) string {
	if code == "2k3" {
		return "WINDOWS"
	}

	return "Windows"
}

// SupportsNetworkAssignment returns whether the Windows version supports offline network reassignment.
// Windows 10 and up keeps a history of MAC addresses to network config GUIDs in the registry.
// Earlier Windows versions do not have this feature and thus we can't determine which NIC should be assigned to which network config post-migration.
func SupportsNetworkAssignment(code string) bool {
	legacyCodes := []string{"xp", "w7", "w8", "2k3", "2k8", "2k12"}
	hasPrefix := func(p string) bool { return strings.HasPrefix(code, p) }

	return !slices.ContainsFunc(legacyCodes, hasPrefix)
}

// ValidateUbuntuVersion validates that the version is empty or of format YY.MM.
func ValidateUbuntuVersion(version string) error {
	if version == "" {
		return nil
	}

	parts := strings.Split(version, ".")

	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return fmt.Errorf("Invalid version format %q, expected YY.MM", version)
	}

	for _, part := range parts {
		_, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("Invalid version format %q, expected YY.MM: %w", version, err)
		}
	}

	return nil
}
