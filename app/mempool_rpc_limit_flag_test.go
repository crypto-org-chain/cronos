package app

import (
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

type RPCMaxInflightFlagTestSuite struct {
	suite.Suite
}

func TestRPCMaxInflightFlagTestSuite(t *testing.T) {
	suite.Run(t, new(RPCMaxInflightFlagTestSuite))
}

func (s *RPCMaxInflightFlagTestSuite) TestParseAndWire() {
	setFlag := func(v any) func(simtestutil.AppOptionsMap) {
		return func(opts simtestutil.AppOptionsMap) { opts[FlagMempoolRPCMaxInflight] = v }
	}
	testCases := []struct {
		name      string
		malleate  func(opts simtestutil.AppOptionsMap)
		want      int
		wantPanic bool
	}{
		{"unset uses default", func(simtestutil.AppOptionsMap) {}, cmdcfg.DefaultMempoolRPCMaxInflight, false},
		{"string integer", setFlag("100"), 100, false},
		{"toml int64", setFlag(int64(100)), 100, false},
		{"zero disables the cap", setFlag(0), 0, false},
		{"negative", setFlag(-1), 0, true},
		{"bool", setFlag(true), 0, true},
		{"float", setFlag(8192.9), 0, true},
		{"fractional string", setFlag("8192.9"), 0, true},
		{"unparseable string", setFlag("abc"), 0, true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := baseTestAppOpts(0)
			opts[FlagMempoolType] = cronosmempool.TypeApp
			tc.malleate(opts)
			newApp := func() *App {
				return New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
			}

			if tc.wantPanic {
				defer func() {
					r := recover()
					s.Require().NotNil(r, "expected a panic")
					s.Contains(fmt.Sprint(r), FlagMempoolRPCMaxInflight)
				}()
				newApp()
				return
			}

			a := newApp()
			s.T().Cleanup(func() { s.Require().NoError(a.Close()) })
			s.Equal(tc.want, a.MempoolManager().RPCMaxInflight())
		})
	}
}

func (s *RPCMaxInflightFlagTestSuite) TestInvalidIgnoredWithoutAppMempool() {
	opts := baseTestAppOpts(0)
	opts[FlagMempoolRPCMaxInflight] = "abc"
	s.NotPanics(func() {
		a := New(log.NewNopLogger(), dbm.NewMemDB(), true, opts, baseapp.SetChainID(TestAppChainID))
		s.Require().NoError(a.Close())
	})
}
