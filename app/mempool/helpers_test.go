package mempool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

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

type declaredGasTx struct {
	ptrTx
	gas uint64
}

func (t *declaredGasTx) GetGas() uint64 { return t.gas }

type BlockGasTestSuite struct {
	suite.Suite
}

func TestBlockGasTestSuite(t *testing.T) {
	suite.Run(t, new(BlockGasTestSuite))
}

func (s *BlockGasTestSuite) TestBlockGas() {
	testCases := []struct {
		name      string
		tx        sdk.Tx
		gasWanted uint64
		expGas    uint64
	}{
		{"ante gas wanted overrides declared gas", &declaredGasTx{gas: 100}, 40, 40},
		{"declared gas when ante reports none", &declaredGasTx{gas: 100}, 0, 100},
		{"ante gas wanted for a tx without declared gas", &ptrTx{}, 40, 40},
		{"zero for a tx without declared or ante gas", &ptrTx{}, 0, 0},
		{"zero for a nil tx", nil, 0, 0},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Require().Equal(tc.expGas, BlockGas(tc.tx, tc.gasWanted))
		})
	}
}
