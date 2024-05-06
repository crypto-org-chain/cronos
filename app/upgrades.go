package app

import (
	"fmt"
	"math/big"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	clientkeeper "github.com/cosmos/ibc-go/v7/modules/core/02-client/keeper"
	"github.com/ethereum/go-ethereum/common"

	e2eetypes "github.com/crypto-org-chain/cronos/v2/x/e2ee/types"
)

type contractMigration struct {
	Contract common.Address
	Slot     common.Hash
	Value    common.Hash
}

// ContractMigrations records the list of contract migrations, chain-id -> migrations
var ContractMigrations = map[string][]contractMigration{
	"cronostestnet_338-3": {
		{
			Contract: common.HexToAddress("0x6265bf2371ccf45767184c8bd77b5c52e752c2bb"),
			Slot:     common.BigToHash(big.NewInt(0)),
			Value:    common.HexToHash("0x000000000000000000000000730CbB94480d50788481373B43d83133e171367e"),
		},
	},
}

func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, clientKeeper clientkeeper.Keeper) {
	planName := "v1.3"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		m, err := app.mm.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return m, err
		}

		// migrate contract states
		for _, migration := range ContractMigrations[ctx.ChainID()] {
			app.EvmKeeper.SetState(ctx, migration.Contract, migration.Slot, migration.Value.Bytes())
		}

		return m, nil
	})

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}
	if !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		if upgradeInfo.Name == planName {
			app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storetypes.StoreUpgrades{
				Added: []string{
					e2eetypes.StoreKey,
				},
			}))
		}
	}
}
