package precompiles

import (
	"testing"

	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"
)

type RelayerContractTestSuite struct {
	suite.Suite
}

func TestRelayerContractTestSuite(t *testing.T) {
	suite.Run(t, new(RelayerContractTestSuite))
}

func (s *RelayerContractTestSuite) TestRequiredGas() {
	rules := params.Rules{IsHomestead: true, IsIstanbul: true, IsShanghai: true, IsCancun: true, IsPrague: true, IsOsaka: true}
	// Pinned to the pre-Amsterdam intrinsic gas: 21000 plus 16 per nonzero and 4 per zero calldata byte.
	testCases := []struct {
		name   string
		input  []byte
		expGas uint64
	}{
		{
			name:   "input shorter than a method selector",
			input:  []byte{1, 0, 2},
			expGas: DefaultGasRequired + 3*10 - (21000 + 2*16 + 4),
		},
		{
			name:   "unknown method selector",
			input:  []byte{0xff, 0xff, 0xff, 0xff},
			expGas: DefaultGasRequired + 4*10 - (21000 + 4*16),
		},
	}

	contract := NewRelayerContract(nil, nil, rules, log.NewNopLogger())
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Require().Equal(tc.expGas, contract.RequiredGas(tc.input))
		})
	}
}
