package app

import (
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/store/v2/dbadapter"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPruneStaleIBCConsensusStateSubkeys(t *testing.T) {
	db := dbm.NewMemDB()
	store := dbadapter.Store{DB: db}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())

	const (
		clientA = "07-tendermint-0"
		clientB = "07-tendermint-1"
	)

	// canonical keys — must survive
	canonical := []string{
		fmt.Sprintf("clients/%s/%s", clientA, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s", clientB, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s/0-100", clientA, host.KeyConsensusStatePrefix),
		fmt.Sprintf("clients/%s/%s/1-200", clientB, host.KeyConsensusStatePrefix),
	}
	for _, k := range canonical {
		store.Set([]byte(k), []byte("canonical-value"))
	}

	// stale old-format keys — must be deleted
	stale := []string{
		fmt.Sprintf("clients/%s/%s/0/50/%s", clientB, host.KeyConsensusStatePrefix, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s/1/200/%s", clientB, host.KeyConsensusStatePrefix, host.KeyClientState),
		fmt.Sprintf("clients/%s/%s/0/100/%s", clientA, host.KeyConsensusStatePrefix, host.KeyClientState),
	}
	for _, k := range stale {
		store.Set([]byte(k), []byte("consensus-bytes-stored-under-clientState-key"))
	}

	require.NoError(t, pruneStaleIBCConsensusStateSubkeys(ctx, store))

	for _, k := range stale {
		require.Nil(t, store.Get([]byte(k)), "stale key %s must be deleted", k)
	}
	for _, k := range canonical {
		require.NotNil(t, store.Get([]byte(k)), "canonical key %s must survive", k)
	}

	// idempotent: second run must not error
	require.NoError(t, pruneStaleIBCConsensusStateSubkeys(ctx, store))
	for _, k := range canonical {
		require.NotNil(t, store.Get([]byte(k)), "canonical key %s must survive second run", k)
	}
}

// TestUpgradeV18CroBridgeContractAddresses verifies that:
//  1. croBridgeContractAddresses are all valid EVM addresses.
//  2. The upgrade handler param migration correctly persists the values via SetParams/GetParams.
func TestUpgradeV18CroBridgeContractAddresses(t *testing.T) {
	for _, addr := range croBridgeContractAddresses {
		require.True(t, common.IsHexAddress(addr),
			"croBridgeContractAddresses entry must be a valid EVM hex address, got: %s",
			addr)
		require.NotEqual(t, common.Address{}, common.HexToAddress(addr),
			"croBridgeContractAddresses entry must not be the zero address, got: %s",
			addr)
	}

	// Verify the migration code path: GetParams → mutate → SetParams → GetParams round-trips correctly.
	a := Setup(t, "")
	ctx := a.NewContext(false)

	// Apply the same mutation the upgrade handler performs.
	params := a.CronosKeeper.GetParams(ctx)
	params.CroBridgeContractAddresses = croBridgeContractAddresses
	require.NoError(t, a.CronosKeeper.SetParams(ctx, params))

	stored := a.CronosKeeper.GetParams(ctx)
	require.ElementsMatch(t, croBridgeContractAddresses, stored.CroBridgeContractAddresses,
		"CroBridgeContractAddresses not persisted by SetParams")
}
