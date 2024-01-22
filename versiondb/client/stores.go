package client

import (
	"fmt"
	"strings"

	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
	"github.com/spf13/cobra"
)

func ListDefaultStoresCmd(stores []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default-stores",
		Short: "List the store names in current binary version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range stores {
				fmt.Println(name)
			}

			return nil
		},
	}
	return cmd
}

func GetStoresOrDefault(cmd *cobra.Command, defaultStores []string) ([]string, error) {
	stores, err := cmd.Flags().GetString(flagStores)
	if err != nil {
		return nil, err
	}
	if len(stores) == 0 {
		return defaultStores, nil
	}
	return strings.Split(stores, " "), nil
}

func GetLatestVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "latest-version [versiondb-path]",
		Short: "Get latest version in current versiondb version",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := args[0]
			db, cfHandle, err := tsrocksdb.OpenVersionDB(dbPath)
			if err != nil {
				return err
			}
			store := tsrocksdb.NewStoreWithDB(db, cfHandle)
			version, err := store.GetLatestVersion()
			if err != nil {
				return err
			}
			cmd.Println(version)
			return nil
		},
	}
	return cmd
}
