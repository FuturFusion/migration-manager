package cmds

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

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
	resp, err := c.global.doHTTPRequestV1("/instances", http.MethodGet, "recursion=1", nil)
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
	batchesMap := make(map[string]string)
	resp, err = c.global.doHTTPRequestV1("/batches", http.MethodGet, "recursion=1", nil)
	if err != nil {
		return err
	}

	err = responseToStruct(resp, &batches)
	if err != nil {
		return err
	}

	for _, b := range batches {
		batchesMap[b.Name] = b.Name
	}

	// Render the table.
	header := []string{"UUID", "Source", "Location", "OS Version", "CPUs", "Memory", "Background Import", "Migration Status"}
	if c.flagVerbose {
		header = []string{"UUID", "Location", "Description", "Source", "Batch", "Migration Status", "Migration Status String", "Last Update from Source", "Architecture", "OS", "OS Version", "Disks", "NICs", "Snapshots", "CPUs", "Memory", "Legacy Boot", "Secure Boot", "TPM", "Background Import"}
	}

	data := [][]string{}

	for _, i := range instances {
		props := i.Properties
		if i.Overrides != nil {
			props.Apply(i.Overrides.Properties)
		}

		if i.MigrationStatusString == "" {
			i.MigrationStatusString = i.MigrationStatus.String()
		}

		row := []string{props.UUID.String(), i.Source, props.Location, props.OSVersion, strconv.Itoa(int(props.CPUs)), units.GetByteSizeStringIEC(props.Memory, 2), strconv.FormatBool(props.BackgroundImport), i.MigrationStatusString}

		if c.flagVerbose {
			disks := []string{}
			for _, disk := range props.Disks {
				disks = append(disks, disk.Name+": ("+units.GetByteSizeStringIEC(disk.Capacity, 2)+")")
			}

			nics := []string{}
			for _, nic := range props.NICs {
				nics = append(nics, nic.HardwareAddress+" ("+nic.Network+", "+nic.ID+")")
			}

			snapshots := []string{}
			for _, snapshot := range props.Snapshots {
				snapshots = append(snapshots, snapshot.Name)
			}

			row = []string{props.UUID.String(), props.Location, props.Description, i.Source, getFrom(batchesMap, i.Batch), i.MigrationStatus.String(), i.MigrationStatusString, i.LastUpdateFromSource.String(), props.Architecture, props.OS, props.OSVersion, strings.Join(disks, "\n"), strings.Join(nics, "\n"), strings.Join(snapshots, "\n"), strconv.Itoa(int(props.CPUs)), units.GetByteSizeStringIEC(props.Memory, 2), strconv.FormatBool(props.LegacyBoot), strconv.FormatBool(props.SecureBoot), strconv.FormatBool(props.TPM), strconv.FormatBool(props.BackgroundImport)}
		}

		data = append(data, row)
	}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, instances)
}

func getFrom(lookupMap map[string]string, key *string) string {
	if key == nil {
		return ""
	}

	return lookupMap[*key]
}
