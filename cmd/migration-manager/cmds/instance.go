package cmds

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/units"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdInstance struct {
	Global *CmdGlobal
}

func (c *CmdInstance) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "instance"
	cmd.Short = "Interact with migration instances"
	cmd.Long = `Description:
  Interact with migration instances

  View and perform limited configuration of instances used by the migration manager.
`

	// List
	instanceListCmd := cmdInstanceList{global: c.Global}
	cmd.AddCommand(instanceListCmd.Command())

	// Set migration state
	instanceSetMigrationStateCmd := cmdInstanceSetMigrationState{global: c.Global}
	cmd.AddCommand(instanceSetMigrationStateCmd.Command())

	// Override
	instanceOverrideCmd := CmdInstanceOverride{Global: c.Global}
	cmd.AddCommand(instanceOverrideCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// List the instances.
type cmdInstanceList struct {
	global *CmdGlobal

	flagFormat  string
	flagVerbose bool
}

func (c *cmdInstanceList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List available instances"
	cmd.Long = `Description:
  List the available instances
`

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)
	cmd.Flags().BoolVarP(&c.flagVerbose, "verbose", "", false, "Enable verbose output")
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateFlagFormat(cmd.Flag("format").Value.String())
	}

	return cmd
}

func (c *cmdInstanceList) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	// Get the list of all instances.
	resp, err := c.global.doHTTPRequestV1("/instances", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	instances := []api.Instance{}

	err = responseToStruct(resp, &instances)
	if err != nil {
		return err
	}

	// Get nice names for the batches.
	batches := []api.Batch{}
	batchesMap := make(map[int]string)
	resp, err = c.global.doHTTPRequestV1("/batches", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	err = responseToStruct(resp, &batches)
	if err != nil {
		return err
	}

	for _, b := range batches {
		batchesMap[b.DatabaseID] = b.Name
	}

	// Get nice names for the sources.
	sources := []api.Source{}
	sourcesMap := make(map[int]string)
	resp, err = c.global.doHTTPRequestV1("/sources", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	err = responseToStruct(resp, &sources)
	if err != nil {
		return err
	}

	for _, s := range sources {
		sourcesMap[s.DatabaseID] = s.Name
	}

	// Get nice names for the targets.
	targets := []api.IncusTarget{}
	targetsMap := make(map[int]string)
	resp, err = c.global.doHTTPRequestV1("/targets", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	err = responseToStruct(resp, &targets)
	if err != nil {
		return err
	}

	for _, t := range targets {
		targetsMap[t.DatabaseID] = t.Name
	}

	// Render the table.
	header := []string{"Name", "Source", "Target", "Batch", "Migration Status", "OS", "OS Version", "Num vCPUs", "Memory", "Disk", "NIC"}
	if c.flagVerbose {
		header = append(header, "UUID", "Inventory Path", "Last Sync")
	}

	data := [][]string{}

	for _, i := range instances {
		// Get the instance override, if any.
		override := api.InstanceOverride{}
		resp, err := c.global.doHTTPRequestV1("/instances/"+i.UUID.String()+"/override", http.MethodGet, "", nil)
		if err == nil {
			err = responseToStruct(resp, &override)
			if err != nil {
				return err
			}
		}

		if override.NumberCPUs != 0 {
			i.NumberCPUs = override.NumberCPUs
		}

		if override.MemoryInBytes != 0 {
			i.MemoryInBytes = override.MemoryInBytes
		}

		disks := []string{}
		for _, disk := range i.Disks {
			disks = append(disks, disk.Name+" ("+units.GetByteSizeStringIEC(disk.SizeInBytes, 2)+")")
		}

		nics := []string{}
		for _, nic := range i.NICs {
			nics = append(nics, nic.Hwaddr+" ("+nic.Network+")")
		}

		if i.MigrationStatusString == "" {
			i.MigrationStatusString = i.MigrationStatus.String()
		}

		row := []string{i.Name, sourcesMap[i.SourceID], getFrom(targetsMap, i.TargetID), getFrom(batchesMap, i.BatchID), i.MigrationStatusString, i.OS, i.OSVersion, strconv.Itoa(i.NumberCPUs), units.GetByteSizeStringIEC(i.MemoryInBytes, 2), strings.Join(disks, "\n"), strings.Join(nics, "\n")}

		if c.flagVerbose {
			row = append(row, i.UUID.String(), i.InventoryPath, i.LastUpdateFromSource.String())
		}

		data = append(data, row)
	}

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, instances)
}

// Disable instance for migration.
type cmdInstanceSetMigrationState struct {
	global *CmdGlobal

	flagUserDisabled bool
}

func (c *cmdInstanceSetMigrationState) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "set-state <instance-uuid>"
	cmd.Short = "Update instance migration state"
	cmd.Long = `Description:
  Set the migration state for the given instance
`

	cmd.RunE = c.Run
	cmd.Flags().BoolVarP(&c.flagUserDisabled, "user-disabled", "", false, "Set migration state for instance to disabled by user")

	return cmd
}

func (c *cmdInstanceSetMigrationState) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	instanceUUID, err := uuid.Parse(args[0])
	if err != nil {
		return err
	}

	// Set instance state.
	_, err = c.global.doHTTPRequestV1(
		fmt.Sprintf("/instances/%s/state", instanceUUID.String()),
		http.MethodPut,
		fmt.Sprintf("migration_user_disabled=%t", c.flagUserDisabled),
		nil,
	)
	if err != nil {
		return err
	}

	return nil
}

func getFrom(lookupMap map[int]string, key *int) string {
	if key == nil {
		return ""
	}

	return lookupMap[*key]
}
