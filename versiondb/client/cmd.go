package client

import (
	"github.com/spf13/cobra"
)

func ChangeSetGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeset",
		Short: "dump and manage change sets files and ingest into versiondb",
	}
	cmd.AddCommand(
		DumpChangeSetCmd(),
		PrintChangeSetCmd(),
		VerifyChangeSetCmd(),
		ConvertToSSTTSCmd(),
		ChangeSetToVersionDBCmd(),
		IngestSSTCmd(),
	)
	return cmd
}
