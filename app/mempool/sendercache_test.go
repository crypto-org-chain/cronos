package mempool_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"
)

func TestSenderCacheServesFinalizeBlockAfterDecodeCacheEviction(t *testing.T) {
	const accounts = 4
	f := setupAdmissionAppWithOpts(t, accounts, minimalOptionsMap{"cronos.tx-cache-size": 1})
	require.NotNil(t, f.app.SenderCache(), "sender cache must be enabled by default")

	txBytes := f.signTransfer(t, &f.accounts[0], nil)
	resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: txBytes})
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, resp.Code)

	for i := 1; i < accounts; i++ {
		bz := f.signTransfer(t, &f.accounts[i], nil)
		resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: bz})
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, resp.Code)
	}

	hitsBefore, missesBefore := f.app.SenderCache().Stats()

	_, err = f.app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Txs:             [][]byte{txBytes},
		Height:          2,
		ProposerAddress: f.consAddress,
	})
	require.NoError(t, err)
	_, err = f.app.Commit()
	require.NoError(t, err)

	hitsAfter, missesAfter := f.app.SenderCache().Stats()
	require.Equal(t, hitsBefore+1, hitsAfter, "FinalizeBlock's ante pass must hit the cache admission populated")
	require.Equal(t, missesBefore, missesAfter, "a hit must not fall through to a fresh ecrecover")
}
