package keeper

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcerrors "github.com/cosmos/ibc-go/v10/modules/core/errors"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"

	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

var _ porttypes.IBCModule = (*IBCModuleV1)(nil)

// IBCModuleV1 implements the ICS26 callbacks for the attestation module using IBC v1
type IBCModuleV1 struct {
	keeper *Keeper
}

// NewIBCModuleV1 creates a new IBCModuleV1 given the attestation keeper
func NewIBCModuleV1(k *Keeper) IBCModuleV1 {
	return IBCModuleV1{
		keeper: k,
	}
}

// OnChanOpenInit implements the IBCModule interface
func (im IBCModuleV1) OnChanOpenInit(
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

	// Validate port ID
	v1PortID, _ := im.keeper.GetV1PortID(ctx, "attestation-layer")
	if v1PortID == "" {
		v1PortID = "attestation" // Default port
	}
	if portID != v1PortID {
		return "", errorsmod.Wrapf(porttypes.ErrInvalidPort, "invalid port: %s, expected %s", portID, v1PortID)
	}

	// Validate version
	if version != types.Version {
		return "", errorsmod.Wrapf(ibcerrors.ErrInvalidRequest, "got %s, expected %s", version, types.Version)
	}

	return version, nil
}

// OnChanOpenTry implements the IBCModule interface
func (im IBCModuleV1) OnChanOpenTry(
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

	// Validate port ID
	v1PortID, _ := im.keeper.GetV1PortID(ctx, "attestation-layer")
	if v1PortID == "" {
		v1PortID = "attestation" // Default port
	}
	if portID != v1PortID {
		return "", errorsmod.Wrapf(porttypes.ErrInvalidPort, "invalid port: %s, expected %s", portID, v1PortID)
	}

	// Validate version
	if counterpartyVersion != types.Version {
		return "", errorsmod.Wrapf(ibcerrors.ErrInvalidRequest, "invalid counterparty version: got: %s, expected %s", counterpartyVersion, types.Version)
	}

	return types.Version, nil
}

// OnChanOpenAck implements the IBCModule interface
func (im IBCModuleV1) OnChanOpenAck(
	ctx sdk.Context,
	portID,
	channelID string,
	counterpartyChannelID string,
	counterpartyVersion string,
) error {
	// Validate counterparty version
	if counterpartyVersion != types.Version {
		return errorsmod.Wrapf(ibcerrors.ErrInvalidRequest, "invalid counterparty version: %s, expected %s", counterpartyVersion, types.Version)
	}

	// Store the port ID and channel ID for future use
	if err := im.keeper.SetV1PortID(ctx, "attestation-layer", portID); err != nil {
		return errorsmod.Wrapf(err, "failed to set v1 port ID")
	}
	if err := im.keeper.SetV1ChannelID(ctx, "attestation-layer", channelID); err != nil {
		return errorsmod.Wrapf(err, "failed to set v1 channel ID")
	}

	// Verify it was stored correctly
	storedChannelID, err := im.keeper.GetV1ChannelID(ctx, "attestation-layer")
	if err != nil {
		im.keeper.Logger(ctx).Error("Failed to verify stored channel ID", "error", err)
	} else {
		im.keeper.Logger(ctx).Info("Verified stored channel ID", "stored_channel_id", storedChannelID)
	}

	// Log for debugging
	im.keeper.Logger(ctx).Info("IBC v1 channel opened (OnChanOpenAck)",
		"port_id", portID,
		"channel_id", channelID,
		"counterparty_channel_id", counterpartyChannelID,
	)

	return nil
}

// OnChanOpenConfirm implements the IBCModule interface
func (im IBCModuleV1) OnChanOpenConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Store the port ID and channel ID for future use
	if err := im.keeper.SetV1PortID(ctx, "attestation-layer", portID); err != nil {
		return errorsmod.Wrapf(err, "failed to set v1 port ID")
	}
	if err := im.keeper.SetV1ChannelID(ctx, "attestation-layer", channelID); err != nil {
		return errorsmod.Wrapf(err, "failed to set v1 channel ID")
	}

	// Log for debugging
	im.keeper.Logger(ctx).Info("IBC v1 channel opened (OnChanOpenConfirm)",
		"port_id", portID,
		"channel_id", channelID,
	)

	return nil
}

// OnChanCloseInit implements the IBCModule interface
func (im IBCModuleV1) OnChanCloseInit(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Disallow user-initiated channel closing for attestation channels
	return errorsmod.Wrap(ibcerrors.ErrInvalidRequest, "user cannot close channel")
}

// OnChanCloseConfirm implements the IBCModule interface
func (im IBCModuleV1) OnChanCloseConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// Store the channel ID for future use
	im.keeper.SetV1ChannelID(ctx, "attestation-layer", channelID)

	return nil
}

// OnRecvPacket implements the IBCModule interface
// Attestation module only sends packets, it doesn't receive them
func (im IBCModuleV1) OnRecvPacket(
	ctx sdk.Context,
	channelID string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) exported.Acknowledgement {
	// Return error acknowledgement - attestation module doesn't handle incoming packets
	return channeltypes.NewErrorAcknowledgement(errorsmod.Wrapf(ibcerrors.ErrInvalidRequest, "attestation module cannot receive packets"))
}

// OnAcknowledgementPacket implements the IBCModule interface
func (im IBCModuleV1) OnAcknowledgementPacket(
	ctx sdk.Context,
	channelID string,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	// Log acknowledgement
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_v1_ack",
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute("ack", string(acknowledgement)),
		),
	)

	return nil
}

// OnTimeoutPacket implements the IBCModule interface
func (im IBCModuleV1) OnTimeoutPacket(
	ctx sdk.Context,
	channelID string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	// Log timeout
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_v1_timeout",
			sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
			sdk.NewAttribute("sequence", fmt.Sprintf("%d", packet.Sequence)),
		),
	)

	return nil
}
