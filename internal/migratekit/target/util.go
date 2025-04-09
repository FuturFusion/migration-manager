package target

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/gosimple/slug"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/FuturFusion/migration-manager/internal/migratekit/vmware"
)

func DiskLabel(vm *object.VirtualMachine, disk *types.VirtualDisk) string {
	return slug.Make(vm.Name() + "-" + strconv.Itoa(int(disk.Key)))
}

func NeedsFullCopy(ctx context.Context, t Target) (bool, bool, error) {
	exists, err := t.Exists(ctx)
	if err != nil {
		return false, false, err
	}

	if !exists {
		slog.Info("Data does not exist, full copy needed")

		return true, true, nil
	}

	currentChangeId, err := t.GetCurrentChangeID(ctx)
	if err != nil && !errors.Is(err, vmware.ErrInvalidChangeID) {
		return false, false, err
	}

	if currentChangeId == nil {
		slog.Info("No or invalid change ID found, assuming full copy is needed")

		return true, false, nil
	}

	snapshotChangeId, err := vmware.GetChangeID(t.GetDisk())
	if err != nil {
		return false, false, err
	}

	if currentChangeId.UUID != snapshotChangeId.UUID {
		slog.Warn("Change ID mismatch, full copy needed", slog.String("currentChangeID", currentChangeId.Value), slog.String("snapshotChangeId", snapshotChangeId.Value))
		return true, false, nil
	}

	slog.Info("Starting incremental copy")

	return false, false, nil
}
