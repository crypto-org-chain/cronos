package rpc

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func newResBlock(t *testing.T, numTxs int) *coretypes.ResultBlock {
	t.Helper()
	txs := make(cmttypes.Txs, numTxs)
	for i := range txs {
		txs[i] = cmttypes.Tx{byte(i)}
	}
	return &coretypes.ResultBlock{
		Block: &cmttypes.Block{
			Header: cmttypes.Header{Height: 100},
			Data:   cmttypes.Data{Txs: txs},
		},
	}
}

func newBlockRes(t *testing.T, numResults int) *coretypes.ResultBlockResults {
	t.Helper()
	results := make([]*abci.ExecTxResult, numResults)
	for i := range results {
		results[i] = &abci.ExecTxResult{}
	}
	return &coretypes.ResultBlockResults{TxsResults: results}
}

type CheckTxsResultsLengthTestSuite struct {
	suite.Suite
}

func TestCheckTxsResultsLengthTestSuite(t *testing.T) {
	suite.Run(t, new(CheckTxsResultsLengthTestSuite))
}

func (s *CheckTxsResultsLengthTestSuite) TestCheckTxsResultsLength() {
	testCases := []struct {
		name    string
		numTxs  int
		numRes  int
		wantErr string
	}{
		{
			name:   "matching lengths returns no error",
			numTxs: 3,
			numRes: 3,
		},
		{
			name:    "fewer tx results than txs returns error instead of panicking",
			numTxs:  3,
			numRes:  2,
			wantErr: "mismatched tx results length at height 100: block has 3 txs, but got 2 tx results",
		},
		{
			name:    "more tx results than txs returns error",
			numTxs:  2,
			numRes:  3,
			wantErr: "mismatched tx results length at height 100: block has 2 txs, but got 3 tx results",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			t := s.T()
			err := checkTxsResultsLength(newResBlock(t, tc.numTxs), newBlockRes(t, tc.numRes))
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}
