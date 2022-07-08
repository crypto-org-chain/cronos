package cmd

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/crypto-org-chain/cronos/app"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/txindex/kv"
	tmstore "github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
)

func mockResult(txGen client.TxConfig, index uint32, success bool) *abci.TxResult {
	txs, _ := app.GenSequenceOfTxs(txGen, nil, nil, nil, 1)
	txBytes, _ := txGen.TxEncoder()(txs[0])
	result := mockResponseDeliverTx(success)
	return &abci.TxResult{
		Height: 1,
		Index:  index,
		Tx:     txBytes,
		Result: *result,
	}
}

func mockResponseDeliverTx(success bool) *abci.ResponseDeliverTx {
	result := abci.ResponseDeliverTx{}
	if success {
		result.Code = abci.CodeTypeOK
		result.Data = []byte{0x01}
		result.Log = "ok"
	} else {
		result.Code = 11
		result.Log = "out of gas in location: block gas meter; gasWanted:"
	}
	return &result
}

func mockBlockResult() *tmstate.ABCIResponses {
	return &tmstate.ABCIResponses{
		DeliverTxs: make([]*abci.ResponseDeliverTx, 0),
		EndBlock:   &abci.ResponseEndBlock{},
		BeginBlock: &abci.ResponseBeginBlock{},
	}
}

func mockTmDb() *tmDB {
	db := tmdb.NewMemDB()
	return &tmDB{
		blockStore: tmstore.NewBlockStore(db),
		stateStore: sm.NewStore(db),
		txIndexer:  kv.NewTxIndex(db),
	}
}

type MockTxResult struct {
	Origin                    *abci.TxResult
	ReplayedResponseDeliverTx *abci.ResponseDeliverTx
}

func TestFindUnluckyTx(t *testing.T) {
	encCfg := simapp.MakeTestEncodingConfig()
	tmDB := mockTmDb()
	testCases := []struct {
		name       string
		txResults  []MockTxResult
		expTxIndex int
	}{
		{
			"no unlucky tx",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, true)},
				{Origin: mockResult(encCfg.TxConfig, 2, true)},
			},
			-1,
		},
		{
			"find unlucky tx",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, false)},
				{Origin: mockResult(encCfg.TxConfig, 2, false)},
			},
			1,
		},
		{
			"find unlucky tx when indexed as success",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, false), ReplayedResponseDeliverTx: mockResponseDeliverTx(true)},
				{Origin: mockResult(encCfg.TxConfig, 2, false)},
			},
			2,
		},
		{
			"find unlucky tx when indexed as fail",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, false), ReplayedResponseDeliverTx: mockResponseDeliverTx(false)},
				{Origin: mockResult(encCfg.TxConfig, 2, false)},
			},
			1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			block := &types.Block{}
			blockRes := mockBlockResult()
			for _, txResults := range tc.txResults {
				txResult := txResults.Origin
				result := txResult.Result
				blockRes.DeliverTxs = append(blockRes.DeliverTxs, &result)
				block.Txs = append(block.Txs, txResult.Tx)
				if txResults.ReplayedResponseDeliverTx != nil {
					txResult.Result = *txResults.ReplayedResponseDeliverTx
				}
				err := tmDB.txIndexer.Index(txResult)
				require.NoError(t, err)
			}
			txIndex, err := tmDB.FindUnluckyTx(blockRes, block)
			require.NoError(t, err)
			require.Equal(t, txIndex, tc.expTxIndex)
		})
	}
}
