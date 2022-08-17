package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
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
	london := ethCfg.IsLondon(blockHeight)
	evmDenom := params.EvmDenom

	// we assume the message executions are successful, they are filtered in json-rpc api
	for _, msg := range req.Msgs {
		// deduct fee
		txData, err := evmtypes.UnpackTxData(msg.Data)
		if err != nil {
			return nil, err
		}

		// populate the `From` field
		if _, err := msg.GetSender(chainID); err != nil {
			return nil, err
		}

		if _, _, err := k.evmKeeper.DeductTxCostsFromUserBalance(
			ctx,
			*msg,
			txData,
			evmDenom,
			homestead,
			istanbul,
			london,
		); err != nil {
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

		rsp, err := k.evmKeeper.EthereumTx(sdk.WrapSDKContext(ctx), msg)
		if err != nil {
			return nil, err
		}
		rsps = append(rsps, rsp)
	}
	return &types.ReplayBlockResponse{
		Responses: rsps,
	}, nil
}
