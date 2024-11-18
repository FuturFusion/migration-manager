package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/lxc/incus/v6/shared/logger"
	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

type cmdWorker struct {
	global *cmdGlobal

	// Common options
	flagMMEndpoint string
	flagUUID       string
}

func (c *cmdWorker) Command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = "migration-manager-worker"
	cmd.Short = "The migration manager worker"
	cmd.Long = `Description:
  The migration manager worker

  This is the migration manager worker that is run within a new Incus VM.
`
	cmd.RunE = c.Run
	cmd.Flags().StringVar(&c.flagMMEndpoint, "endpoint", "", "Controlling migration manager endpoint")
	cmd.MarkFlagRequired("endpoint")
	cmd.Flags().StringVar(&c.flagUUID, "uuid", "", "UUID of instance to migrate")
	cmd.MarkFlagRequired("uuid")

	return cmd
}

func (c *cmdWorker) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "migration-manager-worker" && args[0] != "") {
		return fmt.Errorf("unknown command \"%s\" for \"%s\"", args[0], cmd.CommandPath())
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("This tool must be run as root\n")
	}

	if !util.PathExists("/dev/virtio-ports/org.linuxcontainers.incus") {
		return fmt.Errorf("This tool is designed to be run within an Incus VM\n")
	}

	w, err := newWorker(c.flagMMEndpoint, c.flagUUID)
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, unix.SIGPWR)
	signal.Notify(sigCh, unix.SIGINT)
	signal.Notify(sigCh, unix.SIGQUIT)
	signal.Notify(sigCh, unix.SIGTERM)

	chIgnore := make(chan os.Signal, 1)
	signal.Notify(chIgnore, unix.SIGHUP)

	err = w.Start()
	if err != nil {
		return err
	}

	for {
		select {
		case sig := <-sigCh:
			logger.Info("Received signal", logger.Ctx{"signal": sig})
			if w.shutdownCtx.Err() != nil {
				logger.Warn("Ignoring signal, shutdown already in progress", logger.Ctx{"signal": sig})
			} else {
				go func() {
					w.shutdownDoneCh <- w.Stop(context.Background(), sig)
				}()
			}

		case err = <-w.shutdownDoneCh:
			return err
		}
	}
}
