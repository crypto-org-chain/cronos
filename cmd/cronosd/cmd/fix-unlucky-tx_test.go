package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/crypto-org-chain/cronos/app"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/protoio"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/txindex/kv"
	tmstore "github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
)

type MockTxResult struct {
	Origin                    *abci.TxResult
	ReplayedResponseDeliverTx *abci.ResponseDeliverTx
	NoIndexed                 bool
}

func getExpected(result *abci.TxResult, blockRes *tmstate.ABCIResponses) []byte {
	expected := new(bytes.Buffer)
	protoWriter := protoio.NewDelimitedWriter(expected)
	results := make([]proto.Message, 0)
	if result != nil {
		results = append(results, result)
	}
	if blockRes != nil {
		results = append(results, blockRes)
	}
	for _, res := range results {
		_, err := protoWriter.WriteMsg(res)
		if err != nil {
			log.Fatal(err)
		}
	}
	protoWriter.Close()
	return expected.Bytes()
}

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

func TestPatchToExport(t *testing.T) {
	encCfg := simapp.MakeTestEncodingConfig()
	tmDB := mockTmDb()
	t.Run("TestPatchToExport", func(t *testing.T) {
		blockRes := mockBlockResult()
		res := mockResult(encCfg.TxConfig, 0, true)
		expected := getExpected(res, blockRes)
		b := new(bytes.Buffer)
		err := tmDB.PatchToExport(blockRes, res, b)
		require.NoError(t, err)
		require.Equal(t, b.Bytes(), expected)
	})
}

func TestPatchFromImport(t *testing.T) {
	tmDB := mockTmDb()
	encCfg := simapp.MakeTestEncodingConfig()

	t.Run("happy flow", func(t *testing.T) {
		res := mockResult(encCfg.TxConfig, 0, true)
		blockRes := mockBlockResult()
		blockRes.DeliverTxs = append(blockRes.DeliverTxs, mockResponseDeliverTx(false))
		blockRes.DeliverTxs[res.Index] = &res.Result
		expected := getExpected(res, blockRes)
		err := tmDB.PatchFromImport(encCfg.TxConfig, bytes.NewReader(expected))
		require.NoError(t, err, "import error")
		txHash := types.Tx(res.Tx).Hash()
		newRes, err := tmDB.txIndexer.Get(txHash)
		require.NoError(t, err, "get tx result")
		resultProto, _ := res.Marshal()
		newResProto, _ := newRes.Marshal()
		require.Equal(t, resultProto, newResProto, "check tx result")
		newBlockRes, err := tmDB.stateStore.LoadABCIResponses(res.Height)
		require.NoError(t, err, "get block rseult")
		blockResProto, _ := blockRes.Marshal()
		newBlockResProto, _ := newBlockRes.Marshal()
		require.Equal(t, blockResProto, newBlockResProto, "check block result")
	})

	t.Run("wrong object type", func(t *testing.T) {
		blockRes := mockBlockResult()
		expected := getExpected(nil, blockRes)
		err := tmDB.PatchFromImport(encCfg.TxConfig, bytes.NewReader(expected))
		require.EqualError(t, err, "proto: wrong wireType = 2 for field Index")
	})

	t.Run("wrong last object", func(t *testing.T) {
		res := mockResult(encCfg.TxConfig, 0, true)
		expected := getExpected(res, nil)
		err := tmDB.PatchFromImport(encCfg.TxConfig, bytes.NewReader(expected))
		require.EqualError(t, err, "EOF")
	})
}

func TestFindUnluckyTx(t *testing.T) {
	// rm time prefix in test
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	defer func() {
		log.SetOutput(os.Stderr)
	}()
	encCfg := simapp.MakeTestEncodingConfig()
	tmDB := mockTmDb()
	testCases := []struct {
		name           string
		txResults      []MockTxResult
		expTxIndex     int
		expSkipTxIndex int
	}{
		{
			"no unlucky tx",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, true)},
				{Origin: mockResult(encCfg.TxConfig, 2, true)},
			},
			-1,
			-1,
		},
		{
			"find unlucky tx",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, true)},
				{Origin: mockResult(encCfg.TxConfig, 2, false)},
			},
			2,
			-1,
		},
		{
			"find unlucky tx when indexed as success",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, false), ReplayedResponseDeliverTx: mockResponseDeliverTx(true)},
				{Origin: mockResult(encCfg.TxConfig, 2, false)},
			},
			2,
			1,
		},
		{
			"find unlucky tx when no indexed",
			[]MockTxResult{
				{Origin: mockResult(encCfg.TxConfig, 0, true)},
				{Origin: mockResult(encCfg.TxConfig, 1, false), NoIndexed: true},
				{Origin: mockResult(encCfg.TxConfig, 2, false)},
			},
			1,
			-1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
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
				if !txResults.NoIndexed {
					err := tmDB.txIndexer.Index(txResult)
					require.NoError(t, err)
				}
			}
			txIndex, err := tmDB.FindUnluckyTx(blockRes, block)
			require.NoError(t, err)
			require.Equal(t, txIndex, tc.expTxIndex)
			if tc.expSkipTxIndex >= 0 {
				tx := block.Txs[tc.expSkipTxIndex]
				require.Equal(t, buf.String(), fmt.Sprintf("skip %x at index %d for height %d\n", tx.Hash(), tc.expSkipTxIndex, block.Height))
			}
		})
	}
}
