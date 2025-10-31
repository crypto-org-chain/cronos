package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
)

// finalityStore implements FinalityStore interface
type finalityStore struct {
	db     dbm.DB
	logger log.Logger
	mu     sync.RWMutex

	// In-memory cache for faster access
	cache map[string]*FinalityInfo
}

// NewFinalityStore creates a new finality store
func NewFinalityStore(db dbm.DB, logger log.Logger) FinalityStore {
	return &finalityStore{
		db:     db,
		logger: logger,
		cache:  make(map[string]*FinalityInfo),
	}
}

// NewFinalityStoreFromConfig creates a finality store from configuration
func NewFinalityStoreFromConfig(config *Config, logger log.Logger) (FinalityStore, error) {
	var backendType dbm.BackendType

	switch config.FinalityStoreType {
	case "memory", "memdb":
		backendType = dbm.MemDBBackend
	case "leveldb", "goleveldb":
		backendType = dbm.GoLevelDBBackend
	case "rocksdb":
		backendType = dbm.RocksDBBackend
	default:
		return nil, fmt.Errorf("unsupported finality store type: %s", config.FinalityStoreType)
	}

	db, err := dbm.NewDB("finality", backendType, config.FinalityStorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create finality store database: %w", err)
	}

	return NewFinalityStore(db, logger), nil
}

// SaveFinalityInfo saves finality information
func (fs *finalityStore) SaveFinalityInfo(ctx context.Context, info *FinalityInfo) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	key := fs.makeKey(info.ChainID, info.BlockHeight)

	// Serialize to JSON
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal finality info: %w", err)
	}

	// Save to database
	if err := fs.db.Set(key, data); err != nil {
		return fmt.Errorf("failed to save finality info: %w", err)
	}

	// Update cache
	cacheKey := fmt.Sprintf("%s:%d", info.ChainID, info.BlockHeight)
	fs.cache[cacheKey] = info

	fs.logger.Debug("Saved finality info",
		"chain_id", info.ChainID,
		"block_height", info.BlockHeight,
		"finalized", info.Finalized,
	)

	return nil
}

// GetFinalityInfo retrieves finality information
func (fs *finalityStore) GetFinalityInfo(ctx context.Context, chainID string, height uint64) (*FinalityInfo, error) {
	fs.mu.RLock()

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%d", chainID, height)
	if cached, ok := fs.cache[cacheKey]; ok {
		fs.mu.RUnlock()
		return cached, nil
	}
	fs.mu.RUnlock()

	// Not in cache, fetch from database
	key := fs.makeKey(chainID, height)

	data, err := fs.db.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get finality info: %w", err)
	}

	if data == nil {
		return nil, nil // Not found
	}

	var info FinalityInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal finality info: %w", err)
	}

	// Update cache
	fs.mu.Lock()
	fs.cache[cacheKey] = &info
	fs.mu.Unlock()

	return &info, nil
}

// GetLatestFinalized returns the latest finalized block height for a chain
func (fs *finalityStore) GetLatestFinalized(ctx context.Context, chainID string) (uint64, error) {
	// Scan database for latest finalized block
	prefix := []byte(fmt.Sprintf("finality:%s:", chainID))

	itr, err := fs.db.ReverseIterator(prefix, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer itr.Close()

	for ; itr.Valid(); itr.Next() {
		var info FinalityInfo
		if err := json.Unmarshal(itr.Value(), &info); err != nil {
			continue
		}

		if info.Finalized {
			return info.BlockHeight, nil
		}
	}

	return 0, nil // No finalized blocks found
}

// ListPendingFinality lists blocks pending finality
func (fs *finalityStore) ListPendingFinality(ctx context.Context, chainID string, limit int) ([]*FinalityInfo, error) {
	prefix := []byte(fmt.Sprintf("finality:%s:", chainID))

	itr, err := fs.db.Iterator(prefix, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer itr.Close()

	var pending []*FinalityInfo

	for ; itr.Valid(); itr.Next() {
		var info FinalityInfo
		if err := json.Unmarshal(itr.Value(), &info); err != nil {
			continue
		}

		if !info.Finalized {
			pending = append(pending, &info)

			if limit > 0 && len(pending) >= limit {
				break
			}
		}
	}

	// Sort by block height
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].BlockHeight < pending[j].BlockHeight
	})

	return pending, nil
}

// Close closes the finality store
func (fs *finalityStore) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Clear cache
	fs.cache = make(map[string]*FinalityInfo)

	// Close database
	if fs.db != nil {
		return fs.db.Close()
	}

	return nil
}

// makeKey creates a database key for finality info
func (fs *finalityStore) makeKey(chainID string, height uint64) []byte {
	return []byte(fmt.Sprintf("finality:%s:%020d", chainID, height))
}

// GetStats returns statistics about the finality store
func (fs *finalityStore) GetStats(ctx context.Context, chainID string) (*FinalityStoreStats, error) {
	prefix := []byte(fmt.Sprintf("finality:%s:", chainID))

	itr, err := fs.db.Iterator(prefix, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator: %w", err)
	}
	defer itr.Close()

	stats := &FinalityStoreStats{
		ChainID: chainID,
	}

	for ; itr.Valid(); itr.Next() {
		var info FinalityInfo
		if err := json.Unmarshal(itr.Value(), &info); err != nil {
			continue
		}

		stats.TotalBlocks++

		if info.Finalized {
			stats.FinalizedBlocks++
			if info.BlockHeight > stats.LatestFinalized {
				stats.LatestFinalized = info.BlockHeight
			}
		} else {
			stats.PendingBlocks++
		}

		if info.BlockHeight > stats.LatestBlock {
			stats.LatestBlock = info.BlockHeight
		}
	}

	return stats, nil
}

// FinalityStoreStats contains statistics about the finality store
type FinalityStoreStats struct {
	ChainID         string `json:"chain_id"`
	TotalBlocks     uint64 `json:"total_blocks"`
	FinalizedBlocks uint64 `json:"finalized_blocks"`
	PendingBlocks   uint64 `json:"pending_blocks"`
	LatestBlock     uint64 `json:"latest_block"`
	LatestFinalized uint64 `json:"latest_finalized"`
}
