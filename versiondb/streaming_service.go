package versiondb

import (
	"context"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/store/types"
)

var _ types.ABCIListener = &StreamingService{}

// StreamingService is a concrete implementation of StreamingService that accumulate the state changes in current block,
// writes the ordered changeset out to version storage.
type StreamingService struct {
	versionStore       VersionStore
	currentBlockNumber int64 // the current block number
}

// NewStreamingService creates a new StreamingService for the provided writeDir, (optional) filePrefix, and storeKeys
func NewStreamingService(versionStore VersionStore) *StreamingService {
	return &StreamingService{versionStore: versionStore}
}

// ListenFinalizeBlock satisfies the types.ABCIListener interface
func (fss *StreamingService) ListenFinalizeBlock(ctx context.Context, req abci.RequestFinalizeBlock, res abci.ResponseFinalizeBlock) error {
	fss.currentBlockNumber = req.Height
	return nil
}

func (fss *StreamingService) ListenCommit(ctx context.Context, res abci.ResponseCommit, changeSet []*types.StoreKVPair) error {
	accStoreKey := "acc"
	total := 0
	for _, kv := range changeSet {
		if kv.StoreKey == accStoreKey {
			total += 1
		}
	}
	panic(fmt.Errorf("%s total: %d\n", accStoreKey, total))
	return fss.versionStore.PutAtVersion(fss.currentBlockNumber, changeSet)
}
