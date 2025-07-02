package keeper

import (
	v2 "github.com/crypto-org-chain/cronos/v2/x/cronos/migrations/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper         Keeper
	legacySubspace paramstypes.Subspace
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper, ss paramstypes.Subspace) Migrator {
	return Migrator{keeper: keeper, legacySubspace: ss}
}

// Migrate1to2 migrates from version 1 to 2.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	err := v2.Migrate(ctx, ctx.KVStore(m.keeper.storeKey), m.legacySubspace, m.keeper.cdc)
	return err
}
