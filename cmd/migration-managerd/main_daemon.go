package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/lxc/incus/v6/shared/logger"
)

type cmdDaemon struct {
	global *cmdGlobal

	// Common options
	flagTLSCert string
	flagTLSKey string
}

func (c *cmdDaemon) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "migration-managerd"
	cmd.Short = "The migration manager daemon"
	cmd.Long = `Description:
  The migration manager daemon

  This is the migration manager daemon command line.
`
	cmd.RunE = c.Run
	cmd.Flags().StringVar(&c.flagTLSCert, "tls-cert", "", "TLS certificate file to be used by server")
	cmd.Flags().StringVar(&c.flagTLSKey, "tls-key", "", "TLS key file to be used by server")
	cmd.MarkFlagsRequiredTogether("tls-cert", "tls-key")

	return cmd
}

func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "daemon" && args[0] != "") {
		return fmt.Errorf("unknown command \"%s\" for \"%s\"", args[0], cmd.CommandPath())
	}

	defer logger.Info("Daemon stopped")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, unix.SIGPWR)
	signal.Notify(sigCh, unix.SIGINT)
	signal.Notify(sigCh, unix.SIGQUIT)
	signal.Notify(sigCh, unix.SIGTERM)

	chIgnore := make(chan os.Signal, 1)
	signal.Notify(chIgnore, unix.SIGHUP)

	for {
		select {
		case sig := <-sigCh:
			logger.Info("Received signal", logger.Ctx{"signal": sig})
			return nil
		}
	}
}
