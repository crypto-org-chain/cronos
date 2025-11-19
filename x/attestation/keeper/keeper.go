package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService

	channelKeeper types.ChannelKeeper

	// Chain ID of the Cronos chain
	chainID string
}

// NewKeeper creates a new attestation Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	channelKeeper types.ChannelKeeper,
	chainID string,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		channelKeeper: channelKeeper,
		chainID:       chainID,
	}
}

// Logger returns a module-specific logger
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetPort returns the portID for the module. Used in ExportGenesis
func (k Keeper) GetPort(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	bz, _ := store.Get(types.ParamsKey)
	if bz == nil {
		return types.PortID
	}

	var params types.Params
	k.cdc.MustUnmarshal(bz, &params)
	return params.PortId
}

// SetChannelID stores the IBC channel ID for attestation
func (k Keeper) SetChannelID(ctx context.Context, channelID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.IBCChannelKey, []byte(channelID))
}

// GetChannelID retrieves the IBC channel ID for attestation
func (k Keeper) GetChannelID(ctx context.Context) (string, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.IBCChannelKey)
	if err != nil {
		return "", err
	}
	if bz == nil {
		return "", types.ErrChannelNotFound
	}
	return string(bz), nil
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

// SendAttestationPacket sends block attestations via IBC to the attestation chain
func (k Keeper) SendAttestationPacket(
	ctx context.Context,
	attestations []*types.BlockAttestationData,
	relayer string,
	signature []byte,
	nonce uint64,
) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	if !params.AttestationEnabled {
		return types.ErrAttestationDisabled
	}

	channelID, err := k.GetChannelID(ctx)
	if err != nil {
		return err
	}

	// Convert pointer slice to value slice for proto
	attestationValues := make([]types.BlockAttestationData, len(attestations))
	for i, att := range attestations {
		attestationValues[i] = *att
	}

	// Create packet data
	packetData := &types.AttestationPacketData{
		Type:          types.AttestationPacketTypeBatchBlock,
		SourceChainId: k.chainID,
		Attestations:  attestationValues,
		Relayer:       relayer,
		Signature:     signature,
		Nonce:         nonce,
	}

	// Marshal packet data
	dataBytes, err := k.cdc.Marshal(packetData)
	if err != nil {
		return fmt.Errorf("failed to marshal packet data: %w", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Calculate timeout timestamp (current time + timeout duration)
	timeoutTimestamp := uint64(sdkCtx.BlockTime().UnixNano()) + params.PacketTimeoutTimestamp

	// Send packet (use ZeroHeight() for timeout height in v10)
	sequence, err := k.channelKeeper.SendPacket(
		sdkCtx,
		k.GetPort(ctx),
		channelID,
		clienttypes.ZeroHeight(), // Use zero height for timeout height (rely on timestamp)
		timeoutTimestamp,
		dataBytes,
	)

	if err != nil {
		return types.ErrFailedToSendPacket.Wrapf("sequence: %d, error: %s", sequence, err.Error())
	}

	k.Logger(ctx).Info("sent attestation packet",
		"sequence", sequence,
		"channel", channelID,
		"blocks", len(attestations),
		"relayer", relayer,
	)

	return nil
}
