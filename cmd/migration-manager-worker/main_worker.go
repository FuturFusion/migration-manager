package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/lxc/incus/v6/shared/util"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/FuturFusion/migration-manager/cmd/migration-manager-worker/internal/worker"
	internalUtil "github.com/FuturFusion/migration-manager/internal/util"
)

type cmdWorker struct {
	global *cmdGlobal
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

	return cmd
}

func (c *cmdWorker) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "migration-manager-worker" && args[0] != "") {
		return fmt.Errorf("unknown command \"%s\" for \"%s\"", args[0], cmd.CommandPath())
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("This tool must be run as root")
	}

	if !util.PathExists("/dev/virtio-ports/org.linuxcontainers.incus") {
		return fmt.Errorf("This tool is designed to be run within an Incus VM")
	}

	if !util.PathExists("/dev/incus/sock") {
		return fmt.Errorf("Unable to find Incus socket")
	}

	client := internalUtil.UnixHTTPClient("/dev/incus/sock")
	w, err := worker.NewWorker(cmd.Context(), client)
	if err != nil {
		return err
	}

	chIgnore := make(chan os.Signal, 1)
	signal.Notify(chIgnore, unix.SIGHUP)

	rootCtx, stop := signal.NotifyContext(context.Background(),
		unix.SIGPWR,
		unix.SIGINT,
		unix.SIGQUIT,
		unix.SIGTERM,
	)
	defer stop()

	ctx, shutdown := context.WithCancel(rootCtx)
	defer shutdown()

	shutdownCompleted := make(chan struct{})
	go func() {
		defer close(shutdownCompleted)
		w.Run(ctx)
	}()

	for {
		select {
		case <-rootCtx.Done():
			slog.Info("Shutting down")
			if ctx.Err() != nil {
				slog.Warn("Ignoring signal, shutdown already in progress")
				continue
			}

			shutdown()

		case <-shutdownCompleted:
			return nil
		}
	}
}
