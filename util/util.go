package util

import (
	"fmt"
	"regexp"

	"github.com/lxc/incus/v6/shared/api"
	"github.com/vmware/govmomi/vim25/mo"
)

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
	} else {
		// FIXME handle arm
		fmt.Printf("    WARNING -- defaulting architecture to x86_64 (got %s/%s)\n", guestInfo["architecture"], guestInfo["bits"])
		ret.Architecture = "x86_64"
        }

        ret.Config = make(map[string]string)
        ret.Config["limits.cpu"] = fmt.Sprintf("%d", vm.Summary.Config.NumCpu)
        ret.Config["limits.memory"] = fmt.Sprintf("%dMiB", vm.Summary.Config.MemorySizeMB)

	// FIXME handle secure boot config

	return ret
}
