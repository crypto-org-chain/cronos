package keeper

import (
	"context"
	"fmt"
	"sync"

	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

// FinalityCache provides in-memory caching for recent finality info
type FinalityCache struct {
	mu       sync.RWMutex
	statuses map[uint64]*types.FinalityStatus
	maxSize  int
}

// NewFinalityCache creates a new finality cache
func NewFinalityCache(maxSize int) *FinalityCache {
	return &FinalityCache{
		statuses: make(map[uint64]*types.FinalityStatus),
		maxSize:  maxSize,
	}
}

// Set stores a finality status in the cache
func (c *FinalityCache) Set(height uint64, status *types.FinalityStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entry if cache is full
	if len(c.statuses) >= c.maxSize {
		var minHeight uint64 = ^uint64(0)
		for h := range c.statuses {
			if h < minHeight {
				minHeight = h
			}
		}
		delete(c.statuses, minHeight)
	}

	c.statuses[height] = status
}

// Get retrieves a finality status from the cache
func (c *FinalityCache) Get(height uint64) (*types.FinalityStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status, ok := c.statuses[height]
	return status, ok
}

// Clear empties the cache
func (c *FinalityCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.statuses = make(map[uint64]*types.FinalityStatus)
}

// SetFinalityDB sets the local finality database (call after keeper creation)
func (k *Keeper) SetFinalityDB(db dbm.DB) {
	k.finalityDB = db
}

// SetFinalityCache sets the finality cache (call after keeper creation)
func (k *Keeper) SetFinalityCache(cache *FinalityCache) {
	k.finalityCache = cache
}

// MarkBlockFinalizedLocal stores finality in LOCAL database (no consensus)
// This method writes to a local database that does NOT participate in consensus.
// Each validator node maintains its own copy.
// Also updates the highest finality height in consensus state if this block is higher.
func (k Keeper) MarkBlockFinalizedLocal(ctx context.Context, height uint64, finalizedAt int64, proof []byte) error {
	status := &types.FinalityStatus{
		BlockHeight:   height,
		FinalizedAt:   finalizedAt,
		FinalityProof: proof,
	}

	// 1. Store in memory cache (if available)
	if k.finalityCache != nil {
		k.finalityCache.Set(height, status)
	}

	// 2. Store in local database (if available)
	if k.finalityDB != nil {
		key := types.GetFinalizedBlockKey(height)
		bz := k.cdc.MustMarshal(status)

		if err := k.finalityDB.Set(key, bz); err != nil {
			return fmt.Errorf("failed to store finality in local DB: %w", err)
		}
	}

	// 3. Update highest finality height in consensus state if this is higher
	highestHeight, err := k.GetHighestFinalityHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get highest finality height: %w", err)
	}
	if height > highestHeight {
		if err := k.SetHighestFinalityHeight(ctx, height); err != nil {
			return fmt.Errorf("failed to set highest finality height: %w", err)
		}
	}

	// 4. Emit event (this IS part of consensus, but minimal overhead)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"block_finalized_local",
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", height)),
			sdk.NewAttribute("finalized_at", fmt.Sprintf("%d", finalizedAt)),
			sdk.NewAttribute("has_proof", fmt.Sprintf("%v", len(proof) > 0)),
		),
	)

	k.Logger(ctx).Debug("Stored finality locally (no consensus)",
		"height", height,
		"finalized_at", finalizedAt,
	)

	return nil
}

// GetFinalityStatusLocal retrieves finality from LOCAL storage (no consensus)
// Uses tiered storage: memory cache -> local DB -> not found
func (k Keeper) GetFinalityStatusLocal(ctx context.Context, height uint64) (*types.FinalityStatus, error) {
	// 1. Check memory cache first (fastest)
	if k.finalityCache != nil {
		if status, ok := k.finalityCache.Get(height); ok {
			k.Logger(ctx).Debug("Finality status found in cache",
				"height", height,
			)
			return status, nil
		}
	}

	// 2. Check local database
	if k.finalityDB != nil {
		key := types.GetFinalizedBlockKey(height)
		bz, err := k.finalityDB.Get(key)
		if err != nil {
			return nil, fmt.Errorf("failed to get finality from local DB: %w", err)
		}

		if bz != nil {
			var status types.FinalityStatus
			k.cdc.MustUnmarshal(bz, &status)

			// Populate cache for next time
			if k.finalityCache != nil {
				k.finalityCache.Set(height, &status)
			}

			k.Logger(ctx).Debug("Finality status found in local DB",
				"height", height,
			)
			return &status, nil
		}
	}

	// 3. Not found - return status with FinalizedAt = 0 (not finalized)
	return &types.FinalityStatus{
		BlockHeight: height,
		FinalizedAt: 0,
	}, nil
}

// ListFinalizedBlocksLocal lists finalized blocks from LOCAL database
func (k Keeper) ListFinalizedBlocksLocal(ctx context.Context, startHeight, endHeight uint64) ([]*types.FinalityStatus, error) {
	if k.finalityDB == nil {
		return nil, fmt.Errorf("local finality DB not initialized")
	}

	var statuses []*types.FinalityStatus

	startKey := types.GetFinalizedBlockKey(startHeight)
	endKey := types.GetFinalizedBlockKey(endHeight + 1)

	itr, err := k.finalityDB.Iterator(startKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer itr.Close()

	for ; itr.Valid(); itr.Next() {
		var status types.FinalityStatus
		k.cdc.MustUnmarshal(itr.Value(), &status)
		statuses = append(statuses, &status)
	}

	k.Logger(ctx).Debug("Listed finalized blocks from local DB",
		"start_height", startHeight,
		"end_height", endHeight,
		"count", len(statuses),
	)

	return statuses, nil
}

// GetLatestFinalizedLocal returns the latest finalized block height from LOCAL storage
func (k Keeper) GetLatestFinalizedLocal(ctx context.Context) (uint64, error) {
	if k.finalityDB == nil {
		return 0, fmt.Errorf("local finality DB not initialized")
	}

	// Get iterator in reverse order
	prefix := types.FinalizedBlocksPrefix
	itr, err := k.finalityDB.ReverseIterator(prefix, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create reverse iterator: %w", err)
	}
	defer itr.Close()

	if itr.Valid() {
		var status types.FinalityStatus
		k.cdc.MustUnmarshal(itr.Value(), &status)
		return status.BlockHeight, nil
	}

	return 0, nil // No finalized blocks
}

// PruneFinalizedBlocksLocal prunes old finalized blocks from LOCAL storage
// This is safe because local storage doesn't affect consensus
func (k Keeper) PruneFinalizedBlocksLocal(ctx context.Context, beforeHeight uint64) (int, error) {
	if k.finalityDB == nil {
		return 0, fmt.Errorf("local finality DB not initialized")
	}

	startKey := types.FinalizedBlocksPrefix
	endKey := types.GetFinalizedBlockKey(beforeHeight)

	itr, err := k.finalityDB.Iterator(startKey, endKey)
	if err != nil {
		return 0, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer itr.Close()

	count := 0
	for ; itr.Valid(); itr.Next() {
		if err := k.finalityDB.Delete(itr.Key()); err != nil {
			return count, fmt.Errorf("failed to delete key: %w", err)
		}
		count++
	}

	k.Logger(ctx).Info("Pruned finalized blocks from local DB",
		"before_height", beforeHeight,
		"pruned_count", count,
	)

	return count, nil
}

// GetFinalityStatsLocal returns statistics about local finality storage
func (k Keeper) GetFinalityStatsLocal(ctx context.Context) (*types.FinalityStats, error) {
	if k.finalityDB == nil {
		return nil, fmt.Errorf("local finality DB not initialized")
	}

	stats := &types.FinalityStats{}

	// Count total finalized blocks
	itr, err := k.finalityDB.Iterator(types.FinalizedBlocksPrefix, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer itr.Close()

	var minHeight, maxHeight uint64
	for ; itr.Valid(); itr.Next() {
		var status types.FinalityStatus
		k.cdc.MustUnmarshal(itr.Value(), &status)

		stats.TotalFinalized++
		if minHeight == 0 || status.BlockHeight < minHeight {
			minHeight = status.BlockHeight
		}
		if status.BlockHeight > maxHeight {
			maxHeight = status.BlockHeight
		}
	}

	stats.MinHeight = minHeight
	stats.MaxHeight = maxHeight

	// Cache stats
	if k.finalityCache != nil {
		stats.CacheSize = uint64(len(k.finalityCache.statuses))
		stats.CacheMaxSize = uint64(k.finalityCache.maxSize)
	}

	return stats, nil
}

// CloseFinalityDB closes the local finality database
func (k Keeper) CloseFinalityDB() error {
	if k.finalityDB != nil {
		return k.finalityDB.Close()
	}
	return nil
}
