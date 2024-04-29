package keeper

import (
	"context"
	"fmt"
	"math/big"

	errorsmod "cosmossdk.io/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	evmkeeper "github.com/evmos/ethermint/x/evm/keeper"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

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
	rsps := make([]*evmtypes.MsgEthereumTxResponse, 0, len(req.Msgs))

	// prepare the block context, the multistore version should be setup already in grpc query context.
	ctx := sdk.UnwrapSDKContext(goCtx).
		WithBlockHeight(req.BlockNumber).
		WithBlockTime(req.BlockTime).
		WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	// load parameters
	params := k.evmKeeper.GetParams(ctx)
	chainID := k.evmKeeper.ChainID()
	// the chain_id is irrelevant here
	ethCfg := params.ChainConfig.EthereumConfig(chainID)

	blockHeight := big.NewInt(req.BlockNumber)
	homestead := ethCfg.IsHomestead(blockHeight)
	istanbul := ethCfg.IsIstanbul(blockHeight)
	shanghai := ethCfg.IsShanghai(uint64(req.BlockTime.Unix()))
	evmDenom := params.EvmDenom
	baseFee := k.evmKeeper.GetBaseFee(ctx, ethCfg)

	// we assume the message executions are successful, they are filtered in json-rpc api
	for _, msg := range req.Msgs {
		// deduct fee
		// populate the `From` field
		if _, err := msg.GetSenderLegacy(ethtypes.LatestSignerForChainID(chainID)); err != nil {
			return nil, err
		}
		fees, err := evmkeeper.VerifyFee(msg, evmDenom, baseFee, homestead, istanbul, shanghai, ctx.IsCheckTx())
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
