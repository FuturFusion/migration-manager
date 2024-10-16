package common

import (
	"github.com/spf13/cobra"
)

// Flags used by all applications
type CmdGlobalFlags struct {
	FlagVersion bool
	FlagHelp    bool
}

// Common flags used when connecting to a VMware endpoint
type CmdVMwareFlags struct {
	VmwareEndpoint string
	VmwareInsecure bool
	VmwareUsername string
	VmwarePassword string
}

func (c *CmdGlobalFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&c.FlagVersion, "version", false, "Print version number")
	cmd.Flags().BoolVarP(&c.FlagHelp, "help", "h", false, "Print help")
}

func (c *CmdVMwareFlags) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&c.VmwareEndpoint, "vmware-endpoint", "", "VMware endpoint URL (required)")
	cmd.MarkFlagRequired("vmware-endpoint")
	cmd.Flags().BoolVar(&c.VmwareInsecure, "vmware-insecure", false, "Ignore TLS certificate errors when connecting to VMware endpoint")
	cmd.Flags().StringVar(&c.VmwareUsername, "vmware-username", "", "VMware username (required)")
	cmd.MarkFlagRequired("vmware-username")
	cmd.Flags().StringVar(&c.VmwarePassword, "vmware-password", "", "VMware password (required)")
	cmd.MarkFlagRequired("vmware-password")
}
