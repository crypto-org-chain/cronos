package keeper

import (
	"context"

	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

var _ types.QueryServer = (*Keeper)(nil)

// Params returns the module parameters
func (k Keeper) Params(c context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := k.GetParams(c)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

// GetBlockAttestation returns block attestation data
func (k Keeper) GetBlockAttestation(c context.Context, req *types.QueryGetBlockAttestationRequest) (*types.QueryGetBlockAttestationResponse, error) {
	// TODO: Implement when attestation storage is added
	return &types.QueryGetBlockAttestationResponse{}, nil
}

// GetBlockFinalityStatus returns finality status for a block
func (k Keeper) GetBlockFinalityStatus(c context.Context, req *types.QueryGetBlockFinalityStatusRequest) (*types.QueryGetBlockFinalityStatusResponse, error) {
	// Query from local storage
	status, err := k.GetFinalityStatusLocal(c, req.BlockHeight)
	if err != nil {
		// Not found or error
		return &types.QueryGetBlockFinalityStatusResponse{
			Finalized: false,
		}, nil
	}

	return &types.QueryGetBlockFinalityStatusResponse{
		Finalized:     status.Finalized,
		FinalizedAt:   status.FinalizedAt,
		FinalityProof: status.FinalityProof,
	}, nil
}

// GetPendingForcedTxs returns pending forced transactions
func (k Keeper) GetPendingForcedTxs(c context.Context, req *types.QueryGetPendingForcedTxsRequest) (*types.QueryGetPendingForcedTxsResponse, error) {
	// TODO: Implement when forced tx functionality is added
	return &types.QueryGetPendingForcedTxsResponse{}, nil
}

// GetForcedTx returns a specific forced transaction
func (k Keeper) GetForcedTx(c context.Context, req *types.QueryGetForcedTxRequest) (*types.QueryGetForcedTxResponse, error) {
	// TODO: Implement when forced tx functionality is added
	return &types.QueryGetForcedTxResponse{}, nil
}
