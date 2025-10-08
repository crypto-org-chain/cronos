package cmd

import (
	"bytes"
	"fmt"
	"sort"

	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	"github.com/crypto-org-chain/cronos/v2/app"
	cronostypes "github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	e2eetypes "github.com/crypto-org-chain/cronos/v2/x/e2ee/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	feemarkettypes "github.com/evmos/ethermint/x/feemarket/types"

	dbm "github.com/cosmos/cosmos-db"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	"cosmossdk.io/store/rootmulti"
	"cosmossdk.io/store/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func DumpRootCmd() *cobra.Command {
	keys, _, _, _ := app.StoreKeys()
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)
	return DumpRootGroupCmd(storeNames)
}

func DumpRootGroupCmd(storeNames []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-root",
		Short: "dump module root",
	}
	cmd.AddCommand(
		DumpMemIavlRoot(storeNames),
		DumpIavlRoot(storeNames),
	)
	return cmd
}

func DumpMemIavlRoot(storeNames []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-memiavl-root [dir]",
		Short: "dump mem-iavl root at version [dir]",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			version, err := cmd.Flags().GetUint32("version")
			if err != nil {
				return err
			}
			opts := memiavl.Options{
				InitialStores:   storeNames,
				CreateIfMissing: false,
				TargetVersion:   version,
				ReadOnly:        true,
			}
			db, err := memiavl.Load(dir, opts)
			if err != nil {
				return err
			}
			defer db.Close()
			sort.Strings(storeNames)
			for _, storeName := range storeNames {
				tree := db.TreeByName(storeName)
				if tree != nil {
					fmt.Printf("module %s version %d RootHash %X\n", storeName, tree.Version(), tree.RootHash())
				} else {
					fmt.Printf("module %s not loaded\n", storeName)
				}
			}

			db.MultiTree.UpdateCommitInfo()
			lastCommitInfo := convertCommitInfo(db.MultiTree.LastCommitInfo())

			fmt.Printf("Version %d RootHash %X\n", lastCommitInfo.Version, lastCommitInfo.Hash())
			return nil
		},
	}
	cmd.Flags().Uint32("version", 0, "the version to dump")
	return cmd
}

func convertCommitInfo(commitInfo *memiavl.CommitInfo) *types.CommitInfo {
	storeInfos := make([]types.StoreInfo, len(commitInfo.StoreInfos))
	for i, storeInfo := range commitInfo.StoreInfos {
		storeInfos[i] = types.StoreInfo{
			Name: storeInfo.Name,
			CommitId: types.CommitID{
				Version: storeInfo.CommitId.Version,
				Hash:    storeInfo.CommitId.Hash,
			},
		}
	}
	return &types.CommitInfo{
		Version:    commitInfo.Version,
		StoreInfos: storeInfos,
	}
}

func DumpIavlRoot(storeNames []string) *cobra.Command {
	// this needs to change in different height
	// because some StoreKey version are zero
	// such as consensusparamtypes, circuittypes
	storeNames = []string{
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, capabilitytypes.StoreKey, consensusparamtypes.StoreKey,
		feegrant.StoreKey, crisistypes.StoreKey,
		// ibc keys
		ibcexported.StoreKey, ibctransfertypes.StoreKey,
		ibcfeetypes.StoreKey,
		// ica keys
		icacontrollertypes.StoreKey,
		icahosttypes.StoreKey,
		// ethermint keys
		evmtypes.StoreKey, feemarkettypes.StoreKey,
		// e2ee keys
		e2eetypes.StoreKey,
		// this line is used by starport scaffolding # stargate/app/storeKey
		cronostypes.StoreKey,
	}

	cmd := &cobra.Command{
		Use:   "dump-iavl-root [dir]",
		Short: "dump iavl root at version [dir]",
		Long:  "dump iavl root at version [dir]. To support dumping rocksdb, it should use this https://github.com/cosmos/cosmos-db/blob/9221ee7e2bccf314eff49f89092dd0767588d76e/rocksdb.go#L51.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			version, err := cmd.Flags().GetInt64("version")
			if err != nil {
				return err
			}
			db, err := dbm.NewGoLevelDB("application", dir, dbm.OptionsMap{"read_only": true})
			if err != nil {
				return err
			}
			defer db.Close()
			rs := rootmulti.NewStore(db, log.NewNopLogger(), nil)
			for _, storeKey := range storeNames {
				rs.MountStoreWithDB(types.NewKVStoreKey(storeKey), types.StoreTypeIAVL, nil)
			}

			err = rs.LoadVersion(version)
			if err != nil {
				fmt.Printf("failed to load  version %d %s\n", version, err.Error())
				return err
			}

			var cInfo *types.CommitInfo
			cInfo, err = rs.GetCommitInfo(version)
			if err != nil {
				fmt.Printf("failed to load version %d commit info: %s\n", version, err.Error())
				return err
			}
			infoMaps := make(map[string]types.StoreInfo)
			for _, storeInfo := range cInfo.StoreInfos {
				infoMaps[storeInfo.Name] = storeInfo
			}

			var infos []types.StoreInfo
			sort.Strings(storeNames)
			for _, storeName := range storeNames {
				info, ok := infoMaps[storeName]
				if !ok {
					fmt.Printf("module %s not loaded\n", storeName)
					continue
				}
				commitID := info.CommitId
				fmt.Printf("module %s version %d RootHash %X\n", storeName, commitID.Version, commitID.Hash)
				infos = append(infos, info)
			}

			if len(infos) != len(cInfo.StoreInfos) {
				fmt.Printf("Warning: Partial commit info (loaded %d stores, found %d)\n", len(cInfo.StoreInfos), len(infos))
				storeMaps := make(map[string]struct{})
				for _, storeName := range storeNames {
					storeMaps[storeName] = struct{}{}
				}
				for _, info := range cInfo.StoreInfos {
					if _, ok := storeMaps[info.Name]; !ok {
						fmt.Printf("module %s missed\n", info.Name)
					}
				}
			}

			commitInfo := &types.CommitInfo{
				Version:    version,
				StoreInfos: infos,
			}

			if rs.LastCommitID().Version != commitInfo.Version || !bytes.Equal(rs.LastCommitID().Hash, commitInfo.Hash()) {
				return fmt.Errorf("failed to calculate %d commit info, rs Hash %X, commit Hash %X", rs.LastCommitID().Version, rs.LastCommitID().Hash, commitInfo.Hash())
			}
			fmt.Printf("Version %d RootHash %X\n", commitInfo.Version, commitInfo.Hash())
			return nil
		},
	}
	cmd.Flags().Int64("version", 0, "the version to dump")
	return cmd
}
