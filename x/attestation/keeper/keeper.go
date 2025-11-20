package keeper

import (
	"context"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService

	// Chain ID of the Cronos chain
	chainID string
}

// NewKeeper creates a new attestation Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	chainID string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		chainID:      chainID,
	}
}

// Logger returns a module-specific logger
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// IBC v1 port/channel methods removed - module uses IBC v2 (Eureka) exclusively

// GetParams returns the module parameters
func (k Keeper) GetParams(ctx context.Context) (types.Params, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil {
		return types.Params{}, err
	}

	if bz == nil {
		// Return default params if not set
		return types.DefaultParams(), nil
	}

	var params types.Params
	k.cdc.MustUnmarshal(bz, &params)
	return params, nil
}

// SetParams sets the module parameters
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&params)
	return store.Set(types.ParamsKey, bz)
}

// GetLastSentHeight retrieves the last block height sent for attestation
func (k Keeper) GetLastSentHeight(ctx context.Context) (uint64, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.LastSentHeightKey)
	if err != nil {
		return 0, err
	}
	if bz == nil {
		return 0, nil
	}
	return types.BytesToUint(bz), nil
}

// SetLastSentHeight stores the last block height sent for attestation
func (k Keeper) SetLastSentHeight(ctx context.Context, height uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.LastSentHeightKey, types.UintToBytes(height))
}

// AddPendingAttestation adds a block attestation to the pending queue
func (k Keeper) AddPendingAttestation(ctx context.Context, height uint64, attestation *types.BlockAttestationData) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetPendingAttestationKey(height)
	bz := k.cdc.MustMarshal(attestation)
	return store.Set(key, bz)
}

// GetPendingAttestation retrieves a pending attestation by height
func (k Keeper) GetPendingAttestation(ctx context.Context, height uint64) (*types.BlockAttestationData, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetPendingAttestationKey(height)
	bz, err := store.Get(key)
	if err != nil {
		return nil, err
	}
	if bz == nil {
		return nil, types.ErrAttestationNotFound
	}

	var attestation types.BlockAttestationData
	k.cdc.MustUnmarshal(bz, &attestation)
	return &attestation, nil
}

// GetPendingAttestations retrieves all pending attestations in a height range
func (k Keeper) GetPendingAttestations(ctx context.Context, startHeight, endHeight uint64) ([]*types.BlockAttestationData, error) {
	var attestations []*types.BlockAttestationData

	for height := startHeight; height <= endHeight; height++ {
		attestation, err := k.GetPendingAttestation(ctx, height)
		if err != nil {
			if err == types.ErrAttestationNotFound {
				continue // Skip missing attestations
			}
			return nil, err
		}
		attestations = append(attestations, attestation)
	}

	return attestations, nil
}

// RemovePendingAttestation removes a pending attestation by height
func (k Keeper) RemovePendingAttestation(ctx context.Context, height uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetPendingAttestationKey(height)
	return store.Delete(key)
}

// MarkBlockFinalized marks a block as finalized on the attestation layer
func (k Keeper) MarkBlockFinalized(ctx context.Context, height uint64, finalizedAt int64, proof []byte) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetFinalizedBlockKey(height)

	status := &types.FinalityStatus{
		BlockHeight:   height,
		Finalized:     true,
		FinalizedAt:   finalizedAt,
		FinalityProof: proof,
	}

	bz := k.cdc.MustMarshal(status)
	return store.Set(key, bz)
}

// GetFinalityStatus retrieves the finality status for a block
func (k Keeper) GetFinalityStatus(ctx context.Context, height uint64) (*types.FinalityStatus, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetFinalizedBlockKey(height)
	bz, err := store.Get(key)
	if err != nil {
		return nil, err
	}

	if bz == nil {
		return &types.FinalityStatus{
			BlockHeight: height,
			Finalized:   false,
		}, nil
	}

	var status types.FinalityStatus
	k.cdc.MustUnmarshal(bz, &status)
	return &status, nil
}

// IBC v1 SendAttestationPacket removed - use SendAttestationPacketV2 (in v2_sender.go)
