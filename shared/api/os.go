package api

import (
	"fmt"

	"github.com/lxc/incus/v7/shared/osinfo"
)

type (
	OSType = osinfo.OSType
)

const (
	OSTYPE_FORTIGATE OSType = "fortigate"
	OSTYPE_WINDOWS   OSType = osinfo.Windows
	OSTYPE_LINUX     OSType = osinfo.Linux
	OSTYPE_FREEBSD   OSType = osinfo.FreeBSD
)

func ValidateOSType(os string) error {
	switch osinfo.OSType(os) {
	case osinfo.FreeBSD:
	case osinfo.Linux:
	case osinfo.MacOS:
	case osinfo.Windows:
	case OSTYPE_FORTIGATE:
	default:
		return fmt.Errorf("Unknown OS type %q", os)
	}

	return nil
}

func ValidateDistribution(osType osinfo.OSType, distro string) error {
	switch osType {
	case osinfo.Linux:
		switch osinfo.Distro(distro) {
		case osinfo.AlmaLinux:
		case osinfo.AmazonLinux:
		case osinfo.ArchLinux:
		case osinfo.CentOSLinux:
		case osinfo.DebianLinux:
		case osinfo.FedoraLinux:
		case osinfo.OracleLinux:
		case osinfo.RedHatLinux:
		case osinfo.RockyLinux:
		case osinfo.SUSELinux:
		case osinfo.UbuntuLinux:
		case osinfo.OtherDistro:
		default:
			return fmt.Errorf("Unknown Linux OS distribution %q", distro)
		}

	default:
		if distro != string(osinfo.OtherDistro) {
			return fmt.Errorf("Distribution %q is not compatible with OS type %q", distro, osType)
		}
	}

	return nil
}
