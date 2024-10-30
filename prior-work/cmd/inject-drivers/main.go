package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/flosch/pongo2/v4"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/windows"
	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/cmd/common"
)

type appFlags struct {
	driversSourcePath   string
	windowsArchitecture string
	windowsSourcePath   string
	windowsRESourcePath string
	windowsVersion      string
}

func init() {
	// Filters should be registered in the init() function
	_ = pongo2.RegisterFilter("toHex", toHex)
}

func main() {
	appCmd := appFlags{}
	app := appCmd.Command()
	app.SilenceUsage = true
	app.CompletionOptions = cobra.CompletionOptions{DisableDefaultCmd: true}

	// Workaround for main command
	app.Args = cobra.ArbitraryArgs

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = common.Version

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func (c *appFlags) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "inject-drivers"
	cmd.Short = "Inject VirtIO drivers into Windows"
	cmd.Long = `Description:
  Inject VirtIO drivers into Windows

  This tool injects VirtIO drivers into a specified Windows install and
  corresponding RE wim image.
`

	cmd.RunE = c.Run

	cmd.Flags().StringVar(&c.driversSourcePath, "drivers-source-path", "", "Path to mounted root of VirtIO drivers ISO image")
	cmd.MarkFlagRequired("drivers-source-path")
	cmd.Flags().StringVar(&c.windowsArchitecture, "windows-architecture", "", "Windows architecture")
	cmd.Flags().StringVar(&c.windowsSourcePath, "windows-source-path", "", "Path to mounted C: drive")
	cmd.MarkFlagRequired("windows-source-path")
	cmd.Flags().StringVar(&c.windowsRESourcePath, "windows-re-source-path", "", "Path to mounted Windows rescue environment drive")
	cmd.MarkFlagRequired("windows-re-source-path")
	cmd.Flags().StringVar(&c.windowsVersion, "windows-version", "", "Windows version")

	return cmd
}

func (c *appFlags) Run(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()
	cacheDir := "/tmp/inject-drivers"
	err := os.MkdirAll(cacheDir, 0700)
	if err != nil {
		return err
	}

	logger, err := shared.GetLogger(false)
	if err != nil {
		return fmt.Errorf("Failed to get logger: %w\n", err)
	}

	repackUtuil := windows.NewRepackUtil(cacheDir, ctx, logger)

	reWim, err := shared.FindFirstMatch(c.windowsRESourcePath, "Recovery/WindowsRE", "winre.wim")
	if err != nil {
		return fmt.Errorf("Unable to find winre.wim: %w", err)
	}
	reWimInfo, err := repackUtuil.GetWimInfo(reWim)
	if err != nil {
		return fmt.Errorf("Failed to get RE wim info: %w", err)
	}

	if c.windowsVersion == "" {
		c.windowsVersion = windows.DetectWindowsVersion(reWimInfo.Name(1))
	}

	if c.windowsArchitecture == "" {
		c.windowsArchitecture = windows.DetectWindowsArchitecture(reWimInfo.Architecture(1))
	}

	if c.windowsVersion == "" {
		return fmt.Errorf("Failed to detect Windows version. Please provide the version using the --windows-version flag")
	}

	if c.windowsArchitecture == "" {
		return fmt.Errorf("Failed to detect Windows architecture. Please provide the architecture using the --windows-architecture flag")
	}

	repackUtuil.SetWindowsVersionArchitecture(c.windowsVersion, c.windowsArchitecture)

	// Inject drivers into the RE wim image.
	err = repackUtuil.InjectDriversIntoWim(reWim, reWimInfo, c.driversSourcePath)
	if err != nil {
		return fmt.Errorf("Failed to modify wim %q: %w", reWim, err)
	}
	logger.Info("Successfully injected drivers into RE wim image.")

	// Inject drivers into the Windows install.
	err = repackUtuil.InjectDrivers(c.windowsSourcePath, c.driversSourcePath)
	if err != nil {
		return fmt.Errorf("Failed to inject drivers: %w", err)
	}
	logger.Info("Successfully injected drivers into the Windows install.")

	return nil
}

// toHex is a pongo2 filter which converts the provided value to a hex value understood by the Windows registry.
func toHex(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
	dst := make([]byte, hex.EncodedLen(len(in.String())))
	hex.Encode(dst, []byte(in.String()))

	var builder strings.Builder

	for i := 0; i < len(dst); i += 2 {
		_, err := builder.Write(dst[i : i+2])
		if err != nil {
			return &pongo2.Value{}, &pongo2.Error{Sender: "filter:toHex", OrigError: err}
		}

		_, err = builder.WriteString(",00,")
		if err != nil {
			return &pongo2.Value{}, &pongo2.Error{Sender: "filter:toHex", OrigError: err}
		}
	}

	return pongo2.AsValue(strings.TrimSuffix(builder.String(), ",")), nil
}
