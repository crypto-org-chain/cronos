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

// TestReplayBlockConcurrencyLimit exercises the aggregate concurrency bound
// on the public ReplayBlock gRPC query: a call made while every slot is
// occupied must give up as soon as its own context is done instead of
// waiting forever, and a call made once a slot is free must proceed.
func (suite *KeeperTestSuite) TestReplayBlockConcurrencyLimit() {
	req := &types.ReplayBlockRequest{
		BlockNumber: 1,
		BlockTime:   suite.ctx.BlockTime(),
	}

	// Saturate every concurrency slot so a further call has to wait.
	for i := 0; i < cronoskeeper.ReplayBlockConcurrencyLimit; i++ {
		cronoskeeper.ReplayBlockSemaphore <- struct{}{}
	}

	callCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := suite.app.CronosKeeper.ReplayBlock(suite.ctx.WithContext(callCtx), req)

	// Free the slots before asserting, so a failure doesn't wedge later tests.
	for i := 0; i < cronoskeeper.ReplayBlockConcurrencyLimit; i++ {
		<-cronoskeeper.ReplayBlockSemaphore
	}
	cronoskeeper.ResetReplayBlockQueued()

	suite.Require().Error(err)
	suite.Require().Equal(codes.DeadlineExceeded, status.Code(err),
		"call should give up on its own context deadline rather than an unrelated error")

	// With a free slot, the same request must proceed rather than being
	// mistaken for a queued/rejected caller.
	_, err = suite.app.CronosKeeper.ReplayBlock(suite.ctx, req)
	suite.Require().NoError(err)
}

// TestReplayBlockRejectsWhenQueueFull verifies the waiter bound: once
// replayBlockMaxQueued callers are already running or queued, a further
// call is rejected with ResourceExhausted instead of queuing indefinitely,
// which is what stops an unauthenticated caller from piling up unbounded
// decoded requests in memory behind the semaphore.
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
	suite.Require().NoError(err, "a call within the queue bound must still be admitted")
}

// TestReplayBlockAbortsOnCancelledContext verifies that a cancelled request
// context aborts the per-message replay loop instead of running the whole
// batch: with the context already cancelled before the call, the first
// iteration must fail with a context error rather than attempting message
// execution.
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
