package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

// SendAttestationPacketV1 sends block attestations via IBC v1 to the attestation chain
// This uses the traditional port/channel communication
func (k *Keeper) SendAttestationPacketV1(
	ctx context.Context,
	sourcePort string,
	sourceChannel string,
	attestations []*types.BlockAttestationData,
) (uint64, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 0, err
	}

	if !params.AttestationEnabled {
		return 0, types.ErrAttestationDisabled
	}

	// Check if channel keeper v1 is set
	if k.channelKeeper == nil {
		return 0, fmt.Errorf("IBC v1 channel keeper not initialized")
	}

	// Convert pointer slice to value slice for proto
	attestationValues := make([]types.BlockAttestationData, len(attestations))
	for i, att := range attestations {
		attestationValues[i] = *att
	}

	// Create packet data
	packetData := &types.AttestationPacketData{
		SourceChainId: k.chainID,
		Attestations:  attestationValues,
	}

	// Marshal packet data to JSON
	dataBytes, err := json.Marshal(packetData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal packet data: %w", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Calculate timeout
	timeoutTimestamp := uint64(sdkCtx.BlockTime().UnixNano()) + params.PacketTimeoutTimestamp

	// Use zero height for timeout (only timestamp-based timeout)
	timeoutHeight := clienttypes.ZeroHeight()

	// Verify the channel exists before sending
	channel, found := k.channelKeeper.GetChannel(sdkCtx, sourcePort, sourceChannel)
	if !found {
		k.Logger(ctx).Error("IBC v1 channel not found in channel keeper",
			"source_port", sourcePort,
			"source_channel", sourceChannel,
		)
		return 0, fmt.Errorf("channel %s not found on port %s", sourceChannel, sourcePort)
	}

	k.Logger(ctx).Info("sending attestation packet via IBC v1",
		"source_port", sourcePort,
		"source_channel", sourceChannel,
		"channel_state", channel.State.String(),
		"attestation_count", len(attestations),
		"timeout", timeoutTimestamp,
		"payload_size", len(dataBytes),
	)

	// Send packet via IBC v1 channel keeper
	// IBC v1 uses SendPacket which takes packet data bytes and returns sequence
	sequence, err := k.channelKeeper.SendPacket(
		sdkCtx,
		sourcePort,
		sourceChannel,
		timeoutHeight,
		timeoutTimestamp,
		dataBytes,
	)
	if err != nil {
		k.Logger(ctx).Error("failed to send attestation packet via IBC v1",
			"error", err,
			"source_port", sourcePort,
			"source_channel", sourceChannel,
		)
		return 0, fmt.Errorf("failed to send IBC v1 packet: %w", err)
	}

	k.Logger(ctx).Info("attestation packet sent successfully via IBC v1",
		"sequence", sequence,
		"source_port", sourcePort,
		"source_channel", sourceChannel,
		"attestation_count", len(attestations),
	)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_v1_sent",
			sdk.NewAttribute("source_port", sourcePort),
			sdk.NewAttribute("source_channel", sourceChannel),
			sdk.NewAttribute("sequence", fmt.Sprintf("%d", sequence)),
			sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", len(attestations))),
		),
	)

	return sequence, nil
}

// GetV1ChannelID returns the configured IBC v1 channel ID for attestation
func (k *Keeper) GetV1ChannelID(ctx context.Context, key string) (string, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(append(types.V1ChannelIDPrefix, []byte(key)...))
	if err != nil {
		return "", err
	}
	if len(bz) == 0 {
		return "", fmt.Errorf("v1 channel ID not configured for key: %s", key)
	}
	return string(bz), nil
}

// SetV1ChannelID stores the IBC v1 channel ID for attestation
func (k *Keeper) SetV1ChannelID(ctx context.Context, key string, channelID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(append(types.V1ChannelIDPrefix, []byte(key)...), []byte(channelID))
}

// GetV1PortID returns the configured IBC v1 port ID for attestation
func (k *Keeper) GetV1PortID(ctx context.Context, key string) (string, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(append(types.V1PortIDPrefix, []byte(key)...))
	if err != nil {
		return "", err
	}
	if len(bz) == 0 {
		// Return default port ID if not configured
		return types.PortID, nil
	}
	return string(bz), nil
}

// SetV1PortID stores the IBC v1 port ID for attestation
func (k *Keeper) SetV1PortID(ctx context.Context, key string, portID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(append(types.V1PortIDPrefix, []byte(key)...), []byte(portID))
}
