package main

import (
	"context"
	"fmt"
	"os"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/agent"
)

func main() {
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
}
