package keeper

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	channelv2types "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	"github.com/cosmos/ibc-go/v10/modules/core/api"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

var _ api.IBCModule = (*IBCModuleV2)(nil)

// IBCModuleV2 implements the IBC v2 Module interface for attestation
type IBCModuleV2 struct {
	keeper Keeper
}

// NewIBCModuleV2 creates a new IBCModuleV2 given a keeper
func NewIBCModuleV2(k Keeper) IBCModuleV2 {
	return IBCModuleV2{
		keeper: k,
	}
}

// OnSendPacket implements the IBCModule interface for v2
// Called when sending attestation data from Cronos to attestation layer
func (im IBCModuleV2) OnSendPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	payload channelv2types.Payload,
	signer sdk.AccAddress,
) error {
	// Sender chain does not need to handle this callback function
	return nil
}

// OnRecvPacket implements the IBCModule interface for v2
// Called on attestation layer when receiving attestation data from Cronos
func (im IBCModuleV2) OnRecvPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	payload channelv2types.Payload,
	relayer sdk.AccAddress,
) channelv2types.RecvPacketResult {
	ctx.Logger().Info("IBC v2 Attestation: RecvPacket",
		"source_client", sourceClient,
		"dest_client", destinationClient,
		"sequence", sequence,
		"relayer", relayer.String(),
	)

	// Decode attestation packet data
	var packetData types.AttestationPacketData
	if err := json.Unmarshal(payload.Value, &packetData); err != nil {
		ctx.Logger().Error("failed to unmarshal attestation packet", "error", err)
		return channelv2types.RecvPacketResult{
			Status:          channelv2types.PacketStatus_Failure,
			Acknowledgement: []byte(fmt.Sprintf(`{"success":false,"error":"%s"}`, err.Error())),
		}
	}

	// Process attestations
	var attestationIds []uint64
	var attestBlockHeights []uint64

	for _, attestation := range packetData.Attestations {
		// Store attestation as pending (consensus storage)
		if err := im.keeper.AddPendingAttestation(ctx, attestation.BlockHeight, &attestation); err != nil {
			ctx.Logger().Error("failed to store attestation",
				"height", attestation.BlockHeight,
				"error", err,
			)
			continue
		}

		// Store finality in LOCAL database (no consensus)
		// This provides fast queries without consensus overhead
		if err := im.keeper.MarkBlockFinalizedLocal(
			ctx,
			attestation.BlockHeight,
			ctx.BlockTime().Unix(),
			attestation.BlockHash,
		); err != nil {
			ctx.Logger().Error("failed to store finality locally",
				"height", attestation.BlockHeight,
				"error", err,
			)
			// Don't fail the packet - local storage is optional
		}

		// Track attestation IDs (using sequence as ID for now)
		attestationIds = append(attestationIds, sequence)
		attestBlockHeights = append(attestBlockHeights, attestation.BlockHeight)

		// Emit event
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"attestation_v2_received",
				sdk.NewAttribute("source_client", sourceClient),
				sdk.NewAttribute("dest_client", destinationClient),
				sdk.NewAttribute("sequence", fmt.Sprintf("%d", sequence)),
				sdk.NewAttribute("block_height", fmt.Sprintf("%d", attestation.BlockHeight)),
				sdk.NewAttribute("chain_id", packetData.SourceChainId),
			),
		)
	}

	// Create acknowledgement with finality feedback
	ack := types.AttestationPacketAcknowledgement{
		Success:            true,
		AttestationIds:     attestationIds,
		AttestBlockHeights: attestBlockHeights,
		FinalizedAt:        ctx.BlockTime().Unix(),
		FinalityProof:      nil, // TODO: Add proof if needed
	}

	ackBytes, err := json.Marshal(ack)
	if err != nil {
		ctx.Logger().Error("failed to marshal acknowledgement", "error", err)
		return channelv2types.RecvPacketResult{
			Status:          channelv2types.PacketStatus_Failure,
			Acknowledgement: []byte(fmt.Sprintf(`{"success":false,"error":"failed to marshal ack"}`)),
		}
	}

	return channelv2types.RecvPacketResult{
		Status:          channelv2types.PacketStatus_Success,
		Acknowledgement: ackBytes,
	}
}

// OnAcknowledgementPacket implements the IBCModule interface for v2
// Called on Cronos when receiving acknowledgement from attestation layer
func (im IBCModuleV2) OnAcknowledgementPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	acknowledgement []byte,
	payload channelv2types.Payload,
	relayer sdk.AccAddress,
) error {
	ctx.Logger().Info("IBC v2 Attestation: AcknowledgementPacket",
		"source_client", sourceClient,
		"dest_client", destinationClient,
		"sequence", sequence,
		"relayer", relayer.String(),
	)

	// Decode acknowledgement
	var ack types.AttestationPacketAcknowledgement
	if err := json.Unmarshal(acknowledgement, &ack); err != nil {
		return fmt.Errorf("failed to unmarshal acknowledgement: %w", err)
	}

	if !ack.Success {
		ctx.Logger().Error("attestation packet failed on counterparty",
			"error", ack.Error,
		)
		return nil
	}

	// Process finality feedback for each attested block height
	for _, height := range ack.AttestBlockHeights {
		// Store finality in LOCAL database only (no consensus storage)
		if err := im.keeper.MarkBlockFinalizedLocal(ctx, height, ack.FinalizedAt, ack.FinalityProof); err != nil {
			ctx.Logger().Error("failed to store finality locally",
				"height", height,
				"error", err,
			)
			// Continue - local storage failure shouldn't block processing
		}

		// Emit finality event
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"block_finalized_v2",
				sdk.NewAttribute("block_height", fmt.Sprintf("%d", height)),
				sdk.NewAttribute("finalized_at", fmt.Sprintf("%d", ack.FinalizedAt)),
				sdk.NewAttribute("sequence", fmt.Sprintf("%d", sequence)),
				sdk.NewAttribute("source_client", sourceClient),
				sdk.NewAttribute("dest_client", destinationClient),
			),
		)

		// Remove from pending queue
		if err := im.keeper.RemovePendingAttestation(ctx, height); err != nil {
			ctx.Logger().Error("failed to remove pending attestation",
				"height", height,
				"error", err,
			)
		}
	}

	ctx.Logger().Info("processed finality feedback",
		"finalized_count", len(ack.AttestBlockHeights),
		"attestation_ids", len(ack.AttestationIds),
	)

	return nil
}

// OnTimeoutPacket implements the IBCModule interface for v2
// Called on Cronos when attestation packet times out
func (im IBCModuleV2) OnTimeoutPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	payload channelv2types.Payload,
	relayer sdk.AccAddress,
) error {
	ctx.Logger().Info("IBC v2 Attestation: TimeoutPacket",
		"source_client", sourceClient,
		"dest_client", destinationClient,
		"sequence", sequence,
		"relayer", relayer.String(),
	)

	// Decode attestation packet data
	var packetData types.AttestationPacketData
	if err := json.Unmarshal(payload.Value, &packetData); err != nil {
		return fmt.Errorf("failed to unmarshal packet data: %w", err)
	}

	// Log timeout for monitoring
	ctx.Logger().Error("attestation packet timed out",
		"source_client", sourceClient,
		"dest_client", destinationClient,
		"sequence", sequence,
		"attestation_count", len(packetData.Attestations),
		"chain_id", packetData.SourceChainId,
	)

	// Emit timeout event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_v2_timeout",
			sdk.NewAttribute("source_client", sourceClient),
			sdk.NewAttribute("dest_client", destinationClient),
			sdk.NewAttribute("sequence", fmt.Sprintf("%d", sequence)),
			sdk.NewAttribute("attestation_count", fmt.Sprintf("%d", len(packetData.Attestations))),
		),
	)

	// TODO: Implement retry logic or mark attestations as failed
	// For now, keep them in pending state for manual intervention

	return nil
}
