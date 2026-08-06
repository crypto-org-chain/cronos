package app

import (
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
)

// TestNewApp_AnteCacheBoundedWhenMempoolMaxTxsZero proves the AnteCache's size is
// decoupled from --mempool.max-txs=0 (the documented "unbounded PriorityMempool"
// setting): AnteCache itself treats maxTx==0 as unbounded, so without the app-side
// substitution a funded sender could grow it forever. App.New must substitute a
// bounded default instead.
func TestNewApp_AnteCacheBoundedWhenMempoolMaxTxsZero(t *testing.T) {
	opts := baseTestAppOpts(0)
	opts[server.FlagMempoolMaxTxs] = 0

	a := New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	ac := a.AnteCache()
	require.NotNil(t, ac, "ante handler must always install an AnteCache")

	const bound = cmdcfg.DefaultMempoolTxsPerBlock
	for i := 0; i < bound+1; i++ {
		ac.Set(fmt.Sprintf("addr-%d", i), 0)
	}
	require.LessOrEqual(t, ac.Size(), bound, "AnteCache must stay bounded when mempool.max-txs=0")
	require.False(t, ac.Exists("addr-0", 0), "oldest entry must be evicted once the cache is bounded")
}

// TestNewApp_AnteCacheUsesMempoolMaxTxsWhenPositive: a positive --mempool.max-txs
// still bounds the AnteCache at that same value (unchanged behavior).
func TestNewApp_AnteCacheUsesMempoolMaxTxsWhenPositive(t *testing.T) {
	const maxTxs = 3
	opts := baseTestAppOpts(0)
	opts[server.FlagMempoolMaxTxs] = maxTxs

	a := New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	ac := a.AnteCache()
	require.NotNil(t, ac)

	for i := 0; i < maxTxs+1; i++ {
		ac.Set(fmt.Sprintf("addr-%d", i), 0)
	}
	require.Equal(t, maxTxs, ac.Size(), "AnteCache must cap at mempool.max-txs when it's positive")
	require.False(t, ac.Exists("addr-0", 0), "oldest entry must be evicted at capacity")
}

// TestNewApp_AnteCacheNoOpWhenTxReplacementDisabled: cronos.disable-tx-replacement
// must still yield a no-op AnteCache (maxTx<0), independent of the maxTx==0
// substitution above.
func TestNewApp_AnteCacheNoOpWhenTxReplacementDisabled(t *testing.T) {
	opts := baseTestAppOpts(0)
	opts[server.FlagMempoolMaxTxs] = 0
	opts[FlagDisableTxReplacement] = true

	a := New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	ac := a.AnteCache()
	require.NotNil(t, ac)

	ac.Set("addr-0", 0)
	require.False(t, ac.Exists("addr-0", 0), "disable-tx-replacement must yield a no-op AnteCache")
}
