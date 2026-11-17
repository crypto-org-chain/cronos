package mempool

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type PriorityPoolTestSuite struct {
	suite.Suite
}

func TestPriorityPoolTestSuite(t *testing.T) {
	suite.Run(t, new(PriorityPoolTestSuite))
}

func (s *PriorityPoolTestSuite) TestInsertResend() {
	sender := common.Address{0x1}
	pooledEth := newEthTx(sender, 0)
	resendEth := newEthTx(sender, 0)
	bumpedMsg := evmtypes.NewTx(nil, 0, &common.Address{0xab}, big.NewInt(0), 21000, big.NewInt(2), nil, nil, nil, nil)
	bumpedMsg.From = sender.Bytes()
	bumpedEth := &ethTx{msg: bumpedMsg}
	pooledCosmos, resendCosmos := &ptrTx{id: 1}, &ptrTx{id: 2}
	errRejected := errors.New("replacement rejected")

	s.Require().Equal(pooledEth.msg.Hash(), resendEth.msg.Hash())
	s.Require().NotEqual(pooledEth.msg.Hash(), bumpedEth.msg.Hash())

	testCases := []struct {
		name       string
		pooled     sdk.Tx
		tx         sdk.Tx
		allow      bool
		maxTx      int
		expErr     error
		expReplace bool
	}{
		{name: "exact eth resend is reported as in mempool", pooled: pooledEth, tx: resendEth, allow: true, expErr: sdkerrors.ErrTxInMempoolCache},
		{name: "exact eth resend is reported even when the rule would reject", pooled: pooledEth, tx: resendEth, expErr: sdkerrors.ErrTxInMempoolCache},
		{name: "different eth tx replaces when the rule allows", pooled: pooledEth, tx: bumpedEth, allow: true, expReplace: true},
		{name: "different eth tx keeps the rule's rejection", pooled: pooledEth, tx: bumpedEth, expErr: errRejected},
		{name: "non-eth tx still goes through the rule", pooled: pooledCosmos, tx: resendCosmos, allow: true, expReplace: true},
		{name: "exact eth resend into a full pool is reported as in mempool", pooled: pooledEth, tx: resendEth, allow: true, maxTx: 1, expErr: sdkerrors.ErrTxInMempoolCache},
		{name: "different eth tx into a full pool keeps the capacity error", pooled: pooledEth, tx: bumpedEth, allow: true, maxTx: 1, expErr: sdkmempool.ErrMempoolTxMaxCapacity},
		{name: "non-eth tx into a full pool keeps the capacity error", pooled: pooledCosmos, tx: resendCosmos, allow: true, maxTx: 1, expErr: sdkmempool.ErrMempoolTxMaxCapacity},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			signerData := []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
			pool := NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
				TxPriority: sdkmempool.NewDefaultTxPriority(),
				SignerExtractor: fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
					tc.pooled: signerData,
					tc.tx:     signerData,
				}},
				TxReplacement: func(_, _ int64, _, _ sdk.Tx) bool { return tc.allow },
				MaxTx:         tc.maxTx,
			}, nil)
			s.Require().NoError(pool.Insert(sdk.Context{}, tc.pooled, sdkmempool.InsertOption{}))

			err := pool.Insert(sdk.Context{}, tc.tx, sdkmempool.InsertOption{})
			switch {
			case errors.Is(tc.expErr, errRejected):
				s.Require().Error(err)
				s.Require().NotErrorIs(err, sdkerrors.ErrTxInMempoolCache)
			case tc.expErr != nil:
				s.Require().ErrorIs(err, tc.expErr)
			default:
				s.Require().NoError(err)
			}
			s.Require().Equal(1, pool.CountTx())
			s.Require().Equal(!tc.expReplace, poolHas(pool.PriorityNonceMempool, tc.pooled))
			s.Require().Equal(tc.expReplace, poolHas(pool.PriorityNonceMempool, tc.tx))
		})
	}
}

func (s *PriorityPoolTestSuite) TestInsertResendDoesNotLeak() {
	sender := common.Address{0x1}
	first, resend, next := newEthTx(sender, 0), newEthTx(sender, 0), newEthTx(sender, 1)
	alice := sdk.AccAddress("alice")
	pool := NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority: sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
			first:  {sdkmempool.NewSignerData(alice, 0)},
			resend: {sdkmempool.NewSignerData(alice, 0)},
			next:   {sdkmempool.NewSignerData(alice, 1)},
		}},
	}, nil)

	s.Require().NoError(pool.Insert(sdk.Context{}, first, sdkmempool.InsertOption{}))
	s.Require().ErrorIs(pool.Insert(sdk.Context{}, resend, sdkmempool.InsertOption{}), sdkerrors.ErrTxInMempoolCache)
	s.Require().NoError(pool.Insert(sdk.Context{}, next, sdkmempool.InsertOption{}))
	s.Require().Equal(2, pool.CountTx())
}
