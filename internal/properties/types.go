package properties

import (
	"fmt"

	"github.com/FuturFusion/migration-manager/shared/api"
)

// PropertyType is the type of property on the source or target.
type PropertyType string

const (
	// TypeVMInfo represents top-level VM information from VMware.
	TypeVMInfo PropertyType = "vm_info"

	// TypeGuestInfo represents the guestInfo key-value pairs from VMware.
	TypeGuestInfo PropertyType = "guest_info"

	// TypeVMProperty represents the VM property configuration for VMware.
	TypeVMProperty PropertyType = "property"

	// TypeVMPropertyDisk represents a VM's virtual disk configuration for VMware.
	TypeVMPropertyDisk PropertyType = "property_disk"

	// TypeVMPropertyEthernet represents a VM's ethernet configuration for VMware.
	TypeVMPropertyEthernet PropertyType = "property_ethernet"

	// TypeVMPropertySnapshot represents a VM's snapshot configuration for VMware.
	TypeVMPropertySnapshot PropertyType = "property_snapshot"

	// TypeConfig represents Incus instance config.
	TypeConfig PropertyType = "config"

	// TypeDisk represents Incus disk device config.
	TypeDisk PropertyType = "disk"

	// TypeNIC represents Incus nic device config.
	TypeNIC PropertyType = "nic"

	// TypeTPM represents Incus tpm device config.
	TypeTPM PropertyType = "tpm"
)

func allPropertyTypes[T api.TargetType | api.SourceType](t T) ([]PropertyType, error) {
	switch t := any(t).(type) {
	case api.TargetType:
		if t == api.TARGETTYPE_INCUS {
			return []PropertyType{TypeDisk, TypeNIC, TypeTPM, TypeConfig}, nil
		}

	case api.SourceType:
		if t == api.SOURCETYPE_VMWARE {
			return []PropertyType{TypeVMInfo, TypeVMProperty, TypeVMPropertyDisk, TypeVMPropertyEthernet, TypeVMPropertySnapshot, TypeGuestInfo}, nil
		}
	}

	return nil, fmt.Errorf("Unsupported source or target %q", t)
}
