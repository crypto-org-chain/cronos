package mempool_test

import (
	"testing"

	"github.com/evmos/ethermint/evmd"
	"github.com/stretchr/testify/require"

	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

func TestEthSignerExtractionAdapter_SequenceIsEvmNonce(t *testing.T) {
	f := setupAdmissionApp(t, 1)
	acc := &f.accounts[0]
	acc.Nonce = 7 // distinct from the fresh account's cosmos sequence (0)

	bz := f.signTransfer(t, acc, nil)
	tx, err := f.app.TxConfig().TxDecoder()(bz)
	require.NoError(t, err)

	adapter := evmd.NewEthSignerExtractionAdapter(sdkmempool.NewDefaultSignerExtractionAdapter())
	signers, err := adapter.GetSigners(tx)
	require.NoError(t, err)
	require.Len(t, signers, 1)
	require.Equal(t, uint64(7), signers[0].Sequence,
		"adapter must map the EVM tx nonce, not the cosmos account sequence, into SignerData.Sequence")
}
