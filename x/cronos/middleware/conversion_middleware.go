package middleware

import (
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	transferTypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	cronoskeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"

	"cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ porttypes.UpgradableModule = (*IBCConversionModule)(nil)

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
) (string, error) {
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
			return channeltypes.NewErrorAcknowledgement(errors.Wrap(sdkerrors.ErrUnknownRequest,
				"cannot unmarshal ICS-20 transfer packet data in middleware"))
		}
		denom := im.getIbcDenomFromPacketAndData(packet, data)
		// Check if it can be converted
		if im.canBeConverted(ctx, denom) {
			err = im.convertVouchers(ctx, data, denom, false)
			if err != nil {
				return channeltypes.NewErrorAcknowledgement(err)
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
	if err == nil {
		// Call the middle ware only at the "refund" case
		var ack channeltypes.Acknowledgement
		if err := transferTypes.ModuleCdc.UnmarshalJSON(acknowledgement, &ack); err != nil {
			return errors.Wrapf(sdkerrors.ErrUnknownRequest,
				"cannot unmarshal ICS-20 transfer packet acknowledgement in middleware: %v", err)
		}
		if _, ok := ack.Response.(*channeltypes.Acknowledgement_Error); ok {
			data, err := im.getFungibleTokenPacketData(packet)
			if err != nil {
				return err
			}
			denom := im.getIbcDenomFromDataForRefund(data)
			if im.canBeConverted(ctx, denom) {
				return im.convertVouchers(ctx, data, denom, true)
			}
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
		denom := im.getIbcDenomFromDataForRefund(data)
		if im.canBeConverted(ctx, denom) {
			return im.convertVouchers(ctx, data, denom, true)
		}
	}
	return err
}

func (im IBCConversionModule) getFungibleTokenPacketData(packet channeltypes.Packet) (transferTypes.FungibleTokenPacketData, error) {
	var data transferTypes.FungibleTokenPacketData
	if err := transferTypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return data, errors.Wrapf(sdkerrors.ErrUnknownRequest,
			"cannot unmarshal ICS-20 transfer packet data in middleware: %s", err.Error())
	}
	return data, nil
}

func (im IBCConversionModule) convertVouchers(ctx sdk.Context, data transferTypes.FungibleTokenPacketData, denom string, isSender bool) error {
	// parse the transfer amount
	transferAmount, ok := sdkmath.NewIntFromString(data.Amount)
	if !ok {
		return errors.Wrapf(transferTypes.ErrInvalidAmount,
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

func (im IBCConversionModule) canBeConverted(ctx sdk.Context, denom string) bool {
	params := im.cronoskeeper.GetParams(ctx)
	if denom == params.IbcCroDenom {
		return true
	}
	_, found := im.cronoskeeper.GetContractByDenom(ctx, denom)
	return found
}

func (im IBCConversionModule) getIbcDenomFromDataForRefund(data transferTypes.FungibleTokenPacketData) string {
	return transferTypes.ParseDenomTrace(data.Denom).IBCDenom()
}

func (im IBCConversionModule) getIbcDenomFromPacketAndData(
	packet channeltypes.Packet, data transferTypes.FungibleTokenPacketData,
) string {
	if transferTypes.ReceiverChainIsSource(packet.GetSourcePort(), packet.GetSourceChannel(), data.Denom) {
		voucherPrefix := transferTypes.GetDenomPrefix(packet.GetSourcePort(), packet.GetSourceChannel())
		unprefixedDenom := data.Denom[len(voucherPrefix):]
		denom := unprefixedDenom
		denomTrace := transferTypes.ParseDenomTrace(unprefixedDenom)
		if denomTrace.Path != "" {
			denom = denomTrace.IBCDenom()
		}
		return denom
	}

	// since SendPacket did not prefix the denomination, we must prefix denomination here
	sourcePrefix := transferTypes.GetDenomPrefix(packet.GetDestPort(), packet.GetDestChannel())
	prefixedDenom := sourcePrefix + data.Denom
	denomTrace := transferTypes.ParseDenomTrace(prefixedDenom)
	return denomTrace.IBCDenom()
}

// OnChanUpgradeInit implements the IBCModule interface
func (im IBCConversionModule) OnChanUpgradeInit(
	ctx sdk.Context,
	portID string,
	channelID string,
	proposedOrder channeltypes.Order,
	proposedConnectionHops []string,
	proposedVersion string,
) (string, error) {
	cbs, ok := im.app.(porttypes.UpgradableModule)
	if !ok {
		return "", errors.Wrap(porttypes.ErrInvalidRoute, "upgrade route not found to module in application callstack")
	}
	return cbs.OnChanUpgradeInit(ctx, portID, channelID, proposedOrder, proposedConnectionHops, proposedVersion)
}

// OnChanUpgradeAck implements the IBCModule interface
func (im IBCConversionModule) OnChanUpgradeAck(ctx sdk.Context, portID, channelID, counterpartyVersion string) error {
	cbs, ok := im.app.(porttypes.UpgradableModule)
	if !ok {
		return errors.Wrap(porttypes.ErrInvalidRoute, "upgrade route not found to module in application callstack")
	}
	return cbs.OnChanUpgradeAck(ctx, portID, channelID, counterpartyVersion)
}

// OnChanUpgradeOpen implements the IBCModule interface
func (im IBCConversionModule) OnChanUpgradeOpen(ctx sdk.Context, portID, channelID string, proposedOrder channeltypes.Order, proposedConnectionHops []string, proposedVersion string) {
	cbs, ok := im.app.(porttypes.UpgradableModule)
	if !ok {
		panic(errors.Wrap(porttypes.ErrInvalidRoute, "upgrade route not found to module in application callstack"))
	}
	cbs.OnChanUpgradeOpen(ctx, portID, channelID, proposedOrder, proposedConnectionHops, proposedVersion)
}

// OnChanUpgradeTry implement s the IBCModule interface
func (im IBCConversionModule) OnChanUpgradeTry(ctx sdk.Context, portID, channelID string, proposedOrder channeltypes.Order, proposedConnectionHops []string, counterpartyVersion string) (string, error) {
	cbs, ok := im.app.(porttypes.UpgradableModule)
	if !ok {
		return "", errors.Wrap(porttypes.ErrInvalidRoute, "upgrade route not found to module in application callstack")
	}
	return cbs.OnChanUpgradeTry(ctx, portID, channelID, proposedOrder, proposedConnectionHops, counterpartyVersion)
}
