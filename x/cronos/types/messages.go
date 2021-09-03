package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	TypeMsgConvertVouchers = "ConvertVouchers"
	TypeMsgTransferTokens  = "TransferTokens"
)

var _ sdk.Msg = &MsgConvertVouchers{}

func NewMsgConvertVouchers(address string, coins sdk.Coins) *MsgConvertVouchers {
	return &MsgConvertVouchers{
		Address: address,
		Coins:   coins,
	}
}

// Route ...
func (msg MsgConvertVouchers) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgConvertVouchers) Type() string {
	return TypeMsgConvertVouchers
}

// GetSigners ...
func (msg *MsgConvertVouchers) GetSigners() []sdk.AccAddress {
	address, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{address}
}

// GetSignBytes ...
func (msg *MsgConvertVouchers) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// ValidateBasic ...
func (msg *MsgConvertVouchers) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address (%s)", err)
	}
	if !msg.Coins.IsValid() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}

	if !msg.Coins.IsAllPositive() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}
	return nil
}

var _ sdk.Msg = &MsgTransferTokens{}

func NewMsgTransferTokens(from string, to string, coins sdk.Coins) *MsgTransferTokens {
	return &MsgTransferTokens{
		From:  from,
		To:    to,
		Coins: coins,
	}
}

// Route ...
func (msg MsgTransferTokens) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgTransferTokens) Type() string {
	return TypeMsgTransferTokens
}

// GetSigners ...
func (msg *MsgTransferTokens) GetSigners() []sdk.AccAddress {
	from, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{from}
}

// GetSignBytes ...
func (msg *MsgTransferTokens) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// ValidateBasic ...
func (msg *MsgTransferTokens) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address address (%s)", err)
	}

	// TODO, validate TO address format

	if !msg.Coins.IsValid() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}

	if !msg.Coins.IsAllPositive() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}
	return nil
}
