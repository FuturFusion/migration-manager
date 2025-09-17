package cmds

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lxc/incus/v6/shared/ioprogress"
	"github.com/lxc/incus/v6/shared/units"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/FuturFusion/migration-manager/internal/util"
	"github.com/FuturFusion/migration-manager/shared/api"
)

type CmdArtifact struct {
	Global *CmdGlobal
}

func (c *CmdArtifact) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "artifact"
	cmd.Short = "Manage artifacts"
	cmd.Long = `Description:

	Upload additional components required for complete migration.
`

	// List
	configListCmd := cmdArtifactList{global: c.Global}
	cmd.AddCommand(configListCmd.Command())

	// Show
	configShowCmd := cmdArtifactShow{global: c.Global}
	cmd.AddCommand(configShowCmd.Command())

	// Export
	configExportCmd := cmdArtifactExport{global: c.Global}
	cmd.AddCommand(configExportCmd.Command())

	// Remove
	configRemoveCmd := cmdArtifactRemove{global: c.Global}
	cmd.AddCommand(configRemoveCmd.Command())

	// Upload
	configUploadCmd := cmdArtifactUpload{global: c.Global}
	cmd.AddCommand(configUploadCmd.Command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }

	return cmd
}

type cmdArtifactList struct {
	global     *CmdGlobal
	flagFormat string
}

func (c *cmdArtifactList) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "list"
	cmd.Short = "List artifacts"

	cmd.RunE = c.Run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", `Format (csv|json|table|yaml|compact), use suffix ",noheader" to disable headers and ",header" to enable if demanded, e.g. csv,header`)
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		return validateFlagFormat(cmd.Flag("format").Value.String())
	}

	return cmd
}

func (c *cmdArtifactList) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 0, 0)
	if exit {
		return err
	}

	resp, _, err := c.global.doHTTPRequestV1("/artifacts", http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	artifacts := []api.Artifact{}
	err = responseToStruct(resp, &artifacts)
	if err != nil {
		return err
	}

	// Render the table.
	header := []string{"UUID", "Type", "Files"}
	data := [][]string{}

	for _, a := range artifacts {
		data = append(data, []string{a.UUID.String(), string(a.Type), strconv.Itoa(len(a.Files))})
	}

	sort.Sort(util.SortColumnsNaturally(data))

	return util.RenderTable(cmd.OutOrStdout(), c.flagFormat, header, data, artifacts)
}

type cmdArtifactShow struct {
	global *CmdGlobal
}

func (c *cmdArtifactShow) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "show <uuid>"
	cmd.Short = "Show an artifact by its UUID"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdArtifactShow) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	artUUID := args[0]
	resp, _, err := c.global.doHTTPRequestV1("/artifacts/"+artUUID, http.MethodGet, "", nil)
	if err != nil {
		return err
	}

	artifact := api.Artifact{}
	err = responseToStruct(resp, &artifact)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(artifact)
	if err != nil {
		return err
	}

	fmt.Println(string(b))

	return nil
}

type cmdArtifactExport struct {
	global *CmdGlobal
}

func (c *cmdArtifactExport) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "export <uuid> <file-name> <file-path>"
	cmd.Short = "Export an artifact by its UUID"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdArtifactExport) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 3, 3)
	if exit {
		return err
	}

	artUUID := args[0]
	fileName := args[1]
	filePath := args[2]

	outFile, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer func() { _ = outFile.Close() }()

	progress := util.ProgressRenderer{
		Format: fmt.Sprintf("Downloading artifact to %q: %%s", filePath),
	}

	_, _, err = c.global.doHTTPRequestV1Writer("/artifacts/"+artUUID+"/files/"+fileName, http.MethodGet, outFile, progress.UpdateProgress)
	if err != nil {
		return err
	}

	return nil
}

type cmdArtifactRemove struct {
	global *CmdGlobal
}

func (c *cmdArtifactRemove) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "export <uuid> <file-name>"
	cmd.Short = "Remove an artifact file"

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdArtifactRemove) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 2, 2)
	if exit {
		return err
	}

	artUUID := args[0]
	fileName := args[1]
	_, _, err = c.global.doHTTPRequestV1("/artifacts/"+artUUID+"/files/"+fileName, http.MethodDelete, "", nil)
	if err != nil {
		return err
	}

	return nil
}

type cmdArtifactUpload struct {
	global *CmdGlobal
}

func (c *cmdArtifactUpload) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "upload <type> <os-name/source-type> <file-path> <architectures> <versions>"
	cmd.Short = "Upload an artifact file"
	cmd.Long = `Description:

	Upload an artifact file with the given set of properties.

	Supported types: sdk, os-image

	'architectures' and 'versions' are a comma delimited list, and only compatible with os-image.
	`

	cmd.RunE = c.Run

	return cmd
}

func (c *cmdArtifactUpload) Run(cmd *cobra.Command, args []string) error {
	exit, err := c.global.CheckArgs(cmd, args, 3, 5)
	if exit {
		return err
	}

	artType := api.ArtifactType(args[0])
	filePath := args[2]

	data := api.ArtifactPost{
		Type:       artType,
		Properties: api.ArtifactProperties{},
	}

	if data.Type == api.ARTIFACTTYPE_OSIMAGE || data.Type == api.ARTIFACTTYPE_DRIVER {
		exit, err := c.global.CheckArgs(cmd, args, 4, 5)
		if exit {
			return err
		}

		data.Properties.OS = api.OSType(args[1])
		data.Properties.Architectures = strings.Split(args[3], ",")
		if len(args) == 5 {
			data.Properties.Versions = strings.Split(args[4], ",")
		}
	} else {
		exit, err := c.global.CheckArgs(cmd, args, 3, 3)
		if exit {
			return err
		}

		data.Properties.SourceType = api.SourceType(args[1])
	}

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, header, err := c.global.doHTTPRequestV1("/artifacts", http.MethodPost, "", b)
	if err != nil {
		return err
	}

	location := header.Get("Location")
	if location == "" {
		return fmt.Errorf("Failed to retrieve response location")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	s, err := file.Stat()
	if err != nil {
		return err
	}

	progress := util.ProgressRenderer{
		Format: fmt.Sprintf("Uploading to artifact %s: %%s", filepath.Base(location)),
	}

	reader := &ioprogress.ProgressReader{
		ReadCloser: file,
		Tracker: &ioprogress.ProgressTracker{
			Length: s.Size(),
			Handler: func(percent int64, speed int64) {
				progress.UpdateProgress(ioprogress.ProgressData{Text: fmt.Sprintf("%d%% (%s/s)", percent, units.GetByteSizeString(speed, 2))})
			},
		},
	}

	location = strings.TrimPrefix(location, "/"+api.APIVersion)
	_, _, err = c.global.doHTTPRequestV1Reader(location+"/files", http.MethodPost, "", reader)
	if err != nil {
		return err
	}

	return nil
}
