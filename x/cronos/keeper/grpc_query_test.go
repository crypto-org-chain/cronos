package keeper_test

import (
	"math/big"

	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

// TestReplayBlockBounds exercises the DoS bounds on the public ReplayBlock
// gRPC query: the message-count cap, the per-message gas cap, and the
// cumulative gas budget.
func (suite *KeeperTestSuite) TestReplayBlockBounds() {
	// The block gas limit is unavailable in the query context, so the gas cap
	// is the fixed ReplayBlockGasCap constant.
	const gasCap = cronoskeeper.ReplayBlockGasCap

	newMsg := func(gas uint64) *evmtypes.MsgEthereumTx {
		return evmtypes.NewTx(big.NewInt(1), 0, &suite.address, big.NewInt(0), gas, big.NewInt(1), nil, nil, nil, nil)
	}

	testCases := []struct {
		name string
		msgs []*evmtypes.MsgEthereumTx
		// errMatch is the substring expected in the rejection error; empty means
		// the request must not be rejected by any of the bounds.
		errMatch string
	}{
		{
			name:     "too many messages",
			msgs:     make([]*evmtypes.MsgEthereumTx, types.MaxReplayBlockMsgs+1),
			errMatch: "too many messages",
		},
		{
			name:     "per-message gas cap",
			msgs:     []*evmtypes.MsgEthereumTx{newMsg(gasCap + 1)},
			errMatch: "exceeds ReplayBlock cap",
		},
		{
			name: "cumulative gas budget",
			// each message is within the per-message cap, but together they
			// exceed the 2*gasCap cumulative budget.
			msgs:     []*evmtypes.MsgEthereumTx{newMsg(gasCap), newMsg(gasCap), newMsg(gasCap)},
			errMatch: "cumulative message gas exceeds",
		},
		{
			name: "boundary block not rejected by bounds",
			// a real block sums to at most 2*gasCap (block limit plus one final
			// boundary tx); such a batch must pass all the bounds and only fail
			// later in the EVM execution path.
			msgs:     []*evmtypes.MsgEthereumTx{newMsg(gasCap), newMsg(gasCap)},
			errMatch: "",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := &types.ReplayBlockRequest{
				Msgs:        tc.msgs,
				BlockNumber: 1,
				BlockTime:   suite.ctx.BlockTime(),
			}
			_, err := suite.app.CronosKeeper.ReplayBlock(suite.ctx, req)
			suite.Require().Error(err)
			if tc.errMatch != "" {
				suite.Require().Contains(err.Error(), tc.errMatch)
			} else {
				// not rejected by the DoS bounds; fails later (e.g. signer/fee).
				suite.Require().NotContains(err.Error(), "exceeds ReplayBlock cap")
				suite.Require().NotContains(err.Error(), "cumulative message gas exceeds")
				suite.Require().NotContains(err.Error(), "too many messages")
			}
		})
	}
}
