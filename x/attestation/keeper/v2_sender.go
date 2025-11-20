package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

// SendAttestationPacketV2 sends block attestations via IBC v2 to the attestation chain
// This uses the simplified client-to-client communication without port/channel
func (k Keeper) SendAttestationPacketV2(
	ctx context.Context,
	sourceClient string,
	destinationClient string,
	attestations []*types.BlockAttestationData,
	relayer string,
	signature []byte,
	nonce uint64,
) (uint64, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 0, err
	}

	if !params.AttestationEnabled {
		return 0, types.ErrAttestationDisabled
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

	// Marshal packet data to JSON for v2 payload
	dataBytes, err := json.Marshal(packetData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal packet data: %w", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Calculate timeout
	timeoutTimestamp := uint64(sdkCtx.BlockTime().UnixNano()) + params.PacketTimeoutTimestamp

	k.Logger(ctx).Info("sending attestation packet via IBC v2",
		"source_client", sourceClient,
		"dest_client", destinationClient,
		"attestation_count", len(attestations),
		"timeout", timeoutTimestamp,
		"payload_size", len(dataBytes),
	)

	// Note: In a full implementation, this would call the v2 channel keeper's SendPacket
	// with a properly constructed Payload struct:
	//
	// payload := channelv2types.Payload{
	//     SourcePort:      "attestation",
	//     DestinationPort: "attestation",
	//     Version:         types.Version,
	//     Encoding:        "json",
	//     Value:           dataBytes,
	// }
	//
	// Then call: sequence := channelKeeperV2.SendPacket(ctx, sourceClient, destinationClient, timeoutTimestamp, []Payload{payload})
	//
	// For now, we return a placeholder sequence
	// TODO: Integrate with actual v2 channel keeper when packet sending is fully implemented
	sequence := uint64(sdkCtx.BlockHeight())

	// Emit event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_v2_packet_sent",
			sdk.NewAttribute("source_client", sourceClient),
			sdk.NewAttribute("dest_client", destinationClient),
			sdk.NewAttribute("sequence", fmt.Sprintf("%d", sequence)),
			sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", len(attestations))),
		),
	)

	return sequence, nil
}

// GetV2ClientID returns the configured IBC v2 client ID for attestation
func (k Keeper) GetV2ClientID(ctx context.Context, key string) (string, error) {
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
func (k Keeper) SetV2ClientID(ctx context.Context, key string, clientID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(append(types.V2ClientIDPrefix, []byte(key)...), []byte(clientID))
}
