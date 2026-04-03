package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FuturFusion/migration-manager/shared/api"
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

func GetOSCompatibility(osType api.OSType, distro api.Distro, distroVersion string) (supportsSCSI bool, supportsNet bool, supports9p bool, supportsCPU bool, err error) {
	// Assume full support by default.
	supportsSCSI = true
	supportsNet = true
	supports9p = true
	supportsCPU = true

	switch osType {
	case api.OSTYPE_BSD:
	case api.OSTYPE_FORTIGATE:
	case api.OSTYPE_LINUX:
		var v int
		if distroVersion != "" {
			if distro != api.DISTRO_UBUNTU {
				v, err = strconv.Atoi(distroVersion)
			} else {
				v, err = strconv.Atoi(strings.Split(distroVersion, ".")[0])
			}

			if err != nil {
				return false, false, false, false, fmt.Errorf("Failed to check for virtio support with invalid OS %s (%s) version %s: %w", osType, distro, distroVersion, err)
			}
		}

		if v == 0 {
			return //nolint: revive,nakedret
		}

		switch distro {
		case api.DISTRO_ARCH, api.DISTRO_OTHER:
			// Treat other versions as fully supported.
		case api.DISTRO_AMZN, api.DISTRO_FEDORA:
			// Fedora and Amazon have non-standard versioning.
			supports9p = false
		case api.DISTRO_UBUNTU:
			supportsCPU = v >= 17
		case api.DISTRO_SUSE:
			supportsSCSI = v >= 10
			supportsNet = v >= 11
		case api.DISTRO_DEBIAN:
			supportsSCSI = v >= 6
			supportsNet = v >= 6
			supports9p = v >= 9
			supportsCPU = v <= 5 || v >= 10
		default:
			if distro.IsRHELDerivative() {
				supportsSCSI = v >= 6
				supportsNet = v >= 7
				supportsCPU = distro != api.DISTRO_ORACLE || v >= 7
				supports9p = false
			}
		}
	case api.OSTYPE_WINDOWS:
		code, err := MapWindowsVersionToAbbrev(distroVersion)
		if err != nil {
			return false, false, false, false, fmt.Errorf("Failed to check for Windows virtio support: %w", err)
		}

		supportsSCSI = code != "2k3"
		supportsNet = code != "2k3"
		supports9p = false
	}

	return //nolint: revive,nakedret
}
