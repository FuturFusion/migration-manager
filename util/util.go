package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lxc/incus/v6/shared/api"
	"github.com/vmware/govmomi/vim25/mo"
)

type DiskInfo struct {
	Name string
	Size int64
}

type NICInfo struct {
	Network string
	Hwaddr  string
}

func ConvertVMwareMetadataToIncus(vm mo.VirtualMachine) api.InstancesPost {
	// Some information, such as the VM's architecture, appears to only available via VMware's guest tools integration(?)
	guestInfo := make(map[string]string)
	if vm.Config.ExtraConfig != nil {
		for _, v := range vm.Config.ExtraConfig {
			if v.GetOptionValue().Key == "guestInfo.detailed.data" {
				re := regexp.MustCompile(`architecture='(.+)' bitness='(\d+)'`)
				matches := re.FindStringSubmatch(v.GetOptionValue().Value.(string))
				if matches != nil {
					guestInfo["architecture"] = matches[1]
					guestInfo["bits"] = matches[2]
				}
				break
			}
		}
	}

	ret := api.InstancesPost{
		Name: vm.Summary.Config.Name,
                Source: api.InstanceSource{
                        Type: "none",
                },
		Type: api.InstanceTypeVM,
        }

	if guestInfo["architecture"] == "X86" {
		if guestInfo["bits"] == "64" {
			ret.Architecture = "x86_64"
		} else {
			ret.Architecture = "i686"
		}
	} else if guestInfo["architecture"] == "Arm" {
		if guestInfo["bits"] == "64" {
			ret.Architecture = "aarch64"
		} else {
			ret.Architecture = "armv8l"
		}
	} else {
		fmt.Printf("  WARNING: Defaulting architecture to x86_64 (got %s/%s)\n", guestInfo["architecture"], guestInfo["bits"])
		ret.Architecture = "x86_64"
        }

        ret.Config = make(map[string]string)
        ret.Devices = make(map[string]map[string]string)

	// Set basic config fields.
        ret.Config["image.architecture"] = ret.Architecture
        ret.Config["image.description"] = "Auto-imported from VMware"
        ret.Config["image.os"] = strings.TrimSuffix(vm.Summary.Config.GuestId, "Guest")
        ret.Config["image.release"] = vm.Summary.Config.GuestFullName

	// Apply CPU and memory limits.
        ret.Config["limits.cpu"] = fmt.Sprintf("%d", vm.Summary.Config.NumCpu)
        ret.Config["limits.memory"] = fmt.Sprintf("%dMiB", vm.Summary.Config.MemorySizeMB)

	// Add TPM if needed.
	if *vm.Capability.SecureBootSupported {
		ret.Devices["vtpm"] = map[string]string{
			"type": "tpm",
			"path": "/dev/tpm0",
		}
	}

	// Handle VMs without UEFI and/or secure boot.
	if vm.Config.Firmware == "bios" {
		ret.Config["security.csm"] = "true"
		ret.Config["security.secureboot"] = "false"
	} else if !*vm.Capability.SecureBootSupported {
		ret.Config["security.secureboot"] = "false"
	}

	ret.Description = ret.Config["image.description"]

	// Handle Windows-specific configuration.
	if strings.Contains(ret.Config["image.os"], "windows") {
		// Set some additional QEMU options.
		ret.Config["raw.qemu"] = "-device intel-hda -device hda-duplex -audio spice"

		// If image.os contains the string "Windows", then the incus-agent config drive won't be mapped.
		// But we need that when running the initial migration logic from the ISO image. Reverse the string
		// for now, and un-reverse it before finalizing the VM.
		ret.Config["image.os"] = strings.Replace(ret.Config["image.os"], "windows", "swodniw", 1)
	}

	return ret
}
