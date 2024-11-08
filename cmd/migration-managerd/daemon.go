package main

import (
	"context"
	"crypto/tls"
	"os"

	"github.com/lxc/incus/v6/shared/logger"

	"github.com/FuturFusion/migration-manager/internal/db"
)

type DaemonConfig struct {
	dbPathDir string

	restServerIPAddr  string
	restServerPort    int
	restServerTLSCert tls.Certificate
}

type Daemon struct {
	config         *DaemonConfig

	db             *db.Node

	shutdownCtx    context.Context    // Canceled when shutdown starts.
	shutdownCancel context.CancelFunc // Cancels the shutdownCtx to indicate shutdown starting.
	shutdownDoneCh chan error         // Receives the result of the d.Stop() function and tells the daemon to end.
}

func newDaemon(config *DaemonConfig) *Daemon {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	return &Daemon{
		config: config,
		db: &db.Node{},
		shutdownCtx: shutdownCtx,
		shutdownCancel: shutdownCancel,
		shutdownDoneCh: make(chan error),
	}
}

func (d *Daemon) Start() error {
	var err error

	// Open the local sqlite database.
	d.db, err = db.OpenDatabase(d.config.dbPathDir)
	if err != nil {
		logger.Errorf("Failed to open sqlite database: %s", err)
		return err
	}

	logger.Info("Daemon started")

	return nil
}

func (d *Daemon) Stop(ctx context.Context, sig os.Signal) error {
	logger.Info("Daemon stopped")

	return nil
}
