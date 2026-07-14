package app

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	memiavlstore "github.com/crypto-org-chain/cronos-store/store"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// numOpenFDs returns the number of file descriptors held open by this process.
func numOpenFDs(t *testing.T) int {
	t.Helper()
	p, err := process.NewProcess(int32(os.Getpid()))
	require.NoError(t, err)
	n, err := p.NumFDs()
	if err != nil {
		t.Skipf("fd counting unsupported on this platform: %v", err)
	}
	return int(n)
}

// setupMemIAVLAppWithHistory builds an app backed by the on-disk memiavl store,
// commits `blocks` empty blocks, and returns the app plus a funded account.
func setupMemIAVLAppWithHistory(t *testing.T, blocks int64) (*App, sdk.AccAddress) {
	t.Helper()

	appOpts := MinimalOptionsMap{
		flags.FlagHome:           t.TempDir(),
		flags.FlagChainID:        TestAppChainID,
		memiavlstore.FlagMemIAVL: true,
	}
	app := New(log.NewNopLogger(), nil, true, appOpts, baseapp.SetChainID(TestAppChainID))
	t.Cleanup(func() { _ = app.Close() })

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	consAddress := sdk.ConsAddress(pubKey.Address())
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pubKey, 1)})

	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	acctAddr := sdk.AccAddress(priv.PubKey().Address())
	baseAcct := authtypes.NewBaseAccount(acctAddr, priv.PubKey(), 0, 0)
	balances := []banktypes.Balance{{
		Address: acctAddr.String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntWithDecimal(1000, 18))),
	}}

	genesisState, err := simtestutil.GenesisStateWithValSet(
		app.AppCodec(), app.DefaultGenesis(), valSet,
		[]authtypes.GenesisAccount{baseAcct}, balances...,
	)
	require.NoError(t, err)
	appState, err := json.Marshal(genesisState)
	require.NoError(t, err)

	consensusParams := *DefaultConsensusParams
	consensusParams.Block = &cmtproto.BlockParams{MaxBytes: math.MaxInt64, MaxGas: math.MaxInt64}
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         TestAppChainID,
		AppStateBytes:   appState,
		ConsensusParams: &consensusParams,
	})
	require.NoError(t, err)

	for h := int64(1); h <= blocks; h++ {
		_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: h, ProposerAddress: consAddress})
		require.NoError(t, err)
		_, err = app.Commit()
		require.NoError(t, err)
	}
	return app, acctAddr
}

// TestHistoricalQueryClosesMemIAVLDB checks that historical queries don't leak fds.
// A query below the latest height opens a read-only memiavl DB; baseapp must close
// it (fork #1816), otherwise every such query leaks an fd.
func TestHistoricalQueryClosesMemIAVLDB(t *testing.T) {
	app, acctAddr := setupMemIAVLAppWithHistory(t, 10)

	// height strictly below latest -> read-only memiavl.Load path
	const histHeight = int64(3)
	reqData, err := (&banktypes.QueryBalanceRequest{
		Address: acctAddr.String(),
		Denom:   evmtypes.DefaultEVMDenom,
	}).Marshal()
	require.NoError(t, err)

	query := func() {
		resp, err := app.Query(context.Background(), &abci.RequestQuery{
			Path:   "/cosmos.bank.v1beta1.Query/Balance",
			Data:   reqData,
			Height: histHeight,
		})
		require.NoError(t, err)
		require.Equalf(t, uint32(0), resp.Code, "query failed: %s", resp.Log)
	}

	query() // warm up one-time allocations

	before := numOpenFDs(t)
	const iters = 25
	for i := 0; i < iters; i++ {
		query()
	}
	after := numOpenFDs(t)
	t.Logf("fd delta over %d historical queries: %d (before=%d after=%d)", iters, after-before, before, after)

	require.LessOrEqualf(t, after-before, 2,
		"historical queries leaked file descriptors: read-only memiavl DB not closed")
}

// TestCacheMultiStoreWithVersionLeaksWhenNotClosed confirms the leak is real:
// opening historical read-only stores without closing them grows the fd count.
func TestCacheMultiStoreWithVersionLeaksWhenNotClosed(t *testing.T) {
	app, _ := setupMemIAVLAppWithHistory(t, 10)
	cms := app.CommitMultiStore()

	const iters = 5
	var closers []io.Closer
	t.Cleanup(func() {
		for _, c := range closers {
			_ = c.Close()
		}
	})

	before := numOpenFDs(t)
	for i := 0; i < iters; i++ {
		s, err := cms.CacheMultiStoreWithVersion(3)
		require.NoError(t, err)
		c, ok := s.(io.Closer)
		require.True(t, ok, "historical CacheMultiStoreWithVersion must return an io.Closer")
		closers = append(closers, c)
	}
	after := numOpenFDs(t)
	t.Logf("fd delta over %d unclosed loads: %d (before=%d after=%d)", iters, after-before, before, after)

	require.GreaterOrEqualf(t, after-before, iters,
		"expected unclosed read-only loads to accumulate fds")
}
