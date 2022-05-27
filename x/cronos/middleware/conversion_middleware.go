package middleware

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	transferTypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v3/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v3/modules/core/exported"
	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
)

// IBCConversionModule implements the ICS26 interface.
type IBCConversionModule struct {
	app          porttypes.IBCModule
	cronoskeeper cronoskeeper.Keeper
}

// NewIBCConversionModule creates a new IBCModule given the keeper and underlying application
func NewIBCConversionModule(app porttypes.IBCModule, ck cronoskeeper.Keeper) IBCConversionModule {
	return IBCConversionModule{
		app:          app,
		cronoskeeper: ck,
	}
}

// OnChanOpenInit implements the IBCModule interface
func (im IBCConversionModule) OnChanOpenInit(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	chanCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty,
	version string,
) error {
	return im.app.OnChanOpenInit(ctx, order, connectionHops, portID, channelID, chanCap, counterparty, version)
}

func (im IBCConversionModule) OnChanOpenTry(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID,
	channelID string,
	chanCap *capabilitytypes.Capability,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (string, error) {
	return im.app.OnChanOpenTry(ctx, order, connectionHops, portID, channelID, chanCap, counterparty, counterpartyVersion)
}

// OnChanOpenAck implements the IBCModule interface
func (im IBCConversionModule) OnChanOpenAck(
	ctx sdk.Context,
	portID,
	channelID string,
	counterpartyChannelID string,
	counterpartyVersion string,
) error {
	return im.app.OnChanOpenAck(ctx, portID, channelID, counterpartyChannelID, counterpartyVersion)
}

// OnChanOpenConfirm implements the IBCModule interface
func (im IBCConversionModule) OnChanOpenConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	// call underlying app's OnChanOpenConfirm callback.
	return im.app.OnChanOpenConfirm(ctx, portID, channelID)
}

// OnChanCloseInit implements the IBCModule interface
func (im IBCConversionModule) OnChanCloseInit(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	return im.app.OnChanCloseInit(ctx, portID, channelID)
}

// OnChanCloseConfirm implements the IBCModule interface
func (im IBCConversionModule) OnChanCloseConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	return im.app.OnChanCloseConfirm(ctx, portID, channelID)
}

// OnRecvPacket implements the IBCModule interface.
func (im IBCConversionModule) OnRecvPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) exported.Acknowledgement {
	ack := im.app.OnRecvPacket(ctx, packet, relayer)
	if ack.Success() {
		data, err := im.getFungibleTokenPacketData(packet)
		if err != nil {
			return channeltypes.NewErrorAcknowledgement(
				"cannot unmarshal ICS-20 transfer packet data in middleware")
		}
		// We need to convert the voucher only in case the receiver is "not" the source chain
		if !transferTypes.ReceiverChainIsSource(packet.GetSourcePort(), packet.GetSourceChannel(), data.Denom) {
			sourcePrefix := transferTypes.GetDenomPrefix(packet.GetDestPort(), packet.GetDestChannel())
			// NOTE: sourcePrefix contains the trailing "/"
			prefixedDenom := sourcePrefix + data.Denom
			// construct the denomination trace from the full raw denomination
			denomTrace := transferTypes.ParseDenomTrace(prefixedDenom)
			err = im.convertVouchers(ctx, data, denomTrace.IBCDenom(), false)
			if err != nil {
				return transferTypes.NewErrorAcknowledgement(err)
			}
		}
	}
	return ack
}

// OnAcknowledgementPacket implements the IBCModule interface
func (im IBCConversionModule) OnAcknowledgementPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	err := im.app.OnAcknowledgementPacket(ctx, packet, acknowledgement, relayer)

	if err != nil {
		// Call the middle ware only at the "refund" case
		var ack channeltypes.Acknowledgement
		if err := transferTypes.ModuleCdc.UnmarshalJSON(acknowledgement, &ack); err != nil {
			return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest,
				"cannot unmarshal ICS-20 transfer packet acknowledgement in middleware: %v", err)
		}
		switch ack.Response.(type) {
		case *channeltypes.Acknowledgement_Error:
			data, err := im.getFungibleTokenPacketData(packet)
			if err != nil {
				return err
			}
			// Only in case the token is originated from the receiver chain
			if transferTypes.ReceiverChainIsSource(packet.GetSourcePort(), packet.GetSourceChannel(), data.Denom) {
				// parse the denomination from the full denom path
				trace := transferTypes.ParseDenomTrace(data.Denom)
				err = im.convertVouchers(ctx, data, trace.BaseDenom, true)
				if err != nil {
					return err
				}
			}
		default:
		}
	}

	return err
}

// OnTimeoutPacket implements the IBCModule interface
func (im IBCConversionModule) OnTimeoutPacket(
	ctx sdk.Context,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	err := im.app.OnTimeoutPacket(ctx, packet, relayer)
	// If no error on the refund
	if err == nil {
		data, err := im.getFungibleTokenPacketData(packet)
		if err != nil {
			return err
		}
		// Only in case the token is originated from the receiver chain
		if transferTypes.ReceiverChainIsSource(packet.GetSourcePort(), packet.GetSourceChannel(), data.Denom) {
			// parse the denomination from the full denom path
			trace := transferTypes.ParseDenomTrace(data.Denom)
			err = im.convertVouchers(ctx, data, trace.IBCDenom(), true)
			if err != nil {
				return err
			}
		}
	}

	return err
}

func (im IBCConversionModule) getFungibleTokenPacketData(packet channeltypes.Packet) (transferTypes.FungibleTokenPacketData, error) {
	var data transferTypes.FungibleTokenPacketData
	if err := transferTypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return data, sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest,
			"cannot unmarshal ICS-20 transfer packet data in middleware: %s", err.Error())
	}
	return data, nil
}

func (im IBCConversionModule) convertVouchers(ctx sdk.Context, data transferTypes.FungibleTokenPacketData, denom string, isSender bool) error {

	// parse the transfer amount
	transferAmount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		return sdkerrors.Wrapf(transferTypes.ErrInvalidAmount,
			"unable to parse transfer amount (%s) into sdk.Int in middleware", data.Amount)
	}
	token := sdk.NewCoin(denom, transferAmount)
	if isSender {
		im.cronoskeeper.OnRecvVouchers(ctx, sdk.NewCoins(token), data.Sender)
	} else {
		im.cronoskeeper.OnRecvVouchers(ctx, sdk.NewCoins(token), data.Receiver)
	}
	return nil
}
