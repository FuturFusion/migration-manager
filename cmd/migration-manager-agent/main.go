package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lxc/incus/v6/shared/util"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/agent"
	"github.com/FuturFusion/migration-manager/internal/source"
)

func main() {
	if os.Geteuid() != 0 {
		fmt.Printf("This tool must be run as root\n")
		os.Exit(1)
	}

	if !util.PathExists("/dev/virtio-ports/org.linuxcontainers.incus") {
		fmt.Printf("This tool is designed to be run within an Incus VM\n")
		os.Exit(1)
	}

	ctx := context.TODO()

	fmt.Printf("This is migration-manager-agent v%s\n", internal.Version)

	// TODO -- Fetch this file from the config drive that's mounted into the VM
	agentConfig, err := agent.AgentConfigFromYamlFile("./agent.yaml")
	if err != nil {
		fmt.Printf("Failed to open config file: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config loaded, connecting to migration manager at %s\n", agentConfig.MigrationManagerEndpoint)
	// TODO -- Actually reach out to migration manager and get instructions

	fmt.Printf("Connecting to source %s\n", agentConfig.Source.GetName())
	err = agentConfig.Source.Connect(ctx)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}

	// TODO -- This will eventually be in some sort of callback that triggers a disk import.
	err = importDisks(ctx, agentConfig.Source, agentConfig.VMName)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}

	if agentConfig.VMOperatingSystemName == "Windows" {
		err := agent.WindowsInjectDrivers(ctx, "w11", "/dev/sda3", "/dev/sda4") // TODO -- values are hardcoded
		if err != nil {
			fmt.Printf("ERROR: %s\n", err)
			os.Exit(1)
		}
	} else if agentConfig.VMOperatingSystemName == "Debian" {
		err := agent.LinuxDoPostMigrationConfig("Debian", "/dev/sda1") // TODO -- value is hardcoded
		if err != nil {
			fmt.Printf("ERROR: %s\n", err)
			os.Exit(1)
		}
	}
}

func importDisks(ctx context.Context, source source.Source, vmName string) error {
	err := source.DeleteVMSnapshot(ctx, vmName, internal.IncusSnapshotName)
	if err != nil {
		return err
	}

	return source.ImportDisks(ctx, vmName)
}
