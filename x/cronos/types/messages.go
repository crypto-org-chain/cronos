package types

import (
	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
)

const (
	TypeMsgConvertVouchers    = "ConvertVouchers"
	TypeMsgTransferTokens     = "TransferTokens"
	TypeMsgUpdateTokenMapping = "UpdateTokenMapping"
	TypeMsgUpdateParams       = "UpdateParams"
	TypeMsgTurnBridge         = "TurnBridge"
	TypeMsgUpdatePermissions  = "UpdatePermissions"
)

var (
	_ sdk.Msg = &MsgConvertVouchers{}
	_ sdk.Msg = &MsgTransferTokens{}
	_ sdk.Msg = &MsgUpdateTokenMapping{}
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgTurnBridge{}
	_ sdk.Msg = &MsgUpdatePermissions{}
)

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
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address (%s)", err)
	}
	if !msg.Coins.IsValid() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}

	if !msg.Coins.IsAllPositive() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
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
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid address address (%s)", err)
	}

	// TODO, validate TO address format

	if !msg.Coins.IsValid() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}

	if !msg.Coins.IsAllPositive() {
		return errors.Wrap(sdkerrors.ErrInvalidCoins, msg.Coins.String())
	}
	return nil
}

var _ sdk.Msg = &MsgUpdateTokenMapping{}

// NewMsgUpdateTokenMapping ...
func NewMsgUpdateTokenMapping(admin string, denom string, contract string, symbol string, decimal uint32) *MsgUpdateTokenMapping {
	return &MsgUpdateTokenMapping{
		Sender:   admin,
		Denom:    denom,
		Contract: contract,
		Symbol:   symbol,
		Decimal:  decimal,
	}
}

// GetSigners ...
func (msg *MsgUpdateTokenMapping) GetSigners() []sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{sender}
}

// ValidateBasic ...
func (msg *MsgUpdateTokenMapping) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}

	if !IsValidCoinDenom(msg.Denom) {
		return errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid denom format (%s)", msg.Denom)
	}

	if !common.IsHexAddress(msg.Contract) {
		return errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid contract address (%s)", msg.Contract)
	}

	return nil
}

// Route ...
func (msg MsgUpdateTokenMapping) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgUpdateTokenMapping) Type() string {
	return TypeMsgUpdateTokenMapping
}

// GetSignBytes ...
func (msg *MsgUpdateTokenMapping) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// NewMsgTurnBridge ...
func NewMsgTurnBridge(admin string, enable bool) *MsgTurnBridge {
	return &MsgTurnBridge{
		Sender: admin,
		Enable: enable,
	}
}

// GetSigners ...
func (msg *MsgTurnBridge) GetSigners() []sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{sender}
}

// ValidateBasic ...
func (msg *MsgTurnBridge) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}

	return nil
}

// Route ...
func (msg MsgTurnBridge) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgTurnBridge) Type() string {
	return TypeMsgTurnBridge
}

// GetSignBytes ...
func (msg *MsgTurnBridge) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func NewMsgUpdateParams(authority string, params Params) *MsgUpdateParams {
	return &MsgUpdateParams{
		Authority: authority,
		Params:    params,
	}
}

// GetSigners returns the expected signers for a MsgUpdateParams message.
func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}

// ValidateBasic does a sanity check on the provided data.
func (msg *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return errors.Wrap(err, "invalid authority address")
	}

	if err := msg.Params.Validate(); err != nil {
		return err
	}

	return nil
}

// Route ...
func (msg MsgUpdateParams) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgUpdateParams) Type() string {
	return TypeMsgUpdateParams
}

// GetSignBytes ...
func (msg *MsgUpdateParams) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// NewMsgUpdatePermissions ...
func NewMsgUpdatePermissions(from string, address string, permissions uint64) *MsgUpdatePermissions {
	return &MsgUpdatePermissions{
		From:        from,
		Address:     address,
		Permissions: permissions,
	}
}

// GetSigners ...
func (msg *MsgUpdatePermissions) GetSigners() []sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{sender}
}

// ValidateBasic ...
func (msg *MsgUpdatePermissions) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.From)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
	}
	_, err = sdk.AccAddressFromBech32(msg.Address)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid target address (%s)", err)
	}

	return nil
}

// Route ...
func (msg MsgUpdatePermissions) Route() string {
	return RouterKey
}

// Type ...
func (msg MsgUpdatePermissions) Type() string {
	return TypeMsgUpdatePermissions
}

// GetSignBytes ...
func (msg *MsgUpdatePermissions) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}
