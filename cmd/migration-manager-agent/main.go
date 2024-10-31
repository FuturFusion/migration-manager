package main

import (
	"fmt"
	"os"

	"github.com/FuturFusion/migration-manager/internal"
	"github.com/FuturFusion/migration-manager/internal/agent"
)

func main() {
	fmt.Printf("This is migration-manager-agent v%s\n", internal.Version)

	// TODO -- Fetch this file from the config drive that's mounted into the VM
	agentConfig, err := agent.AgentConfigFromYamlFile("./agent.yaml")
	if err != nil {
		fmt.Printf("Failed to open config file: %s\n", err)
		os.Exit(1)
	}
}
