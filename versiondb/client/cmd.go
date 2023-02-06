package client

import (
	"github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cobra"
)

func ChangeSetGroupCmd(appCreator types.AppCreator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeset",
		Short: "dump and manage change sets files and ingest into versiondb",
	}
	cmd.AddCommand(
		ListStoresCmd(appCreator),
		DumpChangeSetCmd(appCreator),
		PrintChangeSetCmd(),
		VerifyChangeSetCmd(appCreator),
		BuildVersionDBSSTCmd(appCreator),
		IngestVersionDBSSTCmd(),
		ChangeSetToVersionDBCmd(),
		RestoreAppDBCmd(appCreator),
	)
	return cmd
}
