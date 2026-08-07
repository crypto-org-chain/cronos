package app

import (
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
	"github.com/evmos/ethermint/ante/cache"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

type AnteCacheBoundTestSuite struct {
	suite.Suite
}

func TestAnteCacheBoundTestSuite(t *testing.T) {
	suite.Run(t, new(AnteCacheBoundTestSuite))
}

// newApp builds an App from opts and returns its installed AnteCache.
func (s *AnteCacheBoundTestSuite) newApp(opts simtestutil.AppOptionsMap) *cache.AnteCache {
	t := s.T()
	t.Helper()
	a := New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
	t.Cleanup(func() { s.Require().NoError(a.Close()) })

	ac := a.AnteCache()
	s.Require().NotNil(ac, "ante handler must always install an AnteCache")
	return ac
}

func (s *AnteCacheBoundTestSuite) TestBound() {
	testCases := []struct {
		name     string
		malleate func(opts simtestutil.AppOptionsMap)
		fill     int
		check    func(ac *cache.AnteCache)
	}{
		{
			name: "mempool.max-txs=0 falls back to a bounded default",
			malleate: func(opts simtestutil.AppOptionsMap) {
				opts[server.FlagMempoolMaxTxs] = 0
			},
			fill: cmdcfg.DefaultMaxTxPerBlock + 1,
			check: func(ac *cache.AnteCache) {
				s.Require().Equal(cmdcfg.DefaultMaxTxPerBlock, ac.Size(), "AnteCache must stay bounded when mempool.max-txs=0")
				s.Require().False(ac.Exists("addr-0", 0), "oldest entry must be evicted once the cache is bounded")
			},
		},
		{
			name: "positive mempool.max-txs caps the cache at that value",
			malleate: func(opts simtestutil.AppOptionsMap) {
				opts[server.FlagMempoolMaxTxs] = 3
			},
			fill: 4,
			check: func(ac *cache.AnteCache) {
				s.Require().Equal(3, ac.Size(), "AnteCache must cap at mempool.max-txs when it's positive")
				s.Require().False(ac.Exists("addr-0", 0), "oldest entry must be evicted at capacity")
			},
		},
		{
			name: "disable-tx-replacement yields a no-op cache",
			malleate: func(opts simtestutil.AppOptionsMap) {
				opts[server.FlagMempoolMaxTxs] = 0
				opts[FlagDisableTxReplacement] = true
			},
			fill: 1,
			check: func(ac *cache.AnteCache) {
				s.Require().False(ac.Exists("addr-0", 0), "disable-tx-replacement must yield a no-op AnteCache")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := baseTestAppOpts(0)
			tc.malleate(opts)
			ac := s.newApp(opts)

			for i := 0; i < tc.fill; i++ {
				ac.Set(fmt.Sprintf("addr-%d", i), 0)
			}
			tc.check(ac)
		})
	}
}
