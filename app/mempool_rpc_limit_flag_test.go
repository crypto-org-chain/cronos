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
)

type RPCMaxInflightFlagTestSuite struct {
	suite.Suite
}

func TestRPCMaxInflightFlagTestSuite(t *testing.T) {
	suite.Run(t, new(RPCMaxInflightFlagTestSuite))
}

func (s *RPCMaxInflightFlagTestSuite) TestParseAndWire() {
	testCases := []struct {
		name      string
		value     any // nil leaves the flag unset
		want      int
		wantPanic bool
	}{
		{"unset uses default", nil, cmdcfg.DefaultMempoolRPCMaxInflight, false},
		{"string integer", "100", 100, false},
		{"zero disables the cap", 0, 0, false},
		{"negative", -1, 0, true},
		{"bool", true, 0, true},
		{"unparseable string", "abc", 0, true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := baseTestAppOpts(0)
			opts[FlagMempoolType] = cronosmempool.TypeApp
			if tc.value != nil {
				opts[FlagMempoolRPCMaxInflight] = tc.value
			}
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
