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
	// REVIEW: if I understand the workflow correctly, the `migration-manager-worker`
	// is executed within every target machine, we migrate to. So I expect this
	// also to include Windows machines. Correct?
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
	// REVIEW: why is `args[0] != ""` part of the condition below?
	// REVIEW: wouldn't this condition be enough: !(len(args) == 1 && args[0] == "migration-manager-worker")
	if len(args) > 1 || (len(args) == 1 && args[0] != "migration-manager-worker" && args[0] != "") {
		return fmt.Errorf("unknown command \"%s\" for \"%s\"", args[0], cmd.CommandPath())
	}

	// REVIEW: on Windows, this returns -1
	if os.Geteuid() != 0 {
		return fmt.Errorf("This tool must be run as root\n")
	}

	// REVIEW: does this check also work in Windows?
	if !util.PathExists("/dev/virtio-ports/org.linuxcontainers.incus") {
		return fmt.Errorf("This tool is designed to be run within an Incus VM\n")
	}

	w, err := newWorker(c.flagMMEndpoint, c.flagUUID)
	if err != nil {
		return err
	}

	// REVIEW: We can not use these signals on Windows, since those are not available on Windows.
	// REVIEW: From the doc of signal.Notify:
	// > Package signal will not block sending to c: the caller must ensure that c has sufficient buffer space to keep up with the expected signal rate. For a channel used for notification of just one signal value, a buffer of size 1 is sufficient.
	// Therefore we need to make the signal channel at least of size 4.
	// REVIEW: Maybe it would be simpler to use: signal.NotifyContext(parent context.Context, signals ...os.Signal)
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
					// REVIEW: based on the comment for `shutdownDoneCh`, I would send
					// to this channel also in `w.Stop` and not pass this responsability
					// to the caller, where it can be easily forgotten and the promise
					// stated in the comment becomes invalid.
					w.shutdownDoneCh <- w.Stop(context.Background(), sig)
				}()
			}

		case err = <-w.shutdownDoneCh:
			return err

			// REVIEW: should we wait for cmd.Context().Done() as well?
		}
	}
}
