package client

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/crypto-org-chain/cronos/store/rootmulti"
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

func getDataDir(cmd *cobra.Command) string {
	ctx := server.GetServerContextFromCmd(cmd)
	home := ctx.Viper.GetString(flags.FlagHome)
	return filepath.Join(home, "data")
}

func getStore(cmd *cobra.Command) (*tsrocksdb.Store, error) {
	dir := getDataDir(cmd)
	db, cfHandle, err := tsrocksdb.OpenVersionDB(filepath.Join(dir, "versiondb"))
	if err != nil {
		return nil, err
	}
	store := tsrocksdb.NewStoreWithDB(db, cfHandle)
	return &store, nil
}

func GetLatestVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-latest-version",
		Short: "Get latest version of current versiondb",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore(cmd)
			if err != nil {
				return err
			}
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

func SetVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-version",
		Short: "Set target version of current versiondb",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := getStore(cmd)
			if err != nil {
				return err
			}
			targetVersion, err := cmd.Flags().GetInt64(flagTargetVersion)
			if err != nil {
				return err
			}
			if targetVersion <= 0 {
				dir := getDataDir(cmd)
				cms := rootmulti.NewStore(filepath.Join(dir, "memiavl.db"), nil, false, false)
				targetVersion = cms.LastCommitID().Version
			}
			cmd.Println(targetVersion)
			return store.SetLatestVersion(targetVersion)
		},
	}
	cmd.Flags().Int64(flagTargetVersion, 0, "specify the target version, default to latest iavl version")
	return cmd
}
