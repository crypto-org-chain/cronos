package mempool

import (
	"sync"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

// mempoolState holds the CacheMultiStore branch that admission and recheck
// share as the sole nonce authority (docs/architecture/mempool-branched-recheck-context.md).
// mu guards base independently of txExec.mu: it is always the innermost lock
// (mempoolState never calls back out while holding it), so nesting it under
// txExec.mu adds no ordering hazard, and base stays safe to read even if a
// future caller forgets to hold txExec.mu.
type mempoolState struct {
	mu       sync.RWMutex
	base     storetypes.CacheMultiStore
	provider func() storetypes.CommitMultiStore
}

// refreshLocked branches a fresh base off the committed store. Precondition:
// the caller holds txExec.mu, which is what actually keeps this swap
// from racing a concurrent RunTx or the live memiavl tree mid-Commit.
func (s *mempoolState) refreshLocked() {
	base := s.provider().CacheMultiStore()
	s.mu.Lock()
	s.base = base
	s.mu.Unlock()
}

// store returns the current base, or nil so RunTx falls back to checkState.
// Nil-safe on a nil receiver (and nil base) so the newManager() test
// constructor, which leaves txExec.state nil, keeps working without a store.
func (s *mempoolState) store() storetypes.MultiStore {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.base == nil {
		return nil
	}
	return s.base
}
