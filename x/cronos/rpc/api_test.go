package rpc

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"
)

func TestCheckTxsResultsLength(t *testing.T) {
	newResBlock := func(numTxs int) *coretypes.ResultBlock {
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
	newBlockRes := func(numResults int) *coretypes.ResultBlockResults {
		results := make([]*abci.ExecTxResult, numResults)
		for i := range results {
			results[i] = &abci.ExecTxResult{}
		}
		return &coretypes.ResultBlockResults{TxsResults: results}
	}

	t.Run("matching lengths returns no error", func(t *testing.T) {
		err := checkTxsResultsLength(newResBlock(3), newBlockRes(3))
		require.NoError(t, err)
	})

	t.Run("fewer tx results than txs returns error instead of panicking", func(t *testing.T) {
		err := checkTxsResultsLength(newResBlock(3), newBlockRes(2))
		require.EqualError(t, err, "mismatched tx results length at height 100: block has 3 txs, but got 2 tx results")
	})

	t.Run("more tx results than txs returns error", func(t *testing.T) {
		err := checkTxsResultsLength(newResBlock(2), newBlockRes(3))
		require.EqualError(t, err, "mismatched tx results length at height 100: block has 2 txs, but got 3 tx results")
	})
}
