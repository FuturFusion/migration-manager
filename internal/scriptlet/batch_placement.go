package scriptlet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/scriptlet"
	"github.com/lxc/incus/v6/shared/validate"
	"go.starlark.net/starlark"

	"github.com/FuturFusion/migration-manager/shared/api"
)

func BatchPlacementRun(ctx context.Context, loader *scriptlet.Loader, instance api.Instance, batch api.Batch, usedNetworks []api.Network) (*api.Placement, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logFunc := CreateLogger(slog.Default(), "Batch placement scriptlet")

	var resp api.Placement
	setTargetFunc := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var targetName string
		err := starlark.UnpackArgs(b.Name(), args, kwargs, "target_name", &targetName)
		if err != nil {
			return nil, err
		}

		if targetName == "" {
			slog.Error("Batch placement failed. Target name is empty")
			return nil, errors.New("Target name is empty")
		}

		resp.TargetName = targetName
		slog.Info("Batch placement assigned target for instance", slog.String("location", instance.Location), slog.String("target", resp.TargetName))

		return starlark.None, nil
	}

	setProjectFunc := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var projectName string
		err := starlark.UnpackArgs(b.Name(), args, kwargs, "project_name", &projectName)
		if err != nil {
			return nil, err
		}

		if projectName == "" {
			slog.Error("Batch placement failed. Target project name is empty")
			return nil, errors.New("Target project name is empty")
		}

		resp.TargetProject = projectName
		slog.Info("Batch placement assigned project for instance", slog.String("location", instance.Location), slog.String("project", resp.TargetProject))

		return starlark.None, nil
	}

	setPoolFunc := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var diskName string
		var poolName string
		err := starlark.UnpackArgs(b.Name(), args, kwargs, "disk_name", &diskName, "pool_name", &poolName)
		if err != nil {
			return nil, err
		}

		if poolName == "" {
			slog.Error("Batch placement failed. Target storage pool name is empty")
			return nil, errors.New("Target storage pool name is empty")
		}

		var diskExists bool
		for _, d := range instance.Disks {
			if d.Name == diskName && d.Supported {
				diskExists = true
				break
			}
		}

		if !diskExists {
			return nil, fmt.Errorf("No disk found with name %q on instance %q", diskName, instance.Location)
		}

		if resp.StoragePools == nil {
			resp.StoragePools = map[string]string{}
		}

		resp.StoragePools[diskName] = poolName
		slog.Info("Batch placement assigned storage pool for instance", slog.String("location", instance.Location), slog.String("disk", diskName), slog.String("pool", poolName))

		return starlark.None, nil
	}

	setNetworkFunc := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var nicMac string
		var netName string
		var nicType string
		var vlanID string
		err := starlark.UnpackArgs(b.Name(), args, kwargs, "nic_hwaddr", &nicMac, "network_name", &netName, "nic_type", &nicType, "vlan_id", &vlanID)
		if err != nil {
			return nil, err
		}

		if nicType == "" {
			nicType = string(api.INCUSNICTYPE_MANAGED)
		}

		err = api.ValidNICType(nicType)
		if err != nil {
			slog.Error("Batch placement failed. Invalid NIC type", slog.String("nic_type", nicType))
			return nil, err
		}

		if vlanID != "" {
			for _, id := range strings.Split(vlanID, ",") {
				intID, err := strconv.Atoi(id)
				if err != nil || intID == 0 {
					slog.Error("Batch placement failed. VLAN ID is invalid")
					return nil, fmt.Errorf("Invalid vlan ID %q: %w", vlanID, err)
				}
			}
		}

		if netName == "" {
			slog.Error("Batch placement failed. Target network name is empty")
			return nil, errors.New("Target network name is empty")
		}

		var nicExists bool
		for _, d := range instance.NICs {
			if d.HardwareAddress == nicMac {
				nicExists = true
				break
			}
		}

		if !nicExists {
			return nil, fmt.Errorf("No NIC found with hardware address %q on instance %q", nicMac, instance.Location)
		}

		if resp.Networks == nil {
			resp.Networks = map[string]api.NetworkPlacement{}
		}

		placement := api.NetworkPlacement{
			Network: netName,
			NICType: api.IncusNICType(nicType),
			VlanID:  vlanID,
		}

		err = validate.IsAPIName(placement.Network, false)
		if err != nil {
			return nil, fmt.Errorf("Invalid network name %q: %v", placement.Network, err)
		}

		err = placement.Validate()
		if err != nil {
			return nil, fmt.Errorf("Failed to validate network placement: %w", err)
		}

		resp.Networks[nicMac] = placement

		slog.Info("Batch placement assigned network for instance", slog.String("location", instance.Location), slog.String("nic", nicMac), slog.String("network", netName), slog.String("nic_type", nicType), slog.String("vlan_id", vlanID))

		return starlark.None, nil
	}

	env := starlark.StringDict{
		"log_info":  starlark.NewBuiltin("log_info", logFunc),
		"log_warn":  starlark.NewBuiltin("log_warn", logFunc),
		"log_error": starlark.NewBuiltin("log_error", logFunc),

		"set_target":  starlark.NewBuiltin("set_target", setTargetFunc),
		"set_project": starlark.NewBuiltin("set_project", setProjectFunc),
		"set_pool":    starlark.NewBuiltin("set_pool", setPoolFunc),
		"set_network": starlark.NewBuiltin("set_network", setNetworkFunc),
	}

	prog, thread, err := BatchPlacementProgram(loader, batch.Name)
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		thread.Cancel("Request finished")
	}()

	globals, err := prog.Init(thread, env)
	if err != nil {
		return nil, fmt.Errorf("Failed initializing: %w", err)
	}

	globals.Freeze()

	// Retrieve a global variable from starlark environment.
	placement := globals[batchPlacementFunc]
	if placement == nil {
		return nil, fmt.Errorf("Scriptlet missing %q function", batchPlacementFunc)
	}

	instv, err := scriptlet.StarlarkMarshal(instance)
	if err != nil {
		return nil, fmt.Errorf("Marshalling request failed: %w", err)
	}

	batchv, err := scriptlet.StarlarkMarshal(batch)
	if err != nil {
		return nil, fmt.Errorf("Marshalling request failed: %w", err)
	}

	v, err := starlark.Call(thread, placement, nil, []starlark.Tuple{
		{starlark.String("instance"), instv},
		{starlark.String("batch"), batchv},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to run: %w", err)
	}

	if v.Type() != "NoneType" {
		return nil, fmt.Errorf("Failed with unexpected return value: %v", v)
	}

	return &resp, nil
}
