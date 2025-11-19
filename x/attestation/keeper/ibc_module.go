package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

var _ porttypes.IBCModule = (*IBCModule)(nil)

// IBCModule implements the ICS26 interface for the attestation module
type IBCModule struct {
	keeper Keeper
}

// NewIBCModule creates a new IBCModule given the keeper
func NewIBCModule(k Keeper) IBCModule {
	return IBCModule{
		keeper: k,
	}
}

// OnChanOpenInit implements the IBCModule interface
func (im IBCModule) OnChanOpenInit(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	counterparty channeltypes.Counterparty,
	version string,
) (string, error) {
	// Validate parameters
	if order != channeltypes.UNORDERED {
		return "", errorsmod.Wrapf(channeltypes.ErrInvalidChannelOrdering, "expected %s channel, got %s", channeltypes.UNORDERED, order)
	}

	// Validate version
	if version != types.Version {
		return "", errorsmod.Wrapf(types.ErrInvalidChannel, "expected version %s, got %s", types.Version, version)
	}

	return version, nil
}

// OnChanOpenTry implements the IBCModule interface
func (im IBCModule) OnChanOpenTry(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID,
	channelID string,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (string, error) {
	// Validate parameters
	if order != channeltypes.UNORDERED {
		return "", errorsmod.Wrapf(channeltypes.ErrInvalidChannelOrdering, "expected %s channel, got %s", channeltypes.UNORDERED, order)
	}

	// Validate version
	if counterpartyVersion != types.Version {
		return "", errorsmod.Wrapf(types.ErrInvalidChannel, "expected counterparty version %s, got %s", types.Version, counterpartyVersion)
	}

	return types.Version, nil
}

// OnChanOpenAck implements the IBCModule interface
func (im IBCModule) OnChanOpenAck(
	ctx sdk.Context,
	portID,
	channelID string,
	counterpartyChannelID string,
	counterpartyVersion string,
) error {
	if counterpartyVersion != types.Version {
		return errorsmod.Wrapf(types.ErrInvalidChannel, "expected counterparty version %s, got %s", types.Version, counterpartyVersion)
	}

	// Store the channel ID for later use
	if err := im.keeper.SetChannelID(ctx, channelID); err != nil {
		return err
	}

	im.keeper.Logger(ctx).Info("attestation channel opened",
		"port", portID,
		"channel", channelID,
		"counterparty_channel", counterpartyChannelID,
	)

	return nil
}

// OnChanOpenConfirm implements the IBCModule interface
func (im IBCModule) OnChanOpenConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Store the channel ID for later use
	if err := im.keeper.SetChannelID(ctx, channelID); err != nil {
		return err
	}

	im.keeper.Logger(ctx).Info("attestation channel confirmed",
		"port", portID,
		"channel", channelID,
	)

	return nil
}

// OnChanCloseInit implements the IBCModule interface
func (im IBCModule) OnChanCloseInit(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Disallow user-initiated channel closing
	return errorsmod.Wrap(types.ErrInvalidChannel, "user cannot close channel")
}

// OnChanCloseConfirm implements the IBCModule interface
func (im IBCModule) OnChanCloseConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	im.keeper.Logger(ctx).Info("attestation channel closed",
		"port", portID,
		"channel", channelID,
	)
	return nil
}

// OnRecvPacket implements the IBCModule interface
// This is called on the attestation chain when it receives a packet from Cronos
func (im IBCModule) OnRecvPacket(
	ctx sdk.Context,
	channelID string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) ibcexported.Acknowledgement {
	var packetData types.AttestationPacketData
	if err := im.keeper.cdc.Unmarshal(packet.GetData(), &packetData); err != nil {
		return channeltypes.NewErrorAcknowledgement(fmt.Errorf("cannot unmarshal packet data: %w", err))
	}

	// Process the attestation packet
	// This would be implemented on the attestation chain side
	ack := types.AttestationPacketAcknowledgement{
		Success: true,
		Error:   "",
	}

	ackBytes, err := im.keeper.cdc.Marshal(&ack)
	if err != nil {
		return channeltypes.NewErrorAcknowledgement(fmt.Errorf("cannot marshal acknowledgement: %w", err))
	}

	return channeltypes.NewResultAcknowledgement(ackBytes)
}

// OnAcknowledgementPacket implements the IBCModule interface
// This is called on Cronos chain when it receives an ack from the attestation chain
func (im IBCModule) OnAcknowledgementPacket(
	ctx sdk.Context,
	channelID string,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	var ack channeltypes.Acknowledgement
	if err := im.keeper.cdc.Unmarshal(acknowledgement, &ack); err != nil {
		return errorsmod.Wrapf(types.ErrInvalidAck, "cannot unmarshal packet acknowledgement: %v", err)
	}

	switch resp := ack.Response.(type) {
	case *channeltypes.Acknowledgement_Result:
		return im.onAcknowledgementSuccess(ctx, packet, resp.Result)
	case *channeltypes.Acknowledgement_Error:
		return im.onAcknowledgementError(ctx, packet, resp.Error)
	default:
		return errorsmod.Wrapf(types.ErrInvalidAck, "unknown acknowledgement response type: %T", resp)
	}
}

// onAcknowledgementSuccess processes a successful acknowledgement
func (im IBCModule) onAcknowledgementSuccess(ctx sdk.Context, packet channeltypes.Packet, data []byte) error {
	var packetData types.AttestationPacketData
	if err := im.keeper.cdc.Unmarshal(packet.GetData(), &packetData); err != nil {
		return err
	}

	var ack types.AttestationPacketAcknowledgement
	if err := im.keeper.cdc.Unmarshal(data, &ack); err != nil {
		return err
	}

	im.keeper.Logger(ctx).Info("received attestation acknowledgement",
		"success", ack.Success,
		"finalized_count", ack.FinalizedCount,
		"blocks", len(packetData.Attestations),
	)

	// Process finality status for each block
	for height, status := range ack.FinalityStatuses {
		if status.Finalized {
			if err := im.keeper.MarkBlockFinalized(ctx, height, status.FinalizedAt, status.FinalityProof); err != nil {
				im.keeper.Logger(ctx).Error("failed to mark block as finalized",
					"height", height,
					"error", err,
				)
				continue
			}

			// Emit finality event
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"block_finalized",
					sdk.NewAttribute("chain_id", packetData.SourceChainId),
					sdk.NewAttribute("block_height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("finalized_at", fmt.Sprintf("%d", status.FinalizedAt)),
					sdk.NewAttribute("attestation_id", fmt.Sprintf("%d", status.AttestationId)),
				),
			)

			// Remove from pending queue
			if err := im.keeper.RemovePendingAttestation(ctx, height); err != nil {
				im.keeper.Logger(ctx).Error("failed to remove pending attestation",
					"height", height,
					"error", err,
				)
			}
		}
	}

	return nil
}

// onAcknowledgementError processes an error acknowledgement
func (im IBCModule) onAcknowledgementError(ctx sdk.Context, packet channeltypes.Packet, errorMsg string) error {
	var packetData types.AttestationPacketData
	if err := im.keeper.cdc.Unmarshal(packet.GetData(), &packetData); err != nil {
		return err
	}

	im.keeper.Logger(ctx).Error("attestation packet failed",
		"error", errorMsg,
		"blocks", len(packetData.Attestations),
	)

	// TODO: Implement retry logic or alerting for failed attestations

	return nil
}

// OnTimeoutPacket implements the IBCModule interface
func (im IBCModule) OnTimeoutPacket(
	ctx sdk.Context,
	channelID string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	var packetData types.AttestationPacketData
	if err := im.keeper.cdc.Unmarshal(packet.GetData(), &packetData); err != nil {
		return err
	}

	im.keeper.Logger(ctx).Error("attestation packet timed out",
		"blocks", len(packetData.Attestations),
		"timeout_timestamp", packet.TimeoutTimestamp,
	)

	// TODO: Implement retry logic for timed out packets

	return nil
}
