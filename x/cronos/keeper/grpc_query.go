package keeper

import (
	"context"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	evmkeeper "github.com/evmos/ethermint/x/evm/keeper"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// MaxReplayBlockMsgs caps the eth messages one ReplayBlock query may execute.
	MaxReplayBlockMsgs = 10000

	// ReplayBlockGasCap caps per-message EVM gas in a ReplayBlock query.
	// Since historical blocks may have used different limits, we use a fixed upper bound value.
	ReplayBlockGasCap = 60_000_000

	// replayBlockConcurrency bounds how many ReplayBlock queries may run their
	// EVM replay loop at once. The per-call caps above bound a single call, not
	// aggregate load: this endpoint is unauthenticated, and each call can burn
	// up to gasBudget (2x ReplayBlockGasCap) of EVM compute, so a handful of
	// concurrent callers could otherwise saturate the node's execution
	// capacity and starve legitimate queries.
	replayBlockConcurrency = 4

	// replayBlockMaxQueued bounds how many callers may wait for a free slot.
	// Waiting is unbounded compute (goroutines only block), but each waiter
	// keeps its decoded request - up to MaxReplayBlockMsgs eth messages - live
	// in memory for as long as it queues, so an unauthenticated caller could
	// otherwise pile up unbounded memory just by opening many streams and
	// never completing. Once this many callers are already running or
	// queued, further calls are rejected instead of queuing.
	replayBlockMaxQueued = 4 * replayBlockConcurrency
)

// replayBlockSem limits concurrent ReplayBlock executions across all calls to
// this process. It is package-level rather than a Keeper field so it doesn't
// change how the Keeper is constructed; the endpoint is the only caller.
var replayBlockSem = make(chan struct{}, replayBlockConcurrency)

// replayBlockQueued counts callers currently running or waiting for a slot,
// enforcing replayBlockMaxQueued.
var replayBlockQueued int32

var _ types.QueryServer = Keeper{}

// ContractByDenom query contract by denom, returns both external contract and auto deployed contract
func (k Keeper) ContractByDenom(goCtx context.Context, req *types.ContractByDenomRequest) (*types.ContractByDenomResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	rsp := types.ContractByDenomResponse{}
	contract, found := k.getExternalContractByDenom(ctx, req.Denom)
	if found {
		rsp.Contract = contract.String()
	}
	autoContract, found := k.getAutoContractByDenom(ctx, req.Denom)
	if found {
		rsp.AutoContract = autoContract.String()
	}
	if len(rsp.Contract) == 0 && len(rsp.AutoContract) == 0 {
		return nil, fmt.Errorf("contract for the coin denom %s is not found", req.Denom)
	}
	return &rsp, nil
}

// DenomByContract query denom by contract
func (k Keeper) DenomByContract(goCtx context.Context, req *types.DenomByContractRequest) (*types.DenomByContractResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	denom, found := k.GetDenomByContract(ctx, common.HexToAddress(req.Contract))
	if !found {
		return nil, fmt.Errorf("coin denom for contract %s is not found", req.Contract)
	}
	return &types.DenomByContractResponse{
		Denom: denom,
	}, nil
}

// ReplayBlock replay the eth messages in the block to recover the results of false-failed txs.
func (k Keeper) ReplayBlock(goCtx context.Context, req *types.ReplayBlockRequest) (*types.ReplayBlockResponse, error) {
	// bound the batch size on this public gRPC endpoint
	if len(req.Msgs) > MaxReplayBlockMsgs {
		return nil, status.Errorf(codes.InvalidArgument,
			"too many messages in ReplayBlock request: %d (max %d)", len(req.Msgs), MaxReplayBlockMsgs)
	}

	// Reject outright once too many callers are already running or queued,
	// so an attacker can't pile up unbounded waiters (each holding a decoded
	// request in memory) behind the semaphore below.
	if atomic.AddInt32(&replayBlockQueued, 1) > replayBlockMaxQueued {
		atomic.AddInt32(&replayBlockQueued, -1)
		return nil, status.Error(codes.ResourceExhausted, "too many concurrent ReplayBlock queries")
	}
	defer atomic.AddInt32(&replayBlockQueued, -1)

	// Wait for a free execution slot rather than rejecting outright: replay is
	// a legitimate (if heavy) debugging query, so a burst of concurrent
	// callers should queue instead of failing. A client disconnect (goCtx
	// cancelled) frees the caller without consuming a slot.
	select {
	case replayBlockSem <- struct{}{}:
		defer func() { <-replayBlockSem }()
	case <-goCtx.Done():
		return nil, status.FromContextError(goCtx.Err()).Err()
	}

	rsps := make([]*evmtypes.MsgEthereumTxResponse, 0, len(req.Msgs))

	// prepare the block context, the multistore version should be setup already in grpc query context.
	ctx := sdk.UnwrapSDKContext(goCtx).
		WithBlockHeight(req.BlockNumber).
		WithBlockTime(req.BlockTime).
		WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	// Per-message gas cap. A committed tx already fits within the block gas
	// limit, so legitimate replay is unaffected.
	gasCap := uint64(ReplayBlockGasCap)

	// load parameters
	params := k.evmKeeper.GetParams(ctx)
	chainID := k.evmKeeper.ChainID()
	// the chain_id is irrelevant here
	ethCfg := params.ChainConfig.EthereumConfig(chainID)

	blockHeight := big.NewInt(req.BlockNumber)
	blockTime := uint64(req.BlockTime.Unix())
	rules := ethCfg.Rules(blockHeight, ethCfg.MergeNetsplitBlock != nil, blockTime)

	evmDenom := params.EvmDenom
	baseFee := k.evmKeeper.GetBaseFee(ctx, ethCfg)

	// gas budget is 2 times the gas cap because the last transaction can be overload the gas limit in case of legacy blocks
	gasBudget := gasCap * 2
	var cumulativeGas uint64
	for _, msg := range req.Msgs {
		gas := msg.GetGas()
		if gas > gasCap {
			return nil, status.Errorf(codes.InvalidArgument,
				"message gas limit %d exceeds ReplayBlock cap %d", gas, gasCap)
		}
		if gas > gasBudget-cumulativeGas {
			return nil, status.Errorf(codes.InvalidArgument,
				"cumulative message gas exceeds ReplayBlock budget %d", gasBudget)
		}
		cumulativeGas += gas
	}

	// we assume the message executions are successful, they are filtered in json-rpc api
	for _, msg := range req.Msgs {
		// abort a long batch as soon as the caller goes away, instead of
		// burning the remaining gas budget on a query nobody is waiting for.
		if err := ctx.Err(); err != nil {
			return nil, status.FromContextError(err).Err()
		}

		// deduct fee
		// populate the `From` field
		if _, err := msg.GetSenderLegacy(ethtypes.LatestSignerForChainID(chainID)); err != nil {
			return nil, err
		}
		fees, err := evmkeeper.VerifyFee(msg, evmDenom, baseFee, rules, ctx.IsCheckTx())
		if err != nil {
			return nil, errorsmod.Wrapf(err, "failed to verify the fees")
		}
		if err := k.evmKeeper.DeductTxCostsFromUserBalance(ctx, fees, common.BytesToAddress(msg.From)); err != nil {
			return nil, err
		}

		// increase nonce
		acc := k.accountKeeper.GetAccount(ctx, msg.GetFrom())
		if acc == nil {
			return nil, fmt.Errorf("account not found %s", msg.From)
		}
		if err := acc.SetSequence(acc.GetSequence() + 1); err != nil {
			return nil, err
		}
		k.accountKeeper.SetAccount(ctx, acc)

		rsp, err := k.evmKeeper.EthereumTx(ctx, msg)
		if err != nil {
			return nil, err
		}
		rsps = append(rsps, rsp)
	}
	return &types.ReplayBlockResponse{
		Responses: rsps,
	}, nil
}

// Params returns parameters of cronos module
func (k Keeper) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	return &types.QueryParamsResponse{Params: params}, nil
}

// Permissions returns the permissions of a specific account
func (k Keeper) Permissions(goCtx context.Context, req *types.QueryPermissionsRequest) (*types.QueryPermissionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	acc, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}
	admin := k.GetParams(ctx).CronosAdmin
	if admin == acc.String() {
		return &types.QueryPermissionsResponse{
			CanChangeTokenMapping: true,
			CanTurnBridge:         true,
		}, nil
	}
	permissions := k.GetPermissions(ctx, acc)
	return &types.QueryPermissionsResponse{
		CanChangeTokenMapping: CanChangeTokenMapping == (permissions & CanChangeTokenMapping),
		CanTurnBridge:         CanTurnBridge == (permissions & CanTurnBridge),
	}, nil
}

func (k Keeper) BlockList(goCtx context.Context, req *types.QueryBlockListRequest) (*types.QueryBlockListResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	blob := ctx.KVStore(k.storeKey).Get(types.KeyPrefixBlockList)
	return &types.QueryBlockListResponse{
		Blob: blob,
	}, nil
}
