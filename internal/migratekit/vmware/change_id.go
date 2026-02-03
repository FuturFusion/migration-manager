package vmware

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/vim25/types"
)

var ErrInvalidChangeID = errors.New("invalid change ID")

type ChangeID struct {
	UUID   string
	Number string
	Value  string
}

func ParseChangeID(changeId string) (*ChangeID, error) {
	changeIdParts := strings.Split(changeId, "/")
	if len(changeIdParts) != 2 {
		return nil, ErrInvalidChangeID
	}

	return &ChangeID{
		UUID:   changeIdParts[0],
		Number: changeIdParts[1],
		Value:  changeId,
	}, nil
}

func GetChangeID(disk *types.VirtualDisk) (*ChangeID, error) {
	var changeId string

	if b, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
		changeId = b.ChangeId
	} else if b, ok := disk.Backing.(*types.VirtualDiskSparseVer2BackingInfo); ok {
		changeId = b.ChangeId
	} else if b, ok := disk.Backing.(*types.VirtualDiskRawDiskMappingVer1BackingInfo); ok {
		changeId = b.ChangeId
	} else if b, ok := disk.Backing.(*types.VirtualDiskRawDiskVer2BackingInfo); ok {
		changeId = b.ChangeId
	} else {
		return nil, errors.New("failed to get change ID")
	}

	if changeId == "" {
		return nil, fmt.Errorf("CBT is not enabled on disk %d", disk.Key)
	}

	return ParseChangeID(changeId)
}

// IsSupportedDisk checks whether the given VMware disk is supported by migration manager.
func IsSupportedDisk(disk *types.VirtualDisk) (string, []string, error) {
	isSupported := func(diskMode string, sharing string) error {
		if diskMode == string(types.VirtualDiskModeIndependent_persistent) || diskMode == string(types.VirtualDiskModeIndependent_nonpersistent) {
			return fmt.Errorf("Disk does not support snapshots")
		}

		if sharing == "" || sharing == string(types.VirtualDiskSharingSharingNone) {
			return nil
		}

		return fmt.Errorf("Disk has sharing enabled")
	}

	// Ignore raw disks or disks that are excluded from snapshots.
	snapshots := []string{}
	switch t := disk.GetVirtualDevice().Backing.(type) {
	case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
		for t.Parent != nil {
			snapshots = append(snapshots, t.FileName)
			t = t.Parent
		}

		return t.FileName, snapshots, fmt.Errorf("Raw disk cannot be migrated")
	case *types.VirtualDiskRawDiskVer2BackingInfo:
		return t.DeviceName, snapshots, fmt.Errorf("Raw disk cannot be migrated")
	case *types.VirtualDiskPartitionedRawDiskVer2BackingInfo:
		return t.DeviceName, snapshots, fmt.Errorf("Raw disk cannot be migrated")
	case *types.VirtualDiskFlatVer2BackingInfo:
		for t.Parent != nil {
			snapshots = append(snapshots, t.FileName)
			t = t.Parent
		}

		return t.FileName, snapshots, isSupported(t.DiskMode, t.Sharing)
	case *types.VirtualDiskFlatVer1BackingInfo:
		for t.Parent != nil {
			snapshots = append(snapshots, t.FileName)
			t = t.Parent
		}

		return t.FileName, snapshots, isSupported(t.DiskMode, "")
	case *types.VirtualDiskLocalPMemBackingInfo:
		return t.FileName, snapshots, isSupported(t.DiskMode, "")
	case *types.VirtualDiskSeSparseBackingInfo:
		for t.Parent != nil {
			snapshots = append(snapshots, t.FileName)
			t = t.Parent
		}

		return t.FileName, snapshots, isSupported(t.DiskMode, "")
	case *types.VirtualDiskSparseVer1BackingInfo:
		for t.Parent != nil {
			snapshots = append(snapshots, t.FileName)
			t = t.Parent
		}

		return t.FileName, snapshots, isSupported(t.DiskMode, "")
	case *types.VirtualDiskSparseVer2BackingInfo:
		for t.Parent != nil {
			snapshots = append(snapshots, t.FileName)
			t = t.Parent
		}

		return t.FileName, snapshots, isSupported(t.DiskMode, "")
	default:
		return "unknown", snapshots, fmt.Errorf("Unknown disk type %T", t)
	}
}
