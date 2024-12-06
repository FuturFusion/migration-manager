package util

import (
	"strings"
)

func IsDebianOrDerivative(osString string) bool {
	return strings.Contains(strings.ToLower(osString), "debian") || strings.Contains(strings.ToLower(osString), "ubuntu")
}

func IsRHELOrDerivative(osString string) bool {
	return strings.Contains(strings.ToLower(osString), "centos") || strings.Contains(strings.ToLower(osString), "oracle") || strings.Contains(strings.ToLower(osString), "rhel")
}

func IsSUSEOrDerivative(osString string) bool {
	return strings.Contains(strings.ToLower(osString), "suse") || strings.Contains(strings.ToLower(osString), "opensuse")
}
