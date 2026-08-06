package app

import (
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
	"github.com/evmos/ethermint/ante/cache"
)

// newAnteCacheTestApp builds an App from opts and returns its installed AnteCache.
func newAnteCacheTestApp(t *testing.T, opts simtestutil.AppOptionsMap) *cache.AnteCache {
	t.Helper()
	a := New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	ac := a.AnteCache()
	require.NotNil(t, ac, "ante handler must always install an AnteCache")
	return ac
}

func TestNewApp_AnteCacheBoundedWhenMempoolMaxTxsZero(t *testing.T) {
	opts := baseTestAppOpts(0)
	opts[server.FlagMempoolMaxTxs] = 0
	ac := newAnteCacheTestApp(t, opts)

	const bound = cmdcfg.DefaultMempoolTxsPerBlock
	for i := 0; i < bound+1; i++ {
		ac.Set(fmt.Sprintf("addr-%d", i), 0)
	}
	require.LessOrEqual(t, ac.Size(), bound, "AnteCache must stay bounded when mempool.max-txs=0")
	require.False(t, ac.Exists("addr-0", 0), "oldest entry must be evicted once the cache is bounded")
}

func TestNewApp_AnteCacheUsesMempoolMaxTxsWhenPositive(t *testing.T) {
	const maxTxs = 3
	opts := baseTestAppOpts(0)
	opts[server.FlagMempoolMaxTxs] = maxTxs
	ac := newAnteCacheTestApp(t, opts)

	for i := 0; i < maxTxs+1; i++ {
		ac.Set(fmt.Sprintf("addr-%d", i), 0)
	}
	require.Equal(t, maxTxs, ac.Size(), "AnteCache must cap at mempool.max-txs when it's positive")
	require.False(t, ac.Exists("addr-0", 0), "oldest entry must be evicted at capacity")
}

func TestNewApp_AnteCacheNoOpWhenTxReplacementDisabled(t *testing.T) {
	opts := baseTestAppOpts(0)
	opts[server.FlagMempoolMaxTxs] = 0
	opts[FlagDisableTxReplacement] = true
	ac := newAnteCacheTestApp(t, opts)

	ac.Set("addr-0", 0)
	require.False(t, ac.Exists("addr-0", 0), "disable-tx-replacement must yield a no-op AnteCache")
}
