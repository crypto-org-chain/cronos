package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ParamsTestSuite struct {
	suite.Suite
}

func TestParamsTestSuite(t *testing.T) {
	suite.Run(t, new(ParamsTestSuite))
}

func (s *ParamsTestSuite) TestValidate() {
	testCases := []struct {
		name     string
		malleate func(*Params)
		expErr   bool
	}{
		{"default", func(*Params) {}, false},
		{"ibc denom of wrong length", func(p *Params) { p.IbcCroDenom = "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD" }, true},
		{"ibc denom without ibc prefix", func(p *Params) {
			p.IbcCroDenom = "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865"
		}, true},
		{"zero ibc timeout", func(p *Params) { p.IbcTimeout = 0 }, false},
		{"ibc timeout at max", func(p *Params) { p.IbcTimeout = MaxIbcTimeoutValue }, false},
		{"ibc timeout above max", func(p *Params) { p.IbcTimeout = MaxIbcTimeoutValue + 1 }, true},
		{"max uint64 ibc timeout wraps into the past", func(p *Params) { p.IbcTimeout = math.MaxUint64 }, true},
		{"admin address", func(p *Params) { p.CronosAdmin = sdk.AccAddress(make([]byte, 20)).String() }, false},
		{"malformed admin address", func(p *Params) { p.CronosAdmin = "a" }, true},
		{"bridge contract addresses", func(p *Params) {
			p.CroBridgeContractAddresses = []string{"0x0000000000000000000000000000000000000001"}
		}, false},
		{"malformed bridge contract address", func(p *Params) { p.CroBridgeContractAddresses = []string{"0x1"} }, true},
		{"zero bridge contract address", func(p *Params) {
			p.CroBridgeContractAddresses = []string{"0x0000000000000000000000000000000000000000"}
		}, true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			params := DefaultParams()
			tc.malleate(&params)
			err := params.Validate()
			if tc.expErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *ParamsTestSuite) TestMaxIbcTimeoutCannotOverflowBlockTime() {
	s.Require().Greater(uint64(math.MaxUint64)-MaxIbcTimeoutValue, uint64(time.Now().UnixNano()))
}
