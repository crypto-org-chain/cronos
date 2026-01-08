package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	channelv2types "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

// SendAttestationPacketV2 sends block attestations via IBC v2 to the attestation chain
// This uses the simplified client-to-client communication without port/channel
func (k *Keeper) SendAttestationPacketV2(
	ctx context.Context,
	sourceClient string,
	destinationClient string,
	attestations []*types.BlockAttestationData,
) (uint64, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 0, err
	}

	if !params.AttestationEnabled {
		return 0, types.ErrAttestationDisabled
	}

	// Check if channel keeper v2 is set
	if k.channelKeeperV2 == nil {
		return 0, fmt.Errorf("IBC v2 channel keeper not initialized")
	}

	// Convert pointer slice to value slice for proto
	attestationValues := make([]types.BlockAttestationData, len(attestations))
	for i, att := range attestations {
		attestationValues[i] = *att
	}

	// Create packet data
	// Note: IBC v2 handles relayer, signature, and nonce at the transport layer
	packetData := &types.AttestationPacketData{
		SourceChainId: k.chainID,
		Attestations:  attestationValues,
	}

	// Marshal packet data to JSON for v2 payload
	dataBytes, err := json.Marshal(packetData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal packet data: %w", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Calculate timeout (in seconds for IBC v2)
	// PacketTimeoutTimestamp is in nanoseconds, convert to seconds
	timeoutSeconds := params.PacketTimeoutTimestamp / 1_000_000_000
	if timeoutSeconds == 0 {
		timeoutSeconds = 600 // Default 10 minutes if not set
	}
	timeoutTimestamp := uint64(sdkCtx.BlockTime().Unix()) + timeoutSeconds

	k.Logger(ctx).Info("sending attestation packet via IBC v2",
		"source_client", sourceClient,
		"dest_client", destinationClient,
		"attestation_count", len(attestations),
		"timeout", timeoutTimestamp,
		"payload_size", len(dataBytes),
	)

	// Create IBC v2 payload
	payload := channelv2types.NewPayload(
		types.PortID,  // SourcePort
		"da",          // DestinationPort
		types.Version, // Version
		"json",        // Encoding
		dataBytes,     // Value
	)

	// Create MsgSendPacket - use authority (gov module) address as signer
	signer := k.authority

	msg := channelv2types.NewMsgSendPacket(
		sourceClient,
		timeoutTimestamp,
		signer,
		payload,
	)

	// Validate the message
	if err := msg.ValidateBasic(); err != nil {
		return 0, fmt.Errorf("invalid MsgSendPacket: %w", err)
	}

	// Send the packet via IBC v2 channel keeper
	response, err := k.channelKeeperV2.SendPacket(ctx, msg)
	if err != nil {
		k.Logger(ctx).Error("failed to send attestation packet via IBC v2",
			"error", err,
			"source_client", sourceClient,
		)
		return 0, fmt.Errorf("failed to send IBC v2 packet: %w", err)
	}

	sequence := response.Sequence

	k.Logger(ctx).Info("attestation packet sent successfully via IBC v2",
		"sequence", sequence,
		"source_client", sourceClient,
		"dest_client", destinationClient,
		"attestation_count", len(attestations),
	)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_v2_sent",
			sdk.NewAttribute("source_client", sourceClient),
			sdk.NewAttribute("dest_client", destinationClient),
			sdk.NewAttribute("sequence", fmt.Sprintf("%d", sequence)),
			sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", len(attestations))),
		),
	)

	return sequence, nil
}

// GetV2ClientID returns the configured IBC v2 client ID for attestation
func (k *Keeper) GetV2ClientID(ctx context.Context, key string) (string, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(append(types.V2ClientIDPrefix, []byte(key)...))
	if err != nil {
		return "", err
	}
	if len(bz) == 0 {
		return "", fmt.Errorf("v2 client ID not configured for key: %s", key)
	}
	return string(bz), nil
}

// SetV2ClientID stores the IBC v2 client ID for attestation
func (k *Keeper) SetV2ClientID(ctx context.Context, key string, clientID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(append(types.V2ClientIDPrefix, []byte(key)...), []byte(clientID))
}
