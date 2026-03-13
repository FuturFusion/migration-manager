package api

import (
	"fmt"
)

type OSType string

const (
	OSTYPE_WINDOWS   OSType = "windows"
	OSTYPE_LINUX     OSType = "linux"
	OSTYPE_FORTIGATE OSType = "fortigate"
)

type Distro string

const (
	DISTRO_DEBIAN Distro = "debian"
	DISTRO_UBUNTU Distro = "ubuntu"
	DISTRO_ORACLE Distro = "oracle"
	DISTRO_CENTOS Distro = "centos"
	DISTRO_RHEL   Distro = "rhel"
	DISTRO_SUSE   Distro = "suse"
	DISTRO_OTHER  Distro = "other"
)

func ValidateOSType(os string) error {
	switch OSType(os) {
	case OSTYPE_FORTIGATE:
	case OSTYPE_LINUX:
	case OSTYPE_WINDOWS:
	default:
		return fmt.Errorf("Unknown OS type %q", os)
	}

	return nil
}

func ValidateDistribution(distro string) error {
	switch Distro(distro) {
	case DISTRO_CENTOS:
	case DISTRO_DEBIAN:
	case DISTRO_ORACLE:
	case DISTRO_OTHER:
	case DISTRO_RHEL:
	case DISTRO_SUSE:
	case DISTRO_UBUNTU:
	default:
		return fmt.Errorf("Unknown OS distribution %q", distro)
	}

	return nil
}
