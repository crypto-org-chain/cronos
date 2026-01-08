package attestation

import (
	"github.com/spf13/cobra"
)

const (
	// FlagDAIBCVersion is the flag to specify the IBC version for data availability
	FlagDAIBCVersion = "da-ibc-version"

	// DefaultDAIBCVersion is the default IBC version for data availability attestation
	DefaultDAIBCVersion = "v2"
)

// AddModuleInitFlags implements servertypes.ModuleInitFlags interface.
func AddModuleInitFlags(startCmd *cobra.Command) {
	startCmd.Flags().String(
		FlagDAIBCVersion,
		DefaultDAIBCVersion,
		"IBC version for data availability attestation (v1 or v2)",
	)
}
