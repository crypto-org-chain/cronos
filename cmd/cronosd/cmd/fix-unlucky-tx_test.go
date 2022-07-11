package cmd

import (
	"bytes"
	"log"
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
	tmdb "github.com/tendermint/tm-db"
)

func mockBlockResult() *tmstate.ABCIResponses {
	return &tmstate.ABCIResponses{
		DeliverTxs: []*abci.ResponseDeliverTx{
			{Code: 0, Data: []byte{0x01}, Log: "ok"},
			{Code: 1, Log: "not ok"},
		},
		EndBlock:   &abci.ResponseEndBlock{},
		BeginBlock: &abci.ResponseBeginBlock{},
	}
}

func mockResult(txGen client.TxConfig) *abci.TxResult {
	txs, _ := app.GenSequenceOfTxs(txGen, nil, nil, nil, 1)
	txBytes, _ := txGen.TxEncoder()(txs[0])
	return &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     txBytes,
		Result: abci.ResponseDeliverTx{
			Code: abci.CodeTypeOK,
		},
	}
}

func getExpected(result *abci.TxResult, blockRes *tmstate.ABCIResponses) []byte {
	expected := new(bytes.Buffer)
	protoWriter := protoio.NewDelimitedWriter(expected)
	for _, res := range []proto.Message{result, blockRes} {
		_, err := protoWriter.WriteMsg(res)
		if err != nil {
			log.Fatal(err)
		}
	}
	protoWriter.Close()
	return expected.Bytes()
}

func TestPatchToExport(t *testing.T) {
	encCfg := simapp.MakeTestEncodingConfig()
	db := tmdb.NewMemDB()
	tmDB := &tmDB{
		blockStore: tmstore.NewBlockStore(db),
		stateStore: sm.NewStore(db),
		txIndexer:  kv.NewTxIndex(db),
	}
	t.Run("TestPatchToExport", func(t *testing.T) {
		blockRes := mockBlockResult()
		res := mockResult(encCfg.TxConfig)
		expected := getExpected(res, blockRes)
		b := new(bytes.Buffer)
		err := tmDB.PatchToExport(blockRes, res, b)
		require.NoError(t, err)
		require.Equal(t, b.Bytes(), expected)
	})
}
