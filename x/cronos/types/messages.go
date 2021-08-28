package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	TypeMsgConvertTokens   = "ConvertTokens"
	TypeMsgSendToCryptoOrg = "SendToCryptoOrg"
)

var _ sdk.Msg = &MsgConvertTokens{}

func NewMsgConvertTokens(address string, amount sdk.Coins) *MsgConvertTokens {
	return &MsgConvertTokens{
		Address: address,
		Amount:  amount,
	}
}

// Route ...
func (msg MsgConvertTokens) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgConvertTokens) Type() string {
	return TypeMsgConvertTokens
}

// GetSigners ...
func (msg *MsgConvertTokens) GetSigners() []sdk.AccAddress {
	address, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{address}
}

// GetSignBytes ...
func (msg *MsgConvertTokens) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// ValidateBasic ...
func (msg *MsgConvertTokens) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address (%s)", err)
	}
	if !msg.Amount.IsValid() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Amount.String())
	}

	if !msg.Amount.IsAllPositive() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Amount.String())
	}
	return nil
}

var _ sdk.Msg = &MsgSendToCryptoOrg{}

func NewMsgSendToCryptoOrg(from string, to string, amount sdk.Coins) *MsgSendToCryptoOrg {
	return &MsgSendToCryptoOrg{
		From:   from,
		To:     to,
		Amount: amount,
	}
}

// Route ...
func (msg MsgSendToCryptoOrg) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgSendToCryptoOrg) Type() string {
	return TypeMsgSendToCryptoOrg
}

// GetSigners ...
func (msg *MsgSendToCryptoOrg) GetSigners() []sdk.AccAddress {
	from, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{from}
}

// GetSignBytes ...
func (msg *MsgSendToCryptoOrg) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// ValidateBasic ...
func (msg *MsgSendToCryptoOrg) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address address (%s)", err)
	}

	// TODO, validate TO address format

	if !msg.Amount.IsValid() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Amount.String())
	}

	if !msg.Amount.IsAllPositive() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, msg.Amount.String())
	}
	return nil
}
