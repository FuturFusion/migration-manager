package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type cmdInstance struct {
	global *cmdGlobal
}

func (c *cmdInstance) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "instance"
	cmd.Short = "Interact with migration instances"
	cmd.Long = `Description:
  Interact with migration instances

  View and perform limited configuration of instances used by the migration manager.
`

	// List
	instanceListCmd := cmdInstanceList{global: c.global}
	cmd.AddCommand(instanceListCmd.Command())

	// Update
	instanceUpdateCmd := cmdInstanceUpdate{global: c.global}
	cmd.AddCommand(instanceUpdateCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

// List
type cmdInstanceList struct {
	global *cmdGlobal

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
	resp, err := c.global.doHttpRequestV1("/instances", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	instances := []api.Instance{}

	// Loop through returned instances.
	for _, anyInstance := range resp.Metadata.([]any) {
		newInstance, err := parseReturnedInstance(anyInstance)
		if err != nil {
			return err
		}
		instances = append(instances, newInstance.(api.Instance))
	}

	// Get nice names for the batches.
	batchesMap := make(map[int]string)
	batchesMap[internal.INVALID_DATABASE_ID] = ""
	resp, err = c.global.doHttpRequestV1("/batches", http.MethodGet, "", nil)
	if err != nil {
		return err
	}
	for _, anyBatch := range resp.Metadata.([]any) {
		newBatch, err := parseReturnedBatch(anyBatch)
		if err != nil {
			return err
		}
		b := newBatch.(api.Batch)
		batchesMap[b.DatabaseID] = b.Name
	}

	// Get nice names for the sources.
	sourcesMap := make(map[int]string)
	resp, err = c.global.doHttpRequestV1("/sources", http.MethodGet, "", nil)
	if err != nil {
		return err
	}
	for _, anySource := range resp.Metadata.([]any) {
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
	resp, err = c.global.doHttpRequestV1("/targets", http.MethodGet, "", nil)
	if err != nil {
		return err
	}
	for _, anyTarget := range resp.Metadata.([]any) {
		newTarget, err := parseReturnedTarget(anyTarget)
		if err != nil {
			return err
		}
		t := newTarget.(api.IncusTarget)
		targetsMap[t.DatabaseID] = t.Name
	}

	// Render the table.
	header := []string{"Name", "Source", "Target", "Batch", "Migration Status", "OS", "OS Version", "Num vCPUs", "Memory (MiB)"}
	if c.flagVerbose {
		header = append(header, "UUID", "Last Sync", "Last Manual Update")
	}
	data := [][]string{}

	for _, i := range instances {
		row := []string{i.Name, sourcesMap[i.SourceID], targetsMap[i.TargetID], batchesMap[i.BatchID], i.MigrationStatusString, i.OS, i.OSVersion, strconv.Itoa(i.NumberCPUs), strconv.Itoa(i.MemoryInMiB)}
		if c.flagVerbose {
			lastUpdate := "Never"
			if !i.LastManualUpdate.IsZero() {
				lastUpdate = i.LastManualUpdate.String()
			}
			row = append(row, i.UUID.String(), i.LastUpdateFromSource.String(), lastUpdate)
		}
		data = append(data, row)
	}

	return util.RenderTable(c.flagFormat, header, data, instances)
}

func parseReturnedInstance(i any) (any, error) {
	reJsonified, err := json.Marshal(i)
	if err != nil {
		return nil, err
	}

	var ret = api.Instance{}
	err = json.Unmarshal(reJsonified, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Update
type cmdInstanceUpdate struct {
	global *cmdGlobal
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
	resp, err := c.global.doHttpRequestV1("/instances/"+UUIDString, http.MethodGet, "", nil)
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
		val, err := c.global.asker.AskInt("Number of vCPUs: ["+strconv.Itoa(inst.NumberCPUs)+"] ", 1, 1024, strconv.Itoa(inst.NumberCPUs), nil)
		if err != nil {
			return err
		}
		if inst.NumberCPUs != int(val) {
			inst.NumberCPUs = int(val)
			inst.LastManualUpdate = time.Now().UTC()
		}

		val, err = c.global.asker.AskInt("Memory in MiB: ["+strconv.Itoa(inst.MemoryInMiB)+"] ", 1, 1024*1024*1024, strconv.Itoa(inst.MemoryInMiB), nil)
		if err != nil {
			return err
		}
		if inst.MemoryInMiB != int(val) {
			inst.MemoryInMiB = int(val)
			inst.LastManualUpdate = time.Now().UTC()
		}

		i = inst
	}

	content, err := json.Marshal(i)
	if err != nil {
		return err
	}

	_, err = c.global.doHttpRequestV1("/instances/"+UUIDString, http.MethodPut, "", content)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully updated instance '%s'.\n", UUIDString)
	return nil
}
