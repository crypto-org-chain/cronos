package mempool

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestUnorderedPoolSnapshot_SameSetAsPoolSnapshot(t *testing.T) {
	tx1, tx2, tx3 := &ptrTx{id: 1}, &ptrTx{id: 2}, &ptrTx{id: 3}
	pool := &fakePool{txs: []sdk.Tx{tx1, tx2, tx3}}

	ctx := context.Background()
	ordered := PoolSnapshot(ctx, pool)
	unordered := UnorderedPoolSnapshot(ctx, pool)

	if len(unordered) != len(ordered) {
		t.Fatalf("len(unordered) = %d, want %d", len(unordered), len(ordered))
	}

	orderedSet := make(map[sdk.Tx]bool, len(ordered))
	for _, tx := range ordered {
		orderedSet[tx] = true
	}
	for _, tx := range unordered {
		if !orderedSet[tx] {
			t.Fatalf("UnorderedPoolSnapshot returned a tx not in PoolSnapshot's set")
		}
	}
}
