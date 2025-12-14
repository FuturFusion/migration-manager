package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/FuturFusion/migration-manager/internal/logger"
	"github.com/FuturFusion/migration-manager/internal/server/sys"
	"github.com/FuturFusion/migration-manager/internal/version"
)

type cmdGlobal struct {
	cmd *cobra.Command

	flagHelp    bool
	flagVersion bool

	flagLogFile    string
	flagLogDebug   bool
	flagLogVerbose bool

	logHandler *logger.Handler
}

func (c *cmdGlobal) Run(cmd *cobra.Command, args []string) error {
	var err error
	c.logHandler, err = logger.InitLogger(c.flagLogFile, c.flagLogVerbose, c.flagLogDebug)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	// Make sure expected directories exist and create them if missing.
	sysInfo := sys.DefaultOS()
	err := sysInfo.Init()
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}

	defaultLogFile := filepath.Join(sysInfo.LogDir, "migration-manager.log")

	// daemon command (main)
	daemonCmd := cmdDaemon{}
	app := daemonCmd.Command()
	app.SilenceUsage = true
	app.CompletionOptions = cobra.CompletionOptions{DisableDefaultCmd: true}

	// Workaround for main command
	app.Args = cobra.ArbitraryArgs

	// Global flags
	globalCmd := cmdGlobal{cmd: app}
	daemonCmd.global = &globalCmd
	app.PersistentPreRunE = globalCmd.Run
	app.PersistentFlags().BoolVar(&globalCmd.flagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVarP(&globalCmd.flagHelp, "help", "h", false, "Print help")
	app.PersistentFlags().StringVar(&globalCmd.flagLogFile, "logfile", defaultLogFile, "Path to the log file")
	app.PersistentFlags().BoolVarP(&globalCmd.flagLogDebug, "debug", "d", false, "Show all debug messages")
	app.PersistentFlags().BoolVarP(&globalCmd.flagLogVerbose, "verbose", "v", false, "Show all information messages")

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = version.Version

	// Run the main command and handle errors
	err = app.Execute()
	if err != nil {
		app.Printf("%s\n", err)
		os.Exit(1)
	}
}
