package app

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	memiavlstore "github.com/crypto-org-chain/cronos-store/store"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/store/v2/cache"
	pruningtypes "github.com/cosmos/cosmos-sdk/store/v2/pruning/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type InterBlockCacheTestSuite struct {
	suite.Suite
}

func TestInterBlockCacheTestSuite(t *testing.T) {
	suite.Run(t, new(InterBlockCacheTestSuite))
}

func (s *InterBlockCacheTestSuite) TestFlagIsIgnored() {
	testCases := []struct {
		name     string
		malleate func(opts simtestutil.AppOptionsMap)
		wantWarn bool
	}{
		{"iavl, default mempool", func(simtestutil.AppOptionsMap) {}, true},
		{"iavl, app mempool", func(opts simtestutil.AppOptionsMap) {
			opts[FlagMempoolType] = cronosmempool.TypeApp
		}, true},
		{"iavl, flag disabled", func(opts simtestutil.AppOptionsMap) {
			opts[server.FlagInterBlockCache] = false
		}, false},
		// memiavl never wrapped its stores, these cases guard that and the silent log.
		{"memiavl, default mempool", func(opts simtestutil.AppOptionsMap) {
			opts[memiavlstore.FlagMemIAVL] = true
		}, false},
		{"memiavl, app mempool", func(opts simtestutil.AppOptionsMap) {
			opts[memiavlstore.FlagMemIAVL] = true
			opts[FlagMempoolType] = cronosmempool.TypeApp
		}, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := baseTestAppOpts(0)
			opts[flags.FlagHome] = s.T().TempDir()
			opts[flags.FlagChainID] = TestAppChainID
			opts[server.FlagPruning] = pruningtypes.PruningOptionDefault
			opts[server.FlagInterBlockCache] = true
			tc.malleate(opts)

			logs := &lockedBuffer{}
			a := New(log.NewLogger(logs), dbm.NewMemDB(), true, opts, server.DefaultBaseappOptions(opts)...)
			s.T().Cleanup(func() { s.Require().NoError(a.Close()) })

			for _, key := range a.keys {
				_, wrapped := a.CommitMultiStore().GetKVStore(key).(*cache.CommitKVStoreCache)
				s.False(wrapped, "store %s is wrapped by the inter-block cache", key.Name())
			}
			s.Equal(tc.wantWarn, strings.Contains(logs.String(), "inter-block-cache is ignored"))
		})
	}
}
