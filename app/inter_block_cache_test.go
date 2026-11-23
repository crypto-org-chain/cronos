package app

import (
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	memiavlstore "github.com/crypto-org-chain/cronos-store/store"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/store/v2"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

type countingInterBlockCache struct {
	storetypes.MultiStorePersistentCache
	wrappedStores int
}

func (c *countingInterBlockCache) GetStoreCache(key storetypes.StoreKey, s storetypes.CommitKVStore) storetypes.CommitKVStore {
	c.wrappedStores++
	return c.MultiStorePersistentCache.GetStoreCache(key, s)
}

type InterBlockCacheTestSuite struct {
	suite.Suite
}

func TestInterBlockCacheTestSuite(t *testing.T) {
	suite.Run(t, new(InterBlockCacheTestSuite))
}

func (s *InterBlockCacheTestSuite) TestNeverWrapsStores() {
	testCases := []struct {
		name     string
		malleate func(opts simtestutil.AppOptionsMap)
	}{
		{"iavl, default mempool", func(simtestutil.AppOptionsMap) {}},
		{"iavl, app mempool", func(opts simtestutil.AppOptionsMap) {
			opts[FlagMempoolType] = cronosmempool.TypeApp
		}},
		{"iavl, flag disabled", func(opts simtestutil.AppOptionsMap) {
			opts[server.FlagInterBlockCache] = false
		}},
		{"memiavl, default mempool", func(opts simtestutil.AppOptionsMap) {
			opts[memiavlstore.FlagMemIAVL] = true
		}},
		{"memiavl, app mempool", func(opts simtestutil.AppOptionsMap) {
			opts[memiavlstore.FlagMemIAVL] = true
			opts[FlagMempoolType] = cronosmempool.TypeApp
		}},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := baseTestAppOpts(0)
			opts[flags.FlagHome] = s.T().TempDir()
			opts[flags.FlagChainID] = TestAppChainID
			opts[server.FlagInterBlockCache] = true
			tc.malleate(opts)

			cache := &countingInterBlockCache{MultiStorePersistentCache: store.NewCommitKVStoreCacheManager()}
			a := New(
				log.NewNopLogger(), dbm.NewMemDB(), true, opts,
				baseapp.SetChainID(TestAppChainID), baseapp.SetInterBlockCache(cache),
			)
			s.T().Cleanup(func() { s.Require().NoError(a.Close()) })

			s.Zero(cache.wrappedStores)
		})
	}
}
