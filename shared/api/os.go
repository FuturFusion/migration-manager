package api

import (
	"fmt"
)

type OSType string

const (
	OSTYPE_WINDOWS   OSType = "windows"
	OSTYPE_LINUX     OSType = "linux"
	OSTYPE_BSD       OSType = "bsd"
	OSTYPE_FORTIGATE OSType = "fortigate"
)

type Distro string

const (
	DISTRO_ALMA    Distro = "alma"
	DISTRO_ARCH    Distro = "arch"
	DISTRO_DEBIAN  Distro = "debian"
	DISTRO_UBUNTU  Distro = "ubuntu"
	DISTRO_ORACLE  Distro = "oracle"
	DISTRO_CENTOS  Distro = "centos"
	DISTRO_RHEL    Distro = "rhel"
	DISTRO_SUSE    Distro = "suse"
	DISTRO_ROCKY   Distro = "rocky"
	DISTRO_AMZN    Distro = "amazon"
	DISTRO_FEDORA  Distro = "fedora"
	DISTRO_FREEBSD Distro = "freebsd"
	DISTRO_OTHER   Distro = "other"
)

func (d Distro) IsRHELDerivative() bool {
	switch d {
	case DISTRO_CENTOS, DISTRO_ORACLE, DISTRO_RHEL, DISTRO_ROCKY, DISTRO_FEDORA, DISTRO_AMZN, DISTRO_ALMA:
		return true
	default:
		return false
	}
}

func ValidateOSType(os string) error {
	switch OSType(os) {
	case OSTYPE_FORTIGATE:
	case OSTYPE_LINUX:
	case OSTYPE_WINDOWS:
	case OSTYPE_BSD:
	default:
		return fmt.Errorf("Unknown OS type %q", os)
	}

	return nil
}

func ValidateDistribution(osType OSType, distro string) error {
	switch Distro(distro) {
	case DISTRO_ALMA:
	case DISTRO_ARCH:
	case DISTRO_AMZN:
	case DISTRO_CENTOS:
	case DISTRO_DEBIAN:
	case DISTRO_FEDORA:
	case DISTRO_FREEBSD:
		if osType != OSTYPE_BSD {
			return fmt.Errorf("Distribution %q is only compatible with OS type %q, not %q", distro, OSTYPE_BSD, osType)
		}

	case DISTRO_ORACLE:
	case DISTRO_OTHER:
	case DISTRO_RHEL:
	case DISTRO_ROCKY:
	case DISTRO_SUSE:
	case DISTRO_UBUNTU:
	default:
		return fmt.Errorf("Unknown OS distribution %q", distro)
	}

	return nil
}
