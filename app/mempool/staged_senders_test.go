package mempool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type multiMsgTx struct{ msgs []sdk.Msg }

// GetMsgs implements sdk.Tx.
func (t *multiMsgTx) GetMsgs() []sdk.Msg { return t.msgs }

// GetMsgsV2 implements sdk.Tx.
func (t *multiMsgTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func newEthMsg(from common.Address, nonce uint64) *evmtypes.MsgEthereumTx {
	msg := evmtypes.NewTx(nil, nonce, &common.Address{0xab}, big.NewInt(0), 21000, big.NewInt(1), nil, nil, nil, nil)
	msg.From = from.Bytes()
	return msg
}

func TestStageSendersIncludesEveryEthSender(t *testing.T) {
	a, b := common.Address{0x1}, common.Address{0x2}
	batch := &multiMsgTx{msgs: []sdk.Msg{newEthMsg(a, 0), newEthMsg(b, 5)}}
	expSenders := map[string]struct{}{
		sdk.AccAddress(a.Bytes()).String(): {},
		sdk.AccAddress(b.Bytes()).String(): {},
	}

	for _, tc := range []struct {
		name  string
		stage func(*Manager, [][]byte)
	}{
		{"committed block", func(m *Manager, txs [][]byte) { m.StageRecheckSenders(1, txs) }},
		{"proposal-skipped txs", (*Manager).StageSkippedSenders},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newManager(&stubRunner{}, nil, noopEncoder, func([]byte) (sdk.Tx, error) { return batch, nil })
			// The pool keys the batch by its first eth msg only.
			m.signer = fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
				batch: {sdkmempool.NewSignerData(a.Bytes(), 0)},
			}}

			tc.stage(m, [][]byte{[]byte("batch")})
			require.Equal(t, expSenders, m.recheckSenders)
		})
	}
}
