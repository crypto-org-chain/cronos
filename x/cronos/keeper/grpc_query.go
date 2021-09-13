package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
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
