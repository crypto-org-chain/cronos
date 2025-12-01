package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService

	// Chain ID of the Cronos chain
	chainID string

	// Local non-consensus storage
	// These fields store finality data WITHOUT affecting consensus
	finalityDB    dbm.DB         // Local database (persistent, no consensus)
	finalityCache *FinalityCache // Memory cache (fast, no consensus)
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

// InitializeLocalStorage sets up the local finality storage
// This should be called after NewKeeper to enable non-consensus finality storage
func (k *Keeper) InitializeLocalStorage(dbPath string, cacheSize int, backend dbm.BackendType) error {
	// Create local database with specified backend
	db, err := dbm.NewDB("finality", backend, dbPath)
	if err != nil {
		return err
	}

	// Create memory cache
	cache := NewFinalityCache(cacheSize)

	k.finalityDB = db
	k.finalityCache = cache

	return nil
}

// Logger returns a module-specific logger
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// ChainID returns the chain ID
func (k Keeper) ChainID() string {
	return k.chainID
}

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

// AddPendingAttestation adds a block attestation to the pending queue (local storage)
// Pending attestations are tracked locally by each validator, not in consensus state
func (k Keeper) AddPendingAttestation(ctx context.Context, height uint64, attestation *types.BlockAttestationData) error {
	if k.finalityDB == nil {
		return fmt.Errorf("local finality database not initialized")
	}

	key := types.GetPendingAttestationKey(height)
	bz := k.cdc.MustMarshal(attestation)

	if err := k.finalityDB.Set(key, bz); err != nil {
		return fmt.Errorf("failed to add pending attestation: %w", err)
	}

	k.Logger(ctx).Debug("Added pending attestation to local storage",
		"height", height,
	)
	return nil
}

// GetPendingAttestation retrieves a pending attestation by height (from local storage)
func (k Keeper) GetPendingAttestation(ctx context.Context, height uint64) (*types.BlockAttestationData, error) {
	if k.finalityDB == nil {
		return nil, fmt.Errorf("local finality database not initialized")
	}

	key := types.GetPendingAttestationKey(height)
	bz, err := k.finalityDB.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending attestation: %w", err)
	}
	if bz == nil {
		return nil, types.ErrAttestationNotFound
	}

	var attestation types.BlockAttestationData
	k.cdc.MustUnmarshal(bz, &attestation)
	return &attestation, nil
}

// GetPendingAttestations retrieves all pending attestations in a height range (from local storage)
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

// RemovePendingAttestation removes a pending attestation by height (from local storage)
func (k Keeper) RemovePendingAttestation(ctx context.Context, height uint64) error {
	if k.finalityDB == nil {
		return fmt.Errorf("local finality database not initialized")
	}

	key := types.GetPendingAttestationKey(height)
	if err := k.finalityDB.Delete(key); err != nil {
		return fmt.Errorf("failed to remove pending attestation: %w", err)
	}

	k.Logger(ctx).Debug("Removed pending attestation from local storage",
		"height", height,
	)
	return nil
}

// GetHighestFinalityHeight retrieves the highest finalized block height from consensus state
func (k Keeper) GetHighestFinalityHeight(ctx context.Context) (uint64, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.HighestFinalityHeightKey)
	if err != nil {
		return 0, err
	}
	if bz == nil {
		return 0, nil
	}
	return types.BytesToUint(bz), nil
}

// SetHighestFinalityHeight stores the highest finalized block height in consensus state
func (k Keeper) SetHighestFinalityHeight(ctx context.Context, height uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.HighestFinalityHeightKey, types.UintToBytes(height))
}
