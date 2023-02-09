package client

import (
	"sort"

	"github.com/crypto-org-chain/cronos/v2/app"
	"github.com/spf13/cobra"
)

func ChangeSetGroupCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys()
	stores := make([]string, 0, len(keys))
	for name := range keys {
		stores = append(stores, name)
	}
	sort.Strings(stores)

	cmd := &cobra.Command{
		Use:   "changeset",
		Short: "dump and manage change sets files and ingest into versiondb",
	}
	cmd.AddCommand(
		ListStoresCmd(stores),
		DumpChangeSetCmd(stores),
		PrintChangeSetCmd(),
		VerifyChangeSetCmd(stores),
		BuildVersionDBSSTCmd(stores),
		IngestVersionDBSSTCmd(),
		ChangeSetToVersionDBCmd(),
		RestoreAppDBCmd(stores),
	)
	return cmd
}
