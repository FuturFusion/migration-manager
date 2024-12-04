package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"

	"github.com/lxc/incus/v6/shared/logger"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

type cmdDaemon struct {
	global *cmdGlobal

	// Common options
	flagDatabaseDir string
	flagServerIP    string
	flagServerPort  int
	flagTLSCert     string
	flagTLSKey      string
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
	cmd.Flags().StringVar(&c.flagDatabaseDir, "database-dir", "./", "Directory to store sqlite database in")
	cmd.Flags().StringVar(&c.flagServerIP, "server-ip", "0.0.0.0", "IP address to bind to")
	cmd.Flags().IntVar(&c.flagServerPort, "server-port", 8443, "IP port to bind to")
	cmd.Flags().StringVar(&c.flagTLSCert, "tls-cert", "", "TLS certificate file to be used by server")
	cmd.Flags().StringVar(&c.flagTLSKey, "tls-key", "", "TLS key file to be used by server")
	cmd.MarkFlagsRequiredTogether("tls-cert", "tls-key")

	return cmd
}

func (c *cmdDaemon) Run(cmd *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "migration-managerd" && args[0] != "") {
		return fmt.Errorf("unknown command \"%s\" for \"%s\"", args[0], cmd.CommandPath())
	}

	config := &DaemonConfig{
		dbPathDir:           c.flagDatabaseDir,
		restServerIPAddr:    c.flagServerIP,
		restServerPort:      c.flagServerPort,
		restServerTLSConfig: nil,
	}
	if c.flagTLSCert != "" {
		cert, err := tls.LoadX509KeyPair(c.flagTLSCert, c.flagTLSKey)
		if err != nil {
			logger.Errorf("Failed to load TLS cert/key: %s", err)
			return err
		}
		config.restServerTLSConfig = &tls.Config{}
		config.restServerTLSConfig.Certificates = append(config.restServerTLSConfig.Certificates, cert)
	}
	d := newDaemon(config)

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
			if d.shutdownCtx.Err() != nil {
				logger.Warn("Ignoring signal, shutdown already in progress", logger.Ctx{"signal": sig})
			} else {
				go func() {
					d.shutdownDoneCh <- d.Stop(context.Background(), sig)
				}()
			}

		case err = <-d.shutdownDoneCh:
			return err
		}
	}
}
