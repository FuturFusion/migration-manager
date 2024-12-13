package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/lxc/incus/v6/shared/logger"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/FuturFusion/migration-manager/cmd/migration-managerd/api"
	"github.com/FuturFusion/migration-manager/internal/ports"
)

type cmdDaemon struct {
	global *cmdGlobal

	// Common options
	flagGroup      string
	flagServerIP   string
	flagServerPort int
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
	cmd.Flags().StringVar(&c.flagGroup, "group", "", "The group of users that will be allowed to talk to the migration manager")
	cmd.Flags().StringVar(&c.flagServerIP, "server-ip", "0.0.0.0", "IP address to bind to")
	cmd.Flags().IntVar(&c.flagServerPort, "server-port", ports.HTTPSDefaultPort, "IP port to bind to")

	return cmd
}

func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "migration-managerd" && args[0] != "") {
		return fmt.Errorf("unknown command \"%s\" for \"%s\"", args[0], cmd.CommandPath())
	}

	config := &api.DaemonConfig{
		Group:            c.flagGroup,
		RestServerIPAddr: c.flagServerIP,
		RestServerPort:   c.flagServerPort,
	}

	d := api.NewDaemon(config)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, unix.SIGPWR)
	signal.Notify(sigCh, unix.SIGINT)
	signal.Notify(sigCh, unix.SIGQUIT)
	signal.Notify(sigCh, unix.SIGTERM)

	chIgnore := make(chan os.Signal, 1)
	signal.Notify(chIgnore, unix.SIGHUP)

	err := d.Start()
	if err != nil {
		return err
	}

	for {
		select {
		case sig := <-sigCh:
			logger.Info("Received signal", logger.Ctx{"signal": sig})
			if d.ShutdownCtx.Err() != nil {
				logger.Warn("Ignoring signal, shutdown already in progress", logger.Ctx{"signal": sig})
			} else {
				go func() {
					d.ShutdownDoneCh <- d.Stop(context.Background(), sig)
				}()
			}

		case err = <-d.ShutdownDoneCh:
			return err
		}
	}
}
