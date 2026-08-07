package keeper_test

import (
	"context"
	"math/big"
	"time"

	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (suite *KeeperTestSuite) TestReplayBlockConcurrencyLimit() {
	req := &types.ReplayBlockRequest{
		BlockNumber: 1,
		BlockTime:   suite.ctx.BlockTime(),
	}

	for i := 0; i < cronoskeeper.ReplayBlockConcurrencyLimit; i++ {
		cronoskeeper.ReplayBlockSemaphore <- struct{}{}
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := suite.app.CronosKeeper.ReplayBlock(suite.ctx.WithContext(callCtx), req)

	for i := 0; i < cronoskeeper.ReplayBlockConcurrencyLimit; i++ {
		<-cronoskeeper.ReplayBlockSemaphore
	}
	cronoskeeper.ResetReplayBlockQueued()

	suite.Require().Error(err)
	suite.Require().Equal(codes.DeadlineExceeded, status.Code(err))

	_, err = suite.app.CronosKeeper.ReplayBlock(suite.ctx, req)
	suite.Require().NoError(err)
}

func (suite *KeeperTestSuite) TestReplayBlockRejectsWhenQueueFull() {
	defer cronoskeeper.ResetReplayBlockQueued()

	req := &types.ReplayBlockRequest{
		BlockNumber: 1,
		BlockTime:   suite.ctx.BlockTime(),
	}

	cronoskeeper.SetReplayBlockQueued(cronoskeeper.ReplayBlockMaxQueued)
	_, err := suite.app.CronosKeeper.ReplayBlock(suite.ctx, req)
	suite.Require().Error(err)
	suite.Require().Equal(codes.ResourceExhausted, status.Code(err))

	cronoskeeper.SetReplayBlockQueued(cronoskeeper.ReplayBlockMaxQueued - 1)
	_, err = suite.app.CronosKeeper.ReplayBlock(suite.ctx, req)
	suite.Require().NoError(err)
}

func (suite *KeeperTestSuite) TestReplayBlockAbortsOnCancelledContext() {
	newMsg := func(gas uint64) *evmtypes.MsgEthereumTx {
		return evmtypes.NewTx(big.NewInt(1), 0, &suite.address, big.NewInt(0), gas, big.NewInt(1), nil, nil, nil, nil)
	}
	req := &types.ReplayBlockRequest{
		Msgs:        []*evmtypes.MsgEthereumTx{newMsg(1000), newMsg(1000), newMsg(1000)},
		BlockNumber: 1,
		BlockTime:   suite.ctx.BlockTime(),
	}

	callCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := suite.app.CronosKeeper.ReplayBlock(suite.ctx.WithContext(callCtx), req)
	suite.Require().Error(err)
	suite.Require().Equal(codes.Canceled, status.Code(err))
}
