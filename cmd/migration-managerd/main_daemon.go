package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	daemon "github.com/FuturFusion/migration-manager/cmd/migration-managerd/internal/api"
)

type cmdDaemon struct {
	global *cmdGlobal
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

	return cmd
}

func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "migration-managerd" && args[0] != "") {
		return fmt.Errorf(`unknown command %q for %q`, args[0], cmd.CommandPath())
	}

	d := daemon.NewDaemon(c.global.logHandler)

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
			slog.Info("Received signal", slog.Any("signal", sig))
			if d.ShutdownCtx.Err() != nil {
				slog.Warn("Ignoring signal, shutdown already in progress", slog.Any("signal", sig))
			} else {
				go func() {
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer shutdownCancel()
					d.ShutdownDoneCh <- d.Stop(shutdownCtx)
				}()
			}

		case err = <-d.ShutdownDoneCh:
			return err
		}
	}
}
