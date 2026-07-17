package mempool_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"
)

// TestSenderCacheServesFinalizeBlockAfterDecodeCacheEviction proves the
// hash-keyed sender cache (not just go-ethereum's per-object sigCache) is what
// lets FinalizeBlock skip a redundant ecrecover after the tx encode/decode
// cache has evicted the tx and it's re-decoded into a fresh object.
func TestSenderCacheServesFinalizeBlockAfterDecodeCacheEviction(t *testing.T) {
	const accounts = 4
	// tx-cache-size=1 guarantees the first tx's DecodeCache entry is evicted by
	// the time the later admissions below run.
	f := setupAdmissionAppWithOpts(t, accounts, minimalOptionsMap{"cronos.tx-cache-size": 1})
	require.NotNil(t, f.app.SenderCache(), "sender cache must be enabled by default")

	txBytes := f.signTransfer(t, &f.accounts[0], nil)
	resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: txBytes})
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, resp.Code)

	// Evict tx0's DecodeCache entry by admitting other txs into the size-1 cache.
	for i := 1; i < accounts; i++ {
		bz := f.signTransfer(t, &f.accounts[i], nil)
		resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: bz})
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, resp.Code)
	}

	hitsBefore, missesBefore := f.app.SenderCache().Stats()

	// FinalizeBlock re-decodes tx0's raw bytes into a fresh *MsgEthereumTx (the
	// DecodeCache entry is gone), so this only avoids a real ecrecover if the
	// hash-keyed sender cache — populated by admission's pre-verify — is hit.
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
