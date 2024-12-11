package cmds

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/lxc/incus/v6/shared/units"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal"
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

	// Update
	instanceUpdateCmd := cmdInstanceUpdate{global: c.Global}
	cmd.AddCommand(instanceUpdateCmd.Command())

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
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", "Format (csv|json|table|yaml|compact)")
	cmd.Flags().BoolVarP(&c.flagVerbose, "verbose", "", false, "Enable verbose output")

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

	metadata, ok := resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	// Loop through returned instances.
	for _, anyInstance := range metadata {
		newInstance, err := parseReturnedInstance(anyInstance)
		if err != nil {
			return err
		}

		instances = append(instances, newInstance.(api.Instance))
	}

	// Get nice names for the batches.
	batchesMap := make(map[int]string)
	batchesMap[internal.INVALID_DATABASE_ID] = ""
	resp, err = c.global.doHTTPRequestV1("/batches", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	metadata, ok = resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	for _, anyBatch := range metadata {
		newBatch, err := parseReturnedBatch(anyBatch)
		if err != nil {
			return err
		}

		b, ok := newBatch.(api.Batch)
		if !ok {
			return errors.New("Invalid type for batch")
		}

		batchesMap[b.DatabaseID] = b.Name
	}

	// Get nice names for the sources.
	sourcesMap := make(map[int]string)
	resp, err = c.global.doHTTPRequestV1("/sources", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	metadata, ok = resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	for _, anySource := range metadata {
		newSource, err := parseReturnedSource(anySource)
		if err != nil {
			return err
		}

		switch s := newSource.(type) {
		case api.VMwareSource:
			sourcesMap[s.DatabaseID] = s.Name
		}
	}

	// Get nice names for the targets.
	targetsMap := make(map[int]string)
	resp, err = c.global.doHTTPRequestV1("/targets", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	metadata, ok = resp.Metadata.([]any)
	if !ok {
		return errors.New("Unexpected API response, invalid type for metadata")
	}

	for _, anyTarget := range metadata {
		newTarget, err := parseReturnedTarget(anyTarget)
		if err != nil {
			return err
		}

		t, ok := newTarget.(api.IncusTarget)
		if !ok {
			return errors.New("Invalid type for target")
		}

		targetsMap[t.DatabaseID] = t.Name
	}

	// Render the table.
	header := []string{"Name", "Source", "Target", "Batch", "Migration Status", "OS", "OS Version", "Num vCPUs", "Memory"}
	if c.flagVerbose {
		header = append(header, "UUID", "Inventory Path", "Last Sync", "Last Manual Update")
	}

	data := [][]string{}

	for _, i := range instances {
		row := []string{i.Name, sourcesMap[i.SourceID], targetsMap[i.TargetID], batchesMap[i.BatchID], i.MigrationStatusString, i.OS, i.OSVersion, strconv.Itoa(i.NumberCPUs), units.GetByteSizeStringIEC(i.MemoryInBytes, 2)}
		if c.flagVerbose {
			lastUpdate := "Never"
			if !i.LastManualUpdate.IsZero() {
				lastUpdate = i.LastManualUpdate.String()
			}

			row = append(row, i.UUID.String(), i.InventoryPath, i.LastUpdateFromSource.String(), lastUpdate)
		}

		data = append(data, row)
	}

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, instances)
}

func parseReturnedInstance(i any) (any, error) {
	reJsonified, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	ret := api.Instance{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Update the instance.
type cmdInstanceUpdate struct {
	global *CmdGlobal
}

func (c *cmdInstanceUpdate) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "update <uuid>"
	cmd.Short = "Update instance"
	cmd.Long = `Description:
  Update instance

  Only a few fields can be updated, such as the number of vCPUs or memory. Updating
  other values must be done on through the UI/API of the instance's Source.
`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdInstanceUpdate) Run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	UUIDString := args[0]

	// Get the existing instance.
	resp, err := c.global.doHTTPRequestV1("/instances/"+UUIDString, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	i, err := parseReturnedInstance(resp.Metadata)
	if err != nil {
		return err
	}

	// Prompt for updates.
	switch inst := i.(type) {
	case api.Instance:
		val, err := c.global.Asker.AskInt("Number of vCPUs: ["+strconv.Itoa(inst.NumberCPUs)+"] ", 1, 1024, strconv.Itoa(inst.NumberCPUs), nil)
		if err != nil {
			return err
		}

		if inst.NumberCPUs != int(val) {
			inst.NumberCPUs = int(val)
			inst.LastManualUpdate = time.Now().UTC()
		}

		val, err = c.global.Asker.AskInt("Memory in bytes: ["+strconv.FormatInt(inst.MemoryInBytes, 10)+"] ", 1, 1024*1024*1024*1024*1024, strconv.FormatInt(inst.MemoryInBytes, 10), nil)
		if err != nil {
			return err
		}

		if inst.MemoryInBytes != val {
			inst.MemoryInBytes = val
			inst.LastManualUpdate = time.Now().UTC()
		}

		i = inst
	}

	content, err := json.Marshal(i)
	if err != nil {
		return err
	}

	_, err = c.global.doHTTPRequestV1("/instances/"+UUIDString, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	cmd.Printf("Successfully updated instance '%s'.\n", UUIDString)
	return nil
}
