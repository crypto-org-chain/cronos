package keeper

import (
	"context"
	"fmt"
	"sync"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channelkeeper "github.com/cosmos/ibc-go/v10/modules/core/04-channel/keeper"
	channelkeeperv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/keeper"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService

	// Chain ID of the Cronos chain
	chainID string

	// Authority address for signing IBC v2 packets (typically gov module address)
	authority string

	// IBC version to use ("v1" or "v2")
	ibcVersion string

	// IBC v1 channel keeper for sending packets (traditional port/channel)
	channelKeeper *channelkeeper.Keeper

	// IBC v2 channel keeper for sending packets (client-to-client)
	channelKeeperV2 *channelkeeperv2.Keeper

	// Local non-consensus storage
	// These fields store finality data WITHOUT affecting consensus
	finalityDB    dbm.DB         // Local database (persistent, no consensus)
	finalityCache *FinalityCache // Memory cache (fast, no consensus)

	// RPC client for fetching block data (lazy initialization)
	rpcAddress  string           // RPC address for lazy client creation
	rpcClient   rpcclient.Client // Lazily initialized RPC client
	rpcClientMu sync.RWMutex
}

// NewKeeper creates a new attestation Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	chainID string,
	authority string,
	ibcVersion string,
) *Keeper {
	// Default to v2 if not specified or invalid
	if ibcVersion != "v1" && ibcVersion != "v2" {
		ibcVersion = "v2"
	}
	return &Keeper{
		cdc:          cdc,
		storeService: storeService,
		chainID:      chainID,
		authority:    authority,
		ibcVersion:   ibcVersion,
	}
}

// GetAuthority returns the authority address for the attestation module
func (k *Keeper) GetAuthority() string {
	return k.authority
}

// GetIBCVersion returns the configured IBC version ("v1" or "v2")
func (k *Keeper) GetIBCVersion() string {
	return k.ibcVersion
}

// InitializeLocalStorage sets up the local finality storage
// This should be called after NewKeeper to enable non-consensus finality storage
func (k *Keeper) InitializeLocalStorage(dbPath string, cacheSize int, backend dbm.BackendType) error {
	// Create local database with specified backend
	db, err := dbm.NewDB("finality", backend, dbPath)
	if err != nil {
		return err
	}
	k.finalityDB = db

	// Create memory cache
	if backend == dbm.MemDBBackend {
		k.finalityCache = nil
	} else {
		k.finalityCache = NewFinalityCache(cacheSize)
	}

	return nil
}

// SetRPCAddress sets the CometBFT RPC address for lazy client initialization
func (k *Keeper) SetRPCAddress(address string) {
	k.rpcClientMu.Lock()
	defer k.rpcClientMu.Unlock()
	k.rpcAddress = address
}

// ensureRPCClient lazily initializes the RPC client on first use
// Must be called with rpcClientMu held (at least read lock, will upgrade to write if needed)
func (k *Keeper) ensureRPCClient() (rpcclient.Client, error) {
	// Fast path: client already initialized
	k.rpcClientMu.RLock()
	if k.rpcClient != nil {
		client := k.rpcClient
		k.rpcClientMu.RUnlock()
		return client, nil
	}
	rpcAddress := k.rpcAddress
	k.rpcClientMu.RUnlock()

	if rpcAddress == "" {
		return nil, fmt.Errorf("RPC address not configured")
	}

	// Slow path: need to initialize client
	k.rpcClientMu.Lock()
	defer k.rpcClientMu.Unlock()

	// Double-check after acquiring write lock
	if k.rpcClient != nil {
		return k.rpcClient, nil
	}

	// Create and start the RPC client
	client, err := rpchttp.New(rpcAddress, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	if err := client.Start(); err != nil {
		_ = client.Stop() // Clean up partial state
		return nil, fmt.Errorf("failed to start RPC client: %w", err)
	}

	k.rpcClient = client
	return client, nil
}

// GetBlockDataRange fetches block attestation data for a range of heights via RPC
func (k *Keeper) GetBlockDataRange(ctx context.Context, startHeight, endHeight uint64) ([]*types.BlockAttestationData, error) {
	client, err := k.ensureRPCClient()
	if err != nil {
		return nil, err
	}

	// Use BlockchainInfo to fetch multiple headers in one RPC call
	blockchainInfo, err := client.BlockchainInfo(ctx, int64(startHeight), int64(endHeight))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blockchain info for range %d-%d: %w", startHeight, endHeight, err)
	}

	if len(blockchainInfo.BlockMetas) == 0 {
		return nil, fmt.Errorf("no block data found in range %d-%d", startHeight, endHeight)
	}

	result := make([]*types.BlockAttestationData, 0, len(blockchainInfo.BlockMetas))
	for _, meta := range blockchainInfo.BlockMetas {
		result = append(result, &types.BlockAttestationData{
			BlockHeight: uint64(meta.Header.Height),
			AppHash:     meta.Header.AppHash,
		})
	}

	return result, nil
}

// HasRPCClient returns true if the RPC address is configured (client will be lazily created)
func (k *Keeper) HasRPCClient() bool {
	k.rpcClientMu.RLock()
	defer k.rpcClientMu.RUnlock()
	return k.rpcAddress != ""
}

// StopRPCClient stops the RPC client if it's running
func (k *Keeper) StopRPCClient() error {
	k.rpcClientMu.Lock()
	defer k.rpcClientMu.Unlock()

	if k.rpcClient == nil {
		return nil
	}

	if err := k.rpcClient.Stop(); err != nil {
		return fmt.Errorf("failed to stop RPC client: %w", err)
	}
	k.rpcClient = nil
	return nil
}

// SetChannelKeeper sets the IBC v1 channel keeper for sending packets
// This is called after IBCKeeper initialization to avoid circular dependencies
func (k *Keeper) SetChannelKeeper(channelKeeper *channelkeeper.Keeper) {
	k.channelKeeper = channelKeeper
}

// SetChannelKeeperV2 sets the IBC v2 channel keeper for sending packets
// This is called after IBCKeeper initialization to avoid circular dependencies
func (k *Keeper) SetChannelKeeperV2(channelKeeperV2 *channelkeeperv2.Keeper) {
	k.channelKeeperV2 = channelKeeperV2
}

// Logger returns a module-specific logger
func (k *Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// ChainID returns the chain ID
func (k *Keeper) ChainID() string {
	return k.chainID
}

// GetParams returns the module parameters
func (k *Keeper) GetParams(ctx context.Context) (types.Params, error) {
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
func (k *Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}

	store := k.storeService.OpenKVStore(ctx)
	bz := k.cdc.MustMarshal(&params)
	return store.Set(types.ParamsKey, bz)
}

// GetLastSentHeight retrieves the last block height sent for attestation
func (k *Keeper) GetLastSentHeight(ctx context.Context) (uint64, error) {
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
func (k *Keeper) SetLastSentHeight(ctx context.Context, height uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.LastSentHeightKey, types.UintToBytes(height))
}

// AddPendingAttestation adds a block attestation to the pending queue (local storage)
// Pending attestations are tracked locally by each validator, not in consensus state
func (k *Keeper) AddPendingAttestation(ctx context.Context, height uint64, attestation *types.BlockAttestationData) error {
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
func (k *Keeper) GetPendingAttestation(ctx context.Context, height uint64) (*types.BlockAttestationData, error) {
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
func (k *Keeper) GetPendingAttestations(ctx context.Context, startHeight, endHeight uint64) ([]*types.BlockAttestationData, error) {
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
func (k *Keeper) RemovePendingAttestation(ctx context.Context, height uint64) error {
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
func (k *Keeper) GetHighestFinalityHeight(ctx context.Context) (uint64, error) {
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
func (k *Keeper) SetHighestFinalityHeight(ctx context.Context, height uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.HighestFinalityHeightKey, types.UintToBytes(height))
}
